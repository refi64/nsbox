/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// The PTY service runs inside the container, and it's responsible for sending a PTY FD over a
// Unix domain socket, that way nsbox can attach it to the user's terminal session.
package ptyservice

import (
	"fmt"
	"net"
	"os"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"golang.org/x/sys/unix"
)

// If we have a PTY over this size, something is very, very wrong...
// (And if a bug report is filed because *I* was wrong, this comment will have officially aged
// more poorly than the posts /r/iamverysmart of the poster when they were 10 years younger.)
const maxPathSize = 16

func openPty() (int, string, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return 0, "", err
	}

	// We don't need the slave end, but we do need the name.
	slavePath := slave.Name()
	slave.Close()

	return int(master.Fd()), slavePath, nil
}

func handlePtyRequest(conn net.Conn) {
	defer conn.Close()

	fd, path, err := openPty()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open pty: ", err)
		return
	}

	defer unix.Close(fd)

	bytePath := []byte(path)
	if len(bytePath) > maxPathSize {
		fmt.Fprintln(os.Stderr, "path too long: ", path)
		return
	}

	rights := unix.UnixRights(fd)

	connFile, err := conn.(*net.UnixConn).File()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to access file behind connection: ", err)
		return
	}

	if err := unix.Sendmsg(int(connFile.Fd()), []byte(path), rights, nil, 0); err != nil {
		fmt.Fprintln(os.Stderr, "failed to send reply: ", err)
		return
	}
}

func StartPtyService(name string) error {
	socketPath := "/run/host/nsbox/" + paths.PtyServiceSocketName

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old pty service socket")
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return errors.Wrap(err, "failed to listen on pty service socket")
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatal("failed to accept pty service connection: ", err)
				break
			}

			go handlePtyRequest(conn)
		}
	}()

	return nil
}
