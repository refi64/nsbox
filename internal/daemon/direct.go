/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Called by nsboxd to directly start an nspawn container, while also starting a varlink service
// responsible for letting the container talk to the host for various reasons.
package daemon

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdutil "github.com/coreos/go-systemd/v22/util"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/image"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/network"
	"github.com/refi64/nsbox/internal/nspawn"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/selinux"
	"github.com/refi64/nsbox/internal/userdata"
	"github.com/refi64/nsbox/internal/varlinkhost"
	"github.com/varlink/go/varlink"
	"golang.org/x/sys/unix"
)

func getXdgRuntimeDir(usrdata *userdata.Userdata) (string, error) {
	if value, ok := usrdata.Environ["XDG_RUNTIME_DIR"]; ok {
		return value, nil
	} else {
		return "", errors.New("XDG_RUNTIME_DIR must be set")
	}
}

func bindHome(builder *nspawn.Builder, usrdata *userdata.Userdata) error {
	usrdata.Environ["NSBOX_HOME"] = usrdata.User.HomeDir

	// There are two cases for /home on Fedora Silverblue:
	// - The user's $HOME is /home/USER, and /home is a symlink to /var/home.
	// - The user's $HOME is /var/home/USER, but /home is still a symlink to /var/home.
	// These both need special handling here.

	info, err := os.Lstat("/home")
	if err != nil {
		return errors.Wrap(err, "failed to stat home parent")
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolvedHomeRoot, err := filepath.EvalSymlinks("/home")
		if err != nil {
			return errors.Wrap(err, "failed to resolve home parent")
		}

		builder.AddRecursiveBind(resolvedHomeRoot)

		relResolvedHomeRoot, err := filepath.Rel("/", resolvedHomeRoot)
		if err != nil {
			return errors.Wrapf(err, "failed to make home parent %s relative", resolvedHomeRoot)
		}

		usrdata.Environ["NSBOX_HOME_LINK_TARGET"] = relResolvedHomeRoot

		// If it's an older SB layout ($HOME=/home/USER where /home is a symlink),
		// then we need special handling later on to ensure the CWD is correct.
		if homeParent := filepath.Dir(usrdata.User.HomeDir); homeParent == "/home" {
			usrdata.Environ["NSBOX_HOME_LINK_TARGET_ADJUST_CWD"] = "1"
		}
	} else {
		builder.AddRecursiveBind(usrdata.User.HomeDir)
	}

	return nil
}

func bindDevices(builder *nspawn.Builder, ct *container.Container) {
	tmpdir := os.TempDir()
	x11 := filepath.Join(tmpdir, ".X11-unix")
	builder.AddBind(x11)

	bindFullDev := false

	for _, dev := range ct.Config.ShareDevices {
		if dev == "*" {
			builder.AddRecursiveBind("/dev")
			bindFullDev = true
		} else {
			builder.AddRecursiveBind(dev)
		}
	}

	if !bindFullDev {
		builder.AddBind("/dev/dri")
		builder.AddBind("/dev/input")
	}
}

func stripLeadingSlash(path string) string {
	result, err := filepath.Rel("/", path)
	if err != nil {
		panic(errors.Wrapf(err, "unexpected error stripping leading slash from %s", path))
	}

	return result
}

func setupSudo(img *image.Image, ct *container.Container, usrdata *userdata.Userdata) error {
	shouldSetNoPasswd := false

	sudoGroup := img.SudoGroup
	if sudoGroup == "" {
		sudoGroup = "wheel"
	}

	usrdata.Environ["NSBOX_SUDO_GROUP"] = sudoGroup

	if sudoAccess := usrdata.GetSudoAccess(); sudoAccess != userdata.NoSudo {
		usrdata.Environ["NSBOX_CAN_SUDO"] = "1"

		if sudoAccess == userdata.CanSudoNoPasswd {
			shouldSetNoPasswd = true
		}
	} else {
		usrdata.Environ["NSBOX_CAN_SUDO"] = ""
	}

	noPasswdFile := ct.StorageChild("etc", "sudoers.d", "10-nsbox-passwd")
	if shouldSetNoPasswd {
		line := fmt.Sprintf("%%%s ALL=(ALL:ALL) NOPASSWD: ALL", sudoGroup)
		if err := ioutil.WriteFile(noPasswdFile, []byte(line), 0640); err != nil {
			return errors.Wrap(err, "write nopasswd file")
		}

		if err := os.Chmod(noPasswdFile, 0440); err != nil {
			return errors.Wrap(err, "chmod nopasswd file")
		}
	} else {
		if err := os.Remove(noPasswdFile); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "remove nopasswd file")
		}
	}

	return nil
}

