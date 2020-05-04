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
)

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
		return 0, errors.Wrapf(err, "get service property", service, name)
	}

	value := prop.Value.Value()
	ival, ok := value.(int)
	if !ok {
		return 0, errors.Errorf("is %T (%v)", service, name, value, value)
	}

	return ival, nil
}

func (door *systemdDoor) Enter(ct *container.Container, spec *containerEntrySpec) (*processExitStatus, error) {
	leader, err := getLeader(ct.Name)
	if err != nil {
		return nil, errors.Wrap(err, "get container leader")
	}

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

	serviceName := fmt.Sprintf("nsbox-entry-%s.service", uuid.New().String())

	systemdRun := []string{"systemd-run", "--quiet", "--pipe", fmt.Sprintf("--machine=%s", ct.Name), fmt.Sprintf("--unit=%s", serviceName)}
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
	// cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			exitCode, err := getServicePropertyInt(systemd, serviceName, "ExecMainCode")
			if err != nil {
				return nil, errors.Wrapf(err, "get ExecMainCode of %s", serviceName)
			}

			exitStatus, err := getServicePropertyInt(systemd, serviceName, "ExecMainStatus")
			if err != nil {
				return nil, errors.Wrapf(err, "get ExecMainStatus of %s", serviceName)
			}

			if exitCode == cldExited {
				return &processExitStatus{exitType: processExitNormal, result: exitStatus}, nil
			} else {
				return &processExitStatus{exitType: processExitSignaled, result: exitStatus}, nil
			}
		} else {
			return nil, errors.Wrap(err, "starting systemd-run")
		}
	}

	return &processExitStatus{exitType: processExitNormal, result: 0}, nil
}
