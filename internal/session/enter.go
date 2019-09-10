/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Enter a running container session.
package session

import (
	"fmt"
	"github.com/coreos/go-systemd/machine1"
	krpty "github.com/kr/pty"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/ptyservice"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
)

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

func fileIsTerminal(file *os.File) bool {
	return terminal.IsTerminal(int(file.Fd()))
}

func anyFileIsTerminal(files []*os.File) bool {
	for _, file := range files {
		if fileIsTerminal(file) {
			return true
		}
	}

	return false
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

			log.Fatalf("safeCopy %d -> %d failed: %T", source.Fd(), dest.Fd(), err)
		} else {
			hadSuccessYet = true
		}
	}
}

func convertStateToExitCode(state *os.ProcessState) int {
	// XXX: syscall is deprecated, but this cast will fail if it directly jumps to
	// unix.WaitStatus.
	waitStatus := unix.WaitStatus(state.Sys().(syscall.WaitStatus))

	if waitStatus.Signaled() {
		// Treat a signal like most shells do, as 128 + the signal value.
		return 128 + int(waitStatus.Signal())
	} else if waitStatus.Exited() {
		return waitStatus.ExitStatus()
	} else {
		log.Alertf("Unexpected wait status %d", int(waitStatus))
		return 1
	}
}

func EnterContainer(ct *container.Container, command []string, usrdata *userdata.Userdata, workdir string) (int, error) {
	if len(command) == 0 {
		command = []string{"/run/host/login-shell", "-l"}
	}

	leader, err := getLeader(ct.Name)
	if err != nil {
		return 0, err
	}

    // XXX: Okay so PTY handling is a royal mess. nsenter doesn't give us a pty *at all*.
    // Therefore, the plan of action is to ask machined for a pty inside the machine, and let
    // nsbox-enter know where to redirect what.

	// Do not touch this code. If you do, and it breaks, you will be haunted by the ghost of
	// several thousand headless terminals that were spawned and killed during the course of
	// this code's development.

	// NOTE: Order here is important, nsbox-enter.sh assumes it.

	stdio := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	var pty *os.File

	var forwardStdinToPty = false
	var forwardPtyToWriter *os.File

	if anyFileIsTerminal(stdio) {
		pty, err = ptyservice.OpenPtyInContainer(ct)
		if err != nil {
			return 0, err
		}

		if fileIsTerminal(os.Stdin) {
			forwardStdinToPty = true
		}

		// If stdout isn't a tty, we can properly redirect stderr to stderr,
		// but otherwise we can't really tell the difference.
		if fileIsTerminal(os.Stdout) || fileIsTerminal(os.Stderr) {
			if !fileIsTerminal(os.Stdout) {
				forwardPtyToWriter = os.Stderr
			} else {
				forwardPtyToWriter = os.Stdout
			}
		}
	}

	args := []string{"nsenter", "-at", strconv.Itoa(int(leader)), "/run/host/nsbox/nsbox-host", "enter"}

	if log.Verbose() {
		args = append(args, "-v")
	}

	ptyArgMap := []string{"stdin", "stdout", "stderr"}
	for index, file := range stdio {
		if !fileIsTerminal(file) {
			continue
		}

		args = append(args, fmt.Sprintf("-%s=%s", ptyArgMap[index], pty.Name()))
	}

	args = append(args, fmt.Sprintf("-uid=%d", os.Getuid()), fmt.Sprintf("-cwd=%s", workdir))

	// Add the environment variables.
	args = append(args, "env")

	for name, value := range usrdata.Environ {
		args = append(args, fmt.Sprintf("%s=%s", name, value))
	}

	args = append(args, command...)

	log.Debug("running:", args)

	// If there's no pty, we can exec the command directly.
	if pty == nil {
		nsenter, err := exec.LookPath("nsenter")
		if err != nil {
			return 0, errors.Wrap(err, "failed to find nsenter")
		}

		if err := unix.Exec(nsenter, args, os.Environ()); err != nil {
			return 0, errors.Wrap(err, "failed to exec into namespace")
		}

		panic("should not reach here")
	}

	// Forward the PTYs.
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
			if fileIsTerminal(file) {
				krpty.InheritSize(file, pty)
				break
			}
		}
	}

	inheritSize()

	go func() {
		for _ = range sigchan {
			inheritSize()
		}
	}()

	// Finally run nsenter.
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		// Handle ExitError later on.
		if _, ok := err.(*exec.ExitError); !ok {
			return 0, errors.Wrap(err, "failed to enter into namespace")
		}
	}

	return convertStateToExitCode(cmd.ProcessState), nil
}
