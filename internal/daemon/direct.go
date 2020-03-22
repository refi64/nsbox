/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Called by nsboxd to directly start an nspawn container, while also starting a varlink service
// responsible for letting the container talk to the host for various reasons.
package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdutil "github.com/coreos/go-systemd/util"
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

func bindLoopDevices(builder *nspawn.Builder) {
	cmd := exec.Command("losetup", "--find")
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		log.Debug("losetup failed: ", err)
		return
	}

	loopDevices, err := filepath.Glob("/dev/loop*")
	if err != nil {
		panic(fmt.Sprintf("failed to find loop devices: %v", loopDevices))
	}

	for _, device := range loopDevices {
		builder.AddBind(device)
	}
}

func bindHome(builder *nspawn.Builder, usrdata *userdata.Userdata) error {
	homeParent := filepath.Dir(usrdata.User.HomeDir)

	info, err := os.Lstat(homeParent)
	if err != nil {
		return errors.Wrap(err, "failed to stat home parent")
	}

	// Need to handle Silverblue-esque /home symlinks specially.
	if info.Mode()&os.ModeSymlink != 0 {
		resolvedHomeParent, err := filepath.EvalSymlinks(homeParent)
		if err != nil {
			return errors.Wrap(err, "failed to resolve home parent")
		}

		builder.AddRecursiveBind(resolvedHomeParent)

		relResolvedHomeParent, err := filepath.Rel(filepath.Dir(homeParent), resolvedHomeParent)
		if err != nil {
			return errors.Wrapf(err, "failed to make home parent %s relative", resolvedHomeParent)
		}

		usrdata.Environ["NSBOX_HOME_LINK_NAME"] = homeParent
		usrdata.Environ["NSBOX_HOME_LINK_TARGET"] = resolvedHomeParent
		usrdata.Environ["NSBOX_HOME_LINK_TARGET_REL"] = relResolvedHomeParent
	} else {
		builder.AddRecursiveBind(usrdata.User.HomeDir)
	}

	return nil
}

func bindGraphics(builder *nspawn.Builder) error {
	tmpdir := os.TempDir()
	x11 := filepath.Join(tmpdir, ".X11-unix")
	if _, err := os.Stat(x11); err == nil {
		builder.AddBind(x11)
	}

	builder.AddBind("/dev/dri")
	return nil
}

func stripLeadingSlash(path string) string {
	result, err := filepath.Rel("/", path)
	if err != nil {
		panic(errors.Wrapf(err, "unexpected error stripping leading slash from %s", path))
	}

	return result
}

func setUserEnv(hostMachineId string, img *image.Image, ct *container.Container, usrdata *userdata.Userdata) {
	usrdata.Environ["NSBOX_USER"] = usrdata.User.Username
	usrdata.Environ["NSBOX_UID"] = usrdata.User.Uid

	if usrdata.HasSudoAccess() {
		if img.SudoGroup != "" {
			usrdata.Environ["NSBOX_SUDO_GROUP"] = img.SudoGroup
		} else {
			usrdata.Environ["NSBOX_SUDO_GROUP"] = "wheel"
		}
	} else {
		usrdata.Environ["NSBOX_SUDO_GROUP"] = ""
	}

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
	if err := service.Bind(serviceUri); err != nil {
		return nil, errors.Wrap(err, "failed to bind to varlink service")
	}

	listener, err := service.GetListener()
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare to listen on varlink service")
	}

	go func() {
		if err := service.DoListen(0); err != nil {
			log.Alert("failed to listen on host service socket:", err)
		}
	}()

	return listener, err
}

func RunContainerDirectNspawn(ct *container.Container, usrdata *userdata.Userdata) error {
	var firewall network.Firewall

	if err := ct.LockUntilProcessDeath(container.NoWaitForLock); err != nil {
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
	builder.MachineName = ct.Name
	builder.Hostname = ct.Name
	builder.Capabilities = ct.Config.ExtraCapabilities
	builder.SystemCallFilter = strings.Join(ct.Config.SyscallFilters, " ")

	if ct.Config.Boot {
		builder.Boot = true
	} else {
		builder.AsPid2 = true
	}

	usrdata.Environ["HOSTNAME"] = ct.Name

	hostPrivPath := ct.StorageChild(stripLeadingSlash(paths.InContainerPrivPath))
	if err := os.MkdirAll(hostPrivPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create private directory")
	}

	builder.AddBindTo(hostPrivPath, "/run/host/nsbox")

	scripts, err := paths.GetPathRelativeToInstallRoot(paths.Share, paths.ProductName, "data", "scripts")
	if err != nil {
		return errors.Wrap(err, "failed to locate scripts")
	}

	builder.AddBindTo(scripts, filepath.Join(paths.InContainerPrivPath, "scripts"))

	nsboxHost, err := paths.GetPathRelativeToInstallRoot(paths.Libexec, paths.ProductName, "nsbox-host")
	if err != nil {
		return errors.Wrap(err, "failed to locate nsbox-host")
	}

	builder.AddBindTo(nsboxHost, filepath.Join(paths.InContainerPrivPath, "nsbox-host"))

	mainImage, err := image.Open(ct.Config.Image)
	if err != nil {
		return errors.Wrap(err, "failed to get container base image path")
	}

	imgChain, err := mainImage.ResolveChain()
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

	builder.AddBind(filepath.Join("/var/log/journal", machineId))

	releaseDir, err := paths.GetPathRelativeToInstallRoot(paths.Share, paths.ProductName, "release")
	if err != nil {
		return errors.Wrap(err, "failed to locate release files")
	}

	builder.AddBindTo(releaseDir, filepath.Join(paths.InContainerPrivPath, "release"))

	if ct.Config.Boot {
		// Bind the entire xdg runtime directory, then nsbox-init.sh will manually symlink
		// stuff into the in-container runtime directory as needed.
		builder.AddRecursiveBindTo(xdgRuntimeDir, filepath.Join(paths.InContainerPrivPath, "usr-run"))

		dataDir, err := paths.GetPathRelativeToInstallRoot(paths.Share, paths.ProductName, "data")
		if err != nil {
			return errors.Wrap(err, "failed to locate nsbox-init.service")
		}

		nsboxInit := filepath.Join(dataDir, "nsbox-init.service")
		builder.AddBindTo(nsboxInit, "/etc/systemd/system/nsbox-init.service")

		nsboxTarget := filepath.Join(dataDir, "nsbox-container.target")
		builder.AddBindTo(nsboxTarget, "/etc/systemd/system/nsbox-container.target")

		gettyOverride := filepath.Join(dataDir, "getty-override.conf")
		builder.AddBindTo(gettyOverride, "/etc/systemd/system/console-getty.service.d/00-nsbox.conf")

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

	if _, err := os.Stat("/run/media"); err == nil {
		builder.AddBind("/run/media")
	}

	builder.AddBindTo("/etc", "/run/host/etc")

	maildir := filepath.Join("/var/mail", usrdata.User.Username)
	if _, err := os.Stat(maildir); err == nil {
		builder.AddBindTo(maildir, filepath.Join(paths.InContainerPrivPath, "mail"))
	}

	bindLoopDevices(builder)

	if err := bindHome(builder, usrdata); err != nil {
		return errors.Wrap(err, "failed to bind home")
	}

	if err := bindGraphics(builder); err != nil {
		return errors.Wrap(err, "failed to bind graphics")
	}

	if ct.Config.ShareCgroupfs {
		builder.AddRecursiveBind("/sys/fs/cgroup")
	}

	setUserEnv(machineId, mainImage, ct, usrdata)

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