func setUserEnv(hostMachineId string, img *image.Image, ct *container.Container, usrdata *userdata.Userdata) {
	usrdata.Environ["NSBOX_USER"] = usrdata.User.Username
	usrdata.Environ["NSBOX_UID"] = usrdata.User.Uid

	usrdata.Environ["NSBOX_SHELL"] = ct.Shell(usrdata)

	usrdata.Environ["NSBOX_CONTAINER"] = ct.Name
	usrdata.Environ["NSBOX_HOST_MACHINE"] = hostMachineId

	if ct.Config.Boot {
		usrdata.Environ["NSBOX_BOOTED"] = "1"
	}
}

func writeContainerFiles(ct *container.Container, hostPrivPath string, usrdata *userdata.Userdata) error {
	sharedEnv, err := os.Create(filepath.Join(hostPrivPath, "shared-env"))
	if err != nil {
		return err
	}

	defer sharedEnv.Close()

	for name, value := range usrdata.Environ {
		fmt.Fprintf(sharedEnv, "%s=%s\n", name, value)
	}

	if ct.Config.Auth == container.AuthAuto {
		shadowLine, err := usrdata.ShadowLine()
		if err != nil {
			return errors.Wrap(err, "failed to get shadow line")
		}

		shadowEntryPath := filepath.Join(hostPrivPath, "shadow-entry")
		if err := os.Remove(shadowEntryPath); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to delete old shadow-entry")
		}

		shadowEntry, err := os.Create(filepath.Join(hostPrivPath, "shadow-entry"))
		if err != nil {
			return err
		}

		defer shadowEntry.Close()

		shadowEntry.Chmod(0)
		fmt.Fprintln(shadowEntry, shadowLine)
	}

	return nil
}

func startVarlinkService(ct *container.Container, hostPrivPath string) (*net.Listener, error) {
	service, err := varlink.NewService(
		"nsbox",
		"nsbox",
		"1",
		"https://nsbox.dev/",
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create new varlink service")
	}

	host := varlinkhost.New(ct)
	if err := service.RegisterInterface(host); err != nil {
		return nil, errors.Wrap(err, "failed to register varlink interface")
	}

	serviceUri := "unix://" + filepath.Join(hostPrivPath, paths.HostServiceSocketName)
	if err := service.Bind(context.Background(), serviceUri); err != nil {
		return nil, errors.Wrap(err, "failed to bind to varlink service")
	}

	listener, err := service.GetListener()
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare to listen on varlink service")
	}

	go func() {
		if err := service.DoListen(context.Background(), 0); err != nil {
			log.Alert("failed to listen on host service socket:", err)
		}
	}()

	return &listener, err
}

func MkdirAllOwnedByUser(path string, uid int, gid int, perm os.FileMode) error {
	// NOTE: This will assumes path is *not* absolute
	parts := strings.Split(path, string(os.PathSeparator))
	currentRoot := ""

	if filepath.IsAbs(path) {
		parts = parts[1:]
		currentRoot = "/"
	}

	for _, part := range parts {
		path := filepath.Join(currentRoot, part)
		if err := os.Mkdir(path, perm); err != nil {
			if !os.IsExist(err) {
				return errors.Wrapf(err, "mkdir %s", path)
			}
		} else {
			// Only chown if it's a newly created dir (i.e. no error was thrown on mkdir)
			if err := os.Chown(path, uid, gid); err != nil {
				return errors.Wrapf(err, "chown %s", path)
			}
		}

		currentRoot = path
	}

	return nil
}

func bindPrivate(builder *nspawn.Builder, ct *container.Container, usrdata *userdata.Userdata) error {
	for _, private := range ct.Config.PrivateDirs {
		// Sanity check.
		if filepath.IsAbs(private) || strings.Contains(private, "..") {
			panic("unexpected absolute private path " + private)
		}

		hostPath := ct.PrivateHomeStorageChild(usrdata, "home", private)
		uid, gid := usrdata.NumericIds()
		if err := MkdirAllOwnedByUser(hostPath, uid, gid, 0700); err != nil {
			return errors.Wrap(err, "create private storage directory")
		}

		builder.AddBindTo(hostPath, filepath.Join(usrdata.User.HomeDir, private))
	}

	return nil
}

