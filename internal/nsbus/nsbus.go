/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package nsbus

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	"github.com/refi64/nsbox/internal/log"
	"golang.org/x/sys/unix"
)

/*
	nsbus works by entering the mount namespace in a forked subprocess and
	connecting to the D-Bus socket there. However, godbus's NewConn will not
	give us a UNIX-style transport if we immediately create a connection from
	the new fd, thus making passing of file descriptors broken. In order to
	rectify this, a private (thanks to PrivateTmp=true in transient.go) D-Bus
	socket is created that just forwards everything to the true socket, and
	the D-Bus connection is created by dialing into the forwarding socket
	instead of the true one.
*/

const busPath = "/run/dbus/system_bus_socket"
const privateBusPath = "/tmp/nsbox_container_bus_socket"

// Use an error value to propagate the fd up from inside the dialing.
type dialSuccessError struct {
	fd uintptr
}

func (e *dialSuccessError) Error() string {
	return "dial success"
}

//go:linkname runtime_BeforeFork syscall.runtime_BeforeFork
func runtime_BeforeFork()

//go:linkname runtime_AfterFork syscall.runtime_AfterFork
func runtime_AfterFork()

//go:linkname runtime_AfterForkInChild syscall.runtime_AfterForkInChild
func runtime_AfterForkInChild()

func forkAndRunChild(nsfd, sock int, sockaddr *unix.RawSockaddrUnix, sockaddrLen uintptr) (uintptr, unix.Errno) {
	var child uintptr
	var syserr unix.Errno

	runtime_BeforeFork()
	child, _, syserr = unix.RawSyscall(unix.SYS_CLONE, uintptr(unix.SIGCHLD), 0, 0)
	if child != 0 || syserr != 0 {
		return child, syserr
	}

	runtime_AfterForkInChild()

	if _, _, syserr = unix.RawSyscall(unix.SYS_SETNS, uintptr(nsfd), uintptr(unix.CLONE_NEWNS), 0); syserr != 0 {
		unix.Exit(int(syserr))
	}

	if _, _, syserr = unix.RawSyscall(unix.SYS_CONNECT,
		uintptr(sock), uintptr(unsafe.Pointer(sockaddr)), sockaddrLen); syserr != 0 {
		unix.Exit(128 + int(syserr))
	}

	unix.Exit(0)
	panic("exit failed?")
}

func copyForever(dest io.Writer, src io.Reader) {
	for {

		if _, err := io.Copy(dest, src); err != nil {
			log.Fatal("Copying D-Bus forwarder connection data:", err)
		}
	}
}

func forwardListenerToSocket(listener net.Listener, sockfile *os.File) {
	conn, err := listener.Accept()
	listener.Close()
	if err != nil {
		sockfile.Close()
		log.Fatal("Accepting D-Bus forwarder connection:", err)
	}

	go copyForever(conn, sockfile)
	go copyForever(sockfile, conn)
}

func DialBusInsideNamespace(nspid int) (*dbus.Conn, error) {
	// We don't worry about closing the directory, as it will be gone safely
	// the moment that the nsbox service exits anyway.

	forwardSockDir, err := lxtempdir.Create("", "nsbox-socket")
	if err != nil {
		return nil, errors.Wrap(err, "craete nsbus socket dir")
	}

	forwardSockPath := filepath.Join(forwardSockDir.Path, "bus")

	nsfd, err := unix.Open(fmt.Sprintf("/proc/%d/ns/mnt", nspid), unix.O_RDONLY, 0)
	if err != nil {
		return nil, errors.Wrap(err, "open mnt ns fd")
	}
	defer unix.Close(nsfd)

	var sockfile *os.File

	sock, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, errors.Wrap(err, "create socket")
	}
	defer func() {
		if sockfile == nil {
			unix.Close(sock)
		}
	}()

	var sockaddr unix.RawSockaddrUnix
	sockaddr.Family = unix.AF_UNIX
	busPathBytes := []byte(busPath)
	copy(sockaddr.Path[:], *(*[]int8)(unsafe.Pointer(&busPathBytes)))
	sockaddr.Path[len(busPathBytes)] = 0

	sockaddrLen := unsafe.Sizeof(sockaddr.Family) + uintptr(len(busPathBytes)+1)

	child, syserr := forkAndRunChild(nsfd, sock, &sockaddr, sockaddrLen)
	runtime_AfterFork()

	if syserr != 0 {
		return nil, errors.Wrap(syserr, "fork")
	}

	process, err := os.FindProcess(int(child))
	if err != nil {
		log.Fatal("FindProcess returned unexpected error:", err)
	}

	state, err := process.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "wait for bus connector")
	}

	if !state.Exited() || state.ExitCode() != 0 {
		return nil, errors.Errorf("bus connector: %v", state)
	}

	forwardListener, err := net.Listen("unix", forwardSockPath)
	if err != nil {
		unix.Close(sock)
		return nil, errors.Wrap(err, "forward listener")
	}

	sockfile = os.NewFile(uintptr(sock), "d-bus forward socket")

	bus, err := dbus.Dial("unix:path=" + forwardSockPath)
	if err != nil {
		unix.Close(sock)
		forwardListener.Close()
		return nil, errors.Wrap(err, "dialing forwarded bus socket")
	}

	go forwardListenerToSocket(forwardListener, sockfile)
	return bus, nil
}
