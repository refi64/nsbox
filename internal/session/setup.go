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

		var flags int
		if fd == 0 {
			// stdin is readable, not writable.
			flags = unix.O_RDONLY

			if _, err := unix.Setsid(); err != nil {
				return errors.Wrapf(err, "failed to setsid")
			}
		} else {
			flags = unix.O_WRONLY
		}

		ptyFd, err := unix.Open(pty, flags, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to open pty %s for %d", pty, fd)
		}

		if err := unix.Dup2(ptyFd, fd); err != nil {
			return errors.Wrapf(err, "failed to dup %s onto %d", pty, fd)
		}
	}

	return nil
}

func SetupContainerSession(uid int, cwd string, execCommand []string) error {
	script := "/run/host/nsbox/scripts/nsbox-enter-setup.sh"
	execCmdline := append([]string{script, cwd}, execCommand...)
	if err := unix.Exec(script, execCmdline, os.Environ()); err != nil {
		return errors.Wrap(err, "failed to exec command")
	}

	panic("should not reach here")
}
