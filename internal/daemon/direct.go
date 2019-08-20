/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Called by nsboxd to directly start an nspawn container, while also starting a varlink service
// responsible for letting the container talk to the host for various reasons.
package daemon

import (
	"bufio"
	"fmt"
	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/nspawn"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"github.com/refi64/nsbox/internal/varlinkhost"
	"github.com/varlink/go/varlink"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func readMachineId() (id string, err error) {
	file, err := os.Open("/etc/machine-id")
	if err != nil {
		return
	}

	defer file.Close()

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	id = strings.TrimSpace(line)
	return
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
	if info.Mode() & os.ModeSymlink != 0 {
		resolvedHomeParent, err := filepath.EvalSymlinks(homeParent)
		if err != nil {
			return errors.Wrap(err, "failed to resolve home parent")
		}

		builder.AddRecursiveBind(resolvedHomeParent)

		usrdata.Environ["NSBOX_HOME_LINK_NAME"] = homeParent
		usrdata.Environ["NSBOX_HOME_LINK_TARGET"] = resolvedHomeParent
	} else {
		builder.AddRecursiveBind(usrdata.User.HomeDir)
	}

	return nil
}

func stripLeadingSlash(path string) string {
	result, err := filepath.Rel("/", path)
	if err != nil {
		panic(errors.Wrapf(err, "unexpected error stripping leading slash from %s", path))
	}

	return result
}

func setUserEnv(hostMachineId string, ct *container.Container, usrdata *userdata.Userdata) {
	usrdata.Environ["NSBOX_USER"] = usrdata.User.Username
	usrdata.Environ["NSBOX_UID"] = usrdata.User.Uid

	fullShellPath := ct.StorageChild(stripLeadingSlash(usrdata.Shell))

	if _, err := os.Stat(fullShellPath); err != nil {
		usrdata.Environ["NSBOX_SHELL"] = "/bin/bash"
	} else {
		usrdata.Environ["NSBOX_SHELL"] = usrdata.Shell
	}

	usrdata.Environ["NSBOX_CONTAINER"] = ct.Name
	usrdata.Environ["NSBOX_HOST_MACHINE"] = hostMachineId
}

func writeContainerFiles(hostPrivPath string, usrdata *userdata.Userdata) error {
	supplementaryGroups, err := os.Create(filepath.Join(hostPrivPath, "supplementary-groups"))
	if err != nil {
		return err
	}

	defer supplementaryGroups.Close()

	for _, gid := range usrdata.GroupIds {
		if gid == usrdata.User.Gid {
			continue
		}

		fmt.Fprintf(supplementaryGroups, "::%s\n", gid)
	}

	sharedEnv, err := os.Create(filepath.Join(hostPrivPath, "shared-env"))
	if err != nil {
		return err
	}

	defer sharedEnv.Close()

	for name, value := range usrdata.Environ {
		fmt.Fprintf(sharedEnv, "export %s=%s\n", name, shellquote.Join(value))
	}

	return nil
}

func startVarlinkService(hostPrivPath string) (*net.Listener, error) {
	service, err := varlink.NewService(
		"nsbox",
		"nsbox",
		"1",
		"https://nsbox.dev/",
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create new varlink service")
	}

	host := varlinkhost.New()
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
			log.Fatal(err)
		}
	}()

	return listener, err
}

func RunContainerDirectNspawn(ct *container.Container, usrdata *userdata.Userdata) error {
	builder, err := nspawn.NewBuilder()
	if err != nil {
		return err
	}

	builder.Quiet = true
	builder.AsPid2 = true
	builder.MachineDirectory = ct.Storage()
	builder.LinkJournal = "host"
	builder.MachineName = ct.Name
	builder.Hostname = "toolbox"

	hostPrivPath := ct.StorageChild(stripLeadingSlash(paths.InContainerPrivPath))
	if err := os.MkdirAll(hostPrivPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create private directory")
	}

	builder.AddBindTo(hostPrivPath, "/run/host/nsbox")

	scripts, err := paths.GetPathRelativeToInstallRoot("share", "nsbox", "scripts")
	if err != nil {
		return errors.Wrap(err, "failed to locate scripts")
	}

	builder.AddBindTo(scripts, filepath.Join(paths.InContainerPrivPath, "scripts"))

	nsboxUtil, err := paths.GetPathRelativeToInstallRoot(paths.Libexec, "nsbox", "nsbox-host")
	if err != nil {
		return errors.Wrap(err, "failed to locate nsbox-host")
	}

	builder.AddBindTo(nsboxUtil, "/usr/bin/nsbox-host")

	machineId, err := readMachineId()
	if err != nil {
		return errors.Wrap(err, "failed to read machine id")
	}

	builder.AddBind(filepath.Join("/var/log/journal", machineId))
	builder.AddBind("/var/lib/systemd/coredump")

	if value, ok := usrdata.Environ["XDG_RUNTIME_DIR"]; ok {
		builder.AddBind(value)
	}

	if value, ok := usrdata.Environ["DBUS_SYSTEM_BUS_ADDRESS"]; ok {
		builder.AddBind(value)
	} else {
		builder.AddBind("/run/dbus")
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

	setUserEnv(machineId, ct, usrdata)

	if err := writeContainerFiles(hostPrivPath, usrdata); err != nil {
		return errors.Wrap(err, "failed to write private container files")
	}

	varlinkListener, err := startVarlinkService(hostPrivPath)
	if err != nil {
		return err
	}

	defer (*varlinkListener).Close()

	builder.Command = []string{"/run/host/nsbox/scripts/nsbox-init.sh"}
	nspawnArgs := builder.Build()

	log.Debug("running:", nspawnArgs)

	nspawnCmd := exec.Command(nspawnArgs[0], nspawnArgs[1:]...)
	nspawnCmd.Stdout = os.Stdout
	nspawnCmd.Stderr = os.Stderr

	if err := nspawnCmd.Run(); err != nil {
		return err
	}

	return nil
}
