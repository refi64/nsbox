/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package session

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	systemd1 "github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/nsbus"
	"github.com/refi64/nsbox/internal/selinux"
	"github.com/refi64/nsbox/internal/userdata"
)

type systemdSessionHandle struct {
	process     *os.Process
	systemd     *systemd1.Conn
	machineName string
	serviceName string
}

// A door that enters the container environment via a transient unit.
// This is used on booted containers, that way the executed process
// exits within the proper cgroups.
type systemdDoor struct{}

const cldExited = 1

type property struct {
	name, value string
}

func getServicePropertyInt(systemd *systemd1.Conn, service, name string) (int, error) {
	prop, err := systemd.GetServiceProperty(service, name)
	if err != nil {
		return 0, errors.Wrapf(err, "get %s property %s", service, name)
	}

	value := prop.Value.Value()
	ival, ok := value.(int32)
	if !ok {
		return 0, errors.Errorf("%s:%s is %T (%v)", service, name, value, value)
	}

	return int(ival), nil
}

func (handle *systemdSessionHandle) Signal(signal os.Signal) error {
	systemdKill := []string{
		"systemctl", "--machine=" + handle.machineName, "kill", handle.serviceName,
	}

	cmd := exec.Command(systemdKill[0], systemdKill[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "killing %s:%s", handle.machineName, handle.serviceName)
	}

	return nil
}

func (handle *systemdSessionHandle) Wait() (*processExitStatus, error) {
	_, err := handle.process.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "waiting for systemd-run")
	}

	exitCode, err := getServicePropertyInt(handle.systemd, handle.serviceName, "ExecMainCode")
	if err != nil {
		return nil, errors.Wrapf(err, "get ExecMainCode of %s", handle.serviceName)
	}

	exitStatus, err := getServicePropertyInt(handle.systemd, handle.serviceName, "ExecMainStatus")
	if err != nil {
		return nil, errors.Wrapf(err, "get ExecMainStatus of %s", handle.serviceName)
	}

	if exitCode == cldExited {
		return &processExitStatus{exitType: processExitNormal, result: exitStatus}, nil
	} else {
		return &processExitStatus{exitType: processExitSignaled, result: exitStatus}, nil
	}
}

func (handle *systemdSessionHandle) Destroy() {
	handle.systemd.Close()
}

func (door *systemdDoor) Enter(ct *container.Container, spec *containerEntrySpec,
	usrdata *userdata.Userdata) (sessionHandle, error) {
	leader, err := getLeader(ct, usrdata)
	if err != nil {
		return nil, errors.Wrap(err, "get container leader")
	}

	var handle *systemdSessionHandle

	// XXX: We need to use systemd-run *and* go-systemd:
	// - go-systemd is needed to gather the command results...
	// - ...but godbus's fd passing is broken, so we need to use systemd-run to actually
	//   start the process.

	systemd, err := systemd1.NewConnection(func() (*godbus.Conn, error) {
		conn, err := nsbus.DialBusInsideNamespace(int(leader))
		if err != nil {
			return nil, errors.Wrap(err, "dialing bus in ns")
		}

		methods := []godbus.Auth{godbus.AuthExternal(strconv.Itoa(os.Getuid()))}
		if err := conn.Auth(methods); err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "auth bus")
		}

		if err := conn.Hello(); err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "hello bus")
		}

		conn.EnableUnixFDs()

		return conn, nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "connect to container bus")
	}

	defer func() {
		if handle == nil {
			systemd.Close()
		}
	}()

	serviceName := fmt.Sprintf("nsbox-entry-%s.service", uuid.New().String())

	systemdRun := []string{"systemd-run", "--quiet", "--pipe",
		fmt.Sprintf("--machine=%s", ct.MachineName(usrdata)),
		fmt.Sprintf("--unit=%s", serviceName)}
	properties := []struct{ name, value string }{}

	if selinux.Enabled() {
		currentLabel, err := selinux.GetCurrentLabel()
		if err != nil {
			return nil, errors.Wrap(err, "find current selinux label")
		}

		newLabel, err := selinux.GetExecLabel(currentLabel)
		if err != nil {
			return nil, errors.Wrap(err, "find new selinux label")
		}

		properties = append(properties, property{
			name:  "SELinuxContext",
			value: newLabel,
		})
	}

	for _, prop := range properties {
		systemdRun = append(systemdRun, fmt.Sprintf("--property=%s=%s", prop.name, prop.value))
	}

	systemdRun = append(systemdRun, "--setenv=NSBOX_INTERNAL=1", "--")
	systemdRun = append(systemdRun, spec.buildNsboxHostCommand()...)

	log.Debug("Starting transient unit in container:", systemdRun)

	cmd := exec.Command(systemdRun[0], systemdRun[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting systemd-run")
	}

	handle = &systemdSessionHandle{
		process:     cmd.Process,
		systemd:     systemd,
		machineName: ct.MachineName(usrdata),
		serviceName: serviceName}
	return handle, nil
}
