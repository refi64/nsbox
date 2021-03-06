/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Bind to the host form within a container.
package session

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func ConnectPtys(stdinPty, stdoutPty, stderrPty string) error {
	// Order is significant, the indexes map to the file descriptors.
	ptys := []string{stdinPty, stdoutPty, stderrPty}

	for fd, pty := range ptys {
		if pty == "" {
			continue
		}

		if fd == 0 {
			if _, err := unix.Setsid(); err != nil {
				if errno, ok := err.(unix.Errno); !ok || errno != unix.EPERM {
					// EPERM means this is already a session leader.
					return errors.Wrapf(err, "failed to setsid")
				}
			}
		}

		// Some ANSI codes can be *written* to stdin, e.g. https://vt100.net/docs/vt510-rm/DECCKM.html
		// In addition, it's possible to *read* from stdin... Therefore, flags is always RDWR.
		ptyFd, err := unix.Open(pty, os.O_RDWR, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to open pty %s for %d", pty, fd)
		}

		if err := unix.Dup2(ptyFd, fd); err != nil {
			return errors.Wrapf(err, "failed to dup %s onto %d", pty, fd)
		}
	}

	return nil
}

func SetupContainerSession(uid int, cwd string, noReplay bool, execCommand []string) error {
	if noReplay {
		if err := os.Setenv("NSBOX_NO_REPLAY", "1"); err != nil {
			return errors.Wrap(err, "set NSBOX_NO_REPLAY")
		}
	}

	script := "/run/host/nsbox/scripts/nsbox-enter-setup.sh"
	execCmdline := append([]string{script, cwd}, execCommand...)
	if err := unix.Exec(script, execCmdline, os.Environ()); err != nil {
		return errors.Wrap(err, "failed to exec command")
	}

	panic("should not reach here")
}
