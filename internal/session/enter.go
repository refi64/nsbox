/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Enter a running container session.
package session

import (
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/coreos/go-systemd/v22/machine1"
	krpty "github.com/creack/pty"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/ptyservice"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

type ptyIoFlags int

const (
	stdinPtyFlag ptyIoFlags = 1 << iota
	stdoutPtyFlag
	stderrPtyFlag
)

type containerEntrySpec struct {
	ptyPath string
	ptyIo   ptyIoFlags
	env     map[string]string

	verbose bool
	uid     int
	cwd     string

	command []string
}

type processExitType int

const (
	processExitNormal = iota
	processExitSignaled
)

type processExitStatus struct {
	exitType processExitType
	// Either the exit code or the signal.
	result int
}

// A "door" is responsible for entry into a container's environment.
type containerDoor interface {
	Enter(ct *container.Container, spec *containerEntrySpec) (*processExitStatus, error)
}

func getLeader(name string) (uint32, error) {
	machined, err := machine1.New()
	if err != nil {
		return 0, err
	}

	props, err := machined.DescribeMachine(name)
	if err != nil {
		return 0, err
	}

	return props["Leader"].(uint32), nil
}

func safeCopy(dest *os.File, source *os.File) {
	hadSuccessYet := false

	for {
		if _, err := io.Copy(dest, source); err != nil {
			if pathErr, ok := err.(*os.PathError); ok && !hadSuccessYet {
				if unixErr, ok := pathErr.Err.(unix.Errno); ok && unixErr == unix.EIO {
					// XXX: the process probably isn't alive yet. Just try again.
					continue
				}
			}

			log.Fatalf("safeCopy %d -> %d failed: %v", source.Fd(), dest.Fd(), err)
		} else {
			hadSuccessYet = true
		}
	}
}

func (spec containerEntrySpec) buildNsboxHostCommand() []string {
	cmd := []string{"/run/host/nsbox/nsbox-host", "enter"}

	if spec.verbose {
		cmd = append(cmd, "-v")
	}

	cmd = append(cmd, fmt.Sprintf("-uid=%d", spec.uid), fmt.Sprintf("-cwd=%s", spec.cwd))

	// Add the -stdin,-stdout,-stderr=ptypath args if they are PTYs.
	stdioFlags := map[string]ptyIoFlags{"stdin": stdinPtyFlag, "stdout": stdoutPtyFlag, "stderr": stderrPtyFlag}
	for arg, ptyFlag := range stdioFlags {
		if spec.ptyIo&ptyFlag != 0 {
			cmd = append(cmd, fmt.Sprintf("-%s=%s", arg, spec.ptyPath))
		}
	}

	// Add the environment variables.
	cmd = append(cmd, "env")
	for name, value := range spec.env {
		cmd = append(cmd, fmt.Sprintf("%s=%s", name, value))
	}

	return append(cmd, spec.command...)
}

func EnterContainer(ct *container.Container, command []string, usrdata *userdata.Userdata, workdir string) (int, error) {
	if len(command) == 0 {
		command = []string{ct.Shell(usrdata), "-l"}
	}

	// XXX: Okay so PTY handling is a royal mess. nsenter doesn't give us a pty *at all*.
	// Therefore, the plan of action is to ask the pty service for a pty inside the container, and let
	// nsbox-enter know where to redirect what.

	// However, if this is a booted container, the PTY will instead be set up using a systemd
	// transient unit, which is necessary otherwise having an external process inside will
	// mess up cgroups: https://ora.pm/project/211667/kanban/task/3069813

	// Do not touch this code. If you do, and it breaks, you will be haunted by the ghost of
	// several thousand headless terminals that were spawned and killed during the course of
	// this code's development.

	stdio := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	// Indexed using the corresponding FD.
	stdioPtyFlags := []ptyIoFlags{stdinPtyFlag, stdoutPtyFlag, stderrPtyFlag}
	var pty *os.File
	var spec containerEntrySpec

	var forwardStdinToPty = false
	var forwardPtyToWriter *os.File

	var door containerDoor
	if ct.Config.Boot {
		door = &systemdDoor{}
	} else {
		door = &nsenterDoor{}
	}

	for _, file := range stdio {
		fd := int(file.Fd())
		if terminal.IsTerminal(fd) {
			spec.ptyIo |= stdioPtyFlags[fd]
		}
	}

	if spec.ptyIo != 0 {
		var err error
		// Do NOT use := here, a new pty variable will be created and shadow the outer one.
		pty, err = ptyservice.OpenPtyInContainer(ct)
		if err != nil {
			return 0, err
		}

		spec.ptyPath = pty.Name()

		if spec.ptyIo&stdinPtyFlag != 0 {
			forwardStdinToPty = true
		}

		// If stdout isn't a tty, we can properly redirect stderr to stderr,
		// but otherwise we can't really tell the difference.
		if spec.ptyIo&(stdoutPtyFlag|stderrPtyFlag) != 0 {
			if spec.ptyIo&stdoutPtyFlag == 0 {
				forwardPtyToWriter = os.Stderr
			} else {
				forwardPtyToWriter = os.Stdout
			}
		}
	}

	spec.uid = os.Getuid()
	spec.cwd = workdir
	spec.env = usrdata.Environ
	spec.command = command

	// Set-up the PTY forwarding.
	if spec.ptyIo != 0 {
		if forwardStdinToPty {
			go safeCopy(pty, os.Stdin)
		}

		if forwardPtyToWriter != nil {
			go safeCopy(forwardPtyToWriter, pty)
		}

		oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			log.Debug("failed to make terminal raw", err)
		} else {
			defer terminal.Restore(int(os.Stdin.Fd()), oldState)
		}

		// Trace SIGWINCH to forward the window size.
		sigchan := make(chan os.Signal)
		signal.Notify(sigchan, unix.SIGWINCH)

		defer func() {
			signal.Stop(sigchan)
			close(sigchan)
		}()

		inheritSize := func() {
			for _, file := range stdio {
				if spec.ptyIo&stdioPtyFlags[int(file.Fd())] != 0 {
					krpty.InheritSize(file, pty)
					break
				}
			}
		}

		inheritSize()

		go func() {
			for range sigchan {
				inheritSize()
			}
		}()
	}

	status, err := door.Enter(ct, &spec)
	if err != nil {
		return 0, err
	}

	if status.exitType == processExitSignaled {
		// Mimic the shell's exit code on signal.
		return 128 + status.result, nil
	} else {
		return status.result, nil
	}
}
