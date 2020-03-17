/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package ptyservice

import (
	"fmt"
	"net"
	"os"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"golang.org/x/sys/unix"
)

func OpenPtyInContainer(ct *container.Container) (*os.File, error) {
	ptySocketPath := ct.StorageChild(paths.InContainerPrivPath, paths.PtyServiceSocketName)

	conn, err := net.Dial("unix", ptySocketPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to pty service")
	}

	connFile, err := conn.(*net.UnixConn).File()
	if err != nil {
		return nil, errors.Wrap(err, "failed to access connection file")
	}

	// Expecting a max-16 byte path & one 4-byte fd.
	pathBuffer := make([]byte, maxPathSize)
	controlBuffer := make([]byte, unix.CmsgSpace(4))

	// If you look at (_, _, _) long enough it looks like a kaomoji.
	pathLen, _, _, _, err := unix.Recvmsg(int(connFile.Fd()), pathBuffer, controlBuffer, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive pty message")
	}

	path := string(pathBuffer[:pathLen])

	controlMessages, err := unix.ParseSocketControlMessage(controlBuffer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse pty control message")
	}

	if len(controlMessages) != 1 {
		return nil, errors.Errorf("unexpected %f control messages from pty service", len(controlMessages))
	}

	fds, err := unix.ParseUnixRights(&controlMessages[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse pty control rights message")
	}

	if len(fds) != 1 {
		return nil, errors.Errorf("unexpected %d fds from pty service", len(fds))
	}

	fd := fds[0]

	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		panic(fmt.Sprintf("given invalid fd by pty service: ", fd))
	}

	log.Debugf("pty service sent %d, %s", fd, path)

	return file, nil
}