func RunContainerDirectNspawn(ct *container.Container, usrdata *userdata.Userdata) error {
	var firewall network.Firewall

	if err := ct.LockUntilProcessDeath(container.RunLock, container.NoWaitForLock); err != nil {
		return err
	}

	xdgRuntimeDir, err := getXdgRuntimeDir(usrdata)
	if err != nil {
		return err
	}

	builder, err := nspawn.NewBuilder()
	if err != nil {
		return err
	}

	builder.Quiet = true
	builder.KeepUnit = true
	builder.NetworkVeth = ct.Config.VirtualNetwork
	if ct.Config.VirtualNetwork {
		builder.NetworkZone, err = network.GenerateUniqueLinkName(ct.Name, nspawn.NetworkZonePrefix)
		if err != nil {
			return errors.Wrapf(err, "generating zone name for %s", ct.Name)
		}

		firewall = network.GetFirewall()
		if firewall != nil {
			defer func() {
				if err := firewall.Close(); err != nil {
					log.Alert("Failed to close firewall:", err)
				}
			}()

			if err := firewall.TrustInterface(nspawn.NetworkZonePrefix + builder.NetworkZone); err != nil {
				log.Alertf("Failed to trust zone %s: %v", builder.NetworkZone, err)
			} else {
				defer func() {
					if err := firewall.UntrustInterface(nspawn.NetworkZonePrefix + builder.NetworkZone); err != nil {
						log.Alertf("Failed to untrust zone %s: %v", builder.NetworkZone, err)
					}
				}()
			}
		}
	}
	builder.MachineDirectory = ct.Storage()
	builder.LinkJournal = "host"
	builder.MachineName = ct.MachineName(usrdata)
	builder.Hostname = ct.Name
	builder.Capabilities = ct.Config.ExtraCapabilities
	builder.SystemCallFilter = strings.Join(ct.Config.SyscallFilters, " ")

	if ct.Config.Boot {
		builder.Boot = true
	} else {
		builder.AsPid2 = true
		builder.PipeConsole = true
	}

	usrdata.Environ["NSBOX_INTERNAL"] = "1"
	usrdata.Environ["HOSTNAME"] = ct.Name

	hostPrivPath := ct.StorageChild(stripLeadingSlash(paths.InContainerPrivPath))
	if err := os.MkdirAll(hostPrivPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create private directory")
	}

	builder.AddBindTo(hostPrivPath, "/run/host/nsbox")

	dataDir, err := paths.GetDataDir()
	if err != nil {
		return errors.Wrap(err, "failed to locate nsbox data directory")
	}

	builder.AddBindTo(filepath.Join(dataDir, "scripts"), filepath.Join(paths.InContainerPrivPath, "scripts"))

	nsboxHost, err := paths.GetPrivateExecutable("nsbox-host")
	if err != nil {
		return errors.Wrap(err, "failed to locate nsbox-host")
	}

	builder.AddBindTo(nsboxHost, filepath.Join(paths.InContainerPrivPath, "bin/nsbox-host"))

	mainImage, err := image.Open(ct.Config.Image, false)
	if err != nil {
		return errors.Wrap(err, "failed to get container base image path")
	}

	imgChain, err := mainImage.ResolveChain(false)
	if err != nil {
		return errors.Wrap(err, "failed to resolve image chain")
	}

	for i, img := range imgChain {
		builder.AddBindTo(img.RootPath, filepath.Join(paths.InContainerPrivPath, "images", img.Name()))
		if i == 0 {
			usrdata.Environ["NSBOX_IMAGE_CHAIN"] = img.Name()
		} else {
			usrdata.Environ["NSBOX_IMAGE_CHAIN"] += " " + img.Name()
		}
	}

	machineId, err := sdutil.GetMachineID()
	if err != nil {
		return errors.Wrap(err, "failed to read machine id")
	}

	hostJournal := filepath.Join("/var/log/journal", machineId)
	builder.AddBind(hostJournal)
	builder.AddBindTo(hostJournal, "/run/host/journal")

	releaseDir, err := paths.GetReleaseDataDir()
	if err != nil {
		return errors.Wrap(err, "failed to locate release files")
	}

	builder.AddBindTo(releaseDir, filepath.Join(paths.InContainerPrivPath, "release"))

	if ct.Config.Boot {
		// Bind the entire xdg runtime directory, then nsbox-init.sh will manually symlink
		// stuff into the in-container runtime directory as needed.
		builder.AddRecursiveBindTo(xdgRuntimeDir, filepath.Join(paths.InContainerPrivPath, "usr-run"))

		nsboxInit := filepath.Join(dataDir, "nsbox-init.service")
		builder.AddBindTo(nsboxInit, "/etc/systemd/system/nsbox-init.service")

		nsboxTarget := filepath.Join(dataDir, "nsbox-container.target")
		builder.AddBindTo(nsboxTarget, "/etc/systemd/system/nsbox-container.target")

		gettyOverride := filepath.Join(dataDir, "getty-override.conf")
		builder.AddBindTo(gettyOverride, "/etc/systemd/system/console-getty.service.d/00-nsbox.conf")

		/*
			Don't clobber the host's exposed Xorg sockets.
			XXX: This really would fit better in the individual images, but that would mean that
			it would be possible for an image to forget, which would result in nasty behavior. */
		tmpfilesX11 := ct.StorageChild("etc/tmpfiles.d/x11.conf")
		if err := os.Remove(tmpfilesX11); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "delete tmpfiles.d/x11.conf")
		}
		if err := os.Symlink("/dev/null", tmpfilesX11); err != nil {
			return errors.Wrap(err, "mask tmpfiles.d/x11.conf")
		}

		if ct.Config.VirtualNetwork {
			wantsNetworkd := filepath.Join(dataDir, "wants-networkd.conf")
			builder.AddBindTo(wantsNetworkd, "/etc/systemd/system/nsbox-container.target.d/00-nsbox-networkd.conf")
		}
	} else {
		// Binding coredumps for a booted container really doesn't make that much sense...
		builder.AddBind("/var/lib/systemd/coredump")
		builder.AddRecursiveBind(xdgRuntimeDir)

		if value, ok := usrdata.Environ["DBUS_SYSTEM_BUS_ADDRESS"]; ok {
			builder.AddBind(value)
		} else {
			builder.AddBind("/run/dbus")
		}
	}

	builder.AddBind("/run/media")

	builder.AddBindTo("/etc", "/run/host/etc")

	maildir := filepath.Join("/var/mail", usrdata.User.Username)
	builder.AddBindTo(maildir, filepath.Join(paths.InContainerPrivPath, "mail"))

	// Only bind home if not private.
	privateHome := false
	for _, private := range ct.Config.PrivateDirs {
		if private == "." {
			privateHome = true
		}
	}

	if privateHome {
		// XXX: What should we do if homedir still would've been a symlink?
		usrdata.Environ["NSBOX_HOME"] = usrdata.User.HomeDir
	} else {
		if err := bindHome(builder, usrdata); err != nil {
			return errors.Wrap(err, "failed to bind home")
		}
	}

	bindDevices(builder, ct)

	if ct.Config.ShareCgroupfs {
		builder.AddRecursiveBind("/sys/fs/cgroup")
	}

	for _, bind := range ct.Config.ExtraBindMounts {
		parts := strings.SplitN(bind, ":", 2)
		if len(parts) == 1 {
			builder.AddBind(parts[0])
		} else {
			builder.AddBindTo(parts[0], parts[1])
		}
	}

	if err := bindPrivate(builder, ct, usrdata); err != nil {
		return errors.Wrap(err, "bind private storage")
	}

	setUserEnv(machineId, mainImage, ct, usrdata)

	if err := setupSudo(mainImage, ct, usrdata); err != nil {
		return errors.Wrap(err, "setup sudo")
	}

	if err := writeContainerFiles(ct, hostPrivPath, usrdata); err != nil {
		return errors.Wrap(err, "failed to write private container files")
	}

	varlinkListener, err := startVarlinkService(ct, hostPrivPath)
	if err != nil {
		return err
	}

	defer (*varlinkListener).Close()

	if ct.Config.Boot {
		builder.Command = []string{"--", "--unit=nsbox-container.target"}
	} else {
		builder.Command = []string{"/run/host/nsbox/scripts/nsbox-init.sh"}
	}

	nspawnArgs := builder.Build()

	log.Debug("running:", nspawnArgs)

	if err := selinux.SetExecProcessContextContainer(); err != nil {
		log.Alert("failed to set exec context:", err)
	}

	nspawnCmd := exec.Command(nspawnArgs[0], nspawnArgs[1:]...)
	nspawnCmd.Stdout = os.Stdout
	nspawnCmd.Stderr = os.Stderr

	// We don't want nspawn notifying of start, since nsbox-init is responsible for that.
	nspawnCmd.Env = os.Environ()
	nspawnCmd.Env = append(nspawnCmd.Env, "NOTIFY_SOCKET=")

	// Shared IPC namespace is required for XShm to work.
	nspawnCmd.Env = append(nspawnCmd.Env, "SYSTEMD_NSPAWN_SHARE_NS_IPC=1")

	if ct.Config.ShareCgroupfs {
		nspawnCmd.Env = append(nspawnCmd.Env, "SYSTEMD_NSPAWN_USE_CGNS=0")
	}

	// Make sure nspawn dies if we do.
	nspawnCmd.SysProcAttr = &unix.SysProcAttr{
		Pdeathsig: unix.SIGTERM,
	}

	if err := nspawnCmd.Run(); err != nil {
		return err
	}

	return nil
}
