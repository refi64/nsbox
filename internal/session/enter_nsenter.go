/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package session

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/selinux"
	"golang.org/x/sys/unix"
)

// A door that enters the container environment via nsenter.
type nsenterDoor struct{}

func convertStateToProcessExit(state *os.ProcessState) (*processExitStatus, error) {
	// XXX: syscall is deprecated, but this cast will fail if it directly jumps to
	// unix.WaitStatus.
	waitStatus := unix.WaitStatus(state.Sys().(syscall.WaitStatus))

	if waitStatus.Signaled() {
		return &processExitStatus{exitType: processExitSignaled, result: int(waitStatus.Signal())}, nil
	} else if waitStatus.Exited() {
		return &processExitStatus{exitType: processExitNormal, result: waitStatus.ExitStatus()}, nil
	} else {
		return nil, errors.Errorf("Unexpected wait status %d", int(waitStatus))
	}
}

func (door *nsenterDoor) Enter(ct *container.Container, spec *containerEntrySpec) (*processExitStatus, error) {
	leader, err := getLeader(ct.Name)
	if err != nil {
		return nil, errors.Wrap(err, "getting leader process")
	}

	nsenterCmd := []string{"nsenter", "-at", strconv.Itoa(int(leader))}
	nsenterCmd = append(nsenterCmd, spec.buildNsboxHostCommand()...)

	log.Debug("running:", nsenterCmd)

	if err := selinux.SetExecProcessContextContainer(); err != nil {
		log.Alert("failed to set exec context to enter container:", err)
	}

	// If there's no pty, we can exec the command directly.
	if spec.ptyPath == "" {
		nsenter, err := exec.LookPath("nsenter")
		if err != nil {
			return nil, errors.Wrap(err, "failed to find nsenter")
		}

		if err := unix.Exec(nsenter, nsenterCmd, os.Environ()); err != nil {
			return nil, errors.Wrap(err, "failed to exec into namespace")
		}

		panic("should not reach here")
	}

	cmd := exec.Command(nsenterCmd[0], nsenterCmd[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		// Handle ExitError later on.
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, errors.Wrap(err, "failed to enter into namespace")
		}
	}

	return convertStateToProcessExit(cmd.ProcessState)
}
