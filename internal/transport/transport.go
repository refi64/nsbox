/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package transport

import (
	"encoding/binary"
	"encoding/json"
	_ "encoding/json"
	"net"
	_ "net"
	"os"
	_ "os"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"golang.org/x/sys/unix"
)

type UnixFd struct {
	fd int
}

const emptyFd = -1

func (ufd *UnixFd) Close() {
	if ufd.fd != emptyFd {
		if err := unix.Close(ufd.fd); err != nil {
			log.Alertf("failed to close %d: %v", ufd.fd, err)
		}

		ufd.fd = emptyFd
	}
}

func (ufd UnixFd) IsEmpty() bool {
	return ufd.fd == emptyFd
}

func (ufd UnixFd) PeekFd() int {
	return ufd.fd
}

func (ufd *UnixFd) TakeFd() int {
	fd := ufd.fd
	ufd.fd = emptyFd
	return fd
}

type MessageSlot struct {
	Message interface{}
	fds     []UnixFd
}

func NewMessageSlot(message interface{}) *MessageSlot {
	return &MessageSlot{Message: message}
}

func (m *MessageSlot) Destroy() {
	for _, ufd := range m.fds {
		ufd.Close()
	}
}

func (m *MessageSlot) TakeUnixFd(fd int) {
	m.fds = append(m.fds, UnixFd{fd: fd})
}

type Channel struct {
	conn     net.Conn
	connFile *os.File
}

func NewChannel(conn net.Conn) (*Channel, error) {
	connFile, err := conn.(*net.UnixConn).File()
	if err != nil {
		return nil, errors.Wrap(err, "access connection file")
	}

	return &Channel{conn: conn, connFile: connFile}, nil
}

// FDs are sent as 32-bit unsigned integers
const fdSize = 4

type messageHeader struct {
	size uint64
	fds  uint64
}

// Two 8-byte integers -> 16 bytes total
const messageHeaderSize = 16

func newMessageHeaderBuffer() []byte {
	return make([]byte, messageHeaderSize)
}

func parseMessageHeader(buffer []byte) *messageHeader {
	return &messageHeader{
		size: binary.LittleEndian.Uint64(buffer[:8]),
		fds:  binary.LittleEndian.Uint64(buffer[8:]),
	}
}

func putMessageHeader(hdr messageHeader) []byte {
	buffer := newMessageHeaderBuffer()
	binary.LittleEndian.PutUint64(buffer[:8], hdr.size)
	binary.LittleEndian.PutUint64(buffer[8:], hdr.fds)
	return buffer
}

func (ch *Channel) Send(slot *MessageSlot) error {
	data, err := json.Marshal(slot.Message)
	if err != nil {
		return errors.Wrap(err, "marshalling message")
	}

	fds := []int{}
	for _, ufd := range slot.fds {
		fds = append(fds, ufd.fd)
	}

	hdr := messageHeader{size: uint64(len(data)), fds: uint64(len(fds))}
	hdrBuffer := putMessageHeader(hdr)

	if err := unix.Sendmsg(int(ch.connFile.Fd()), hdrBuffer, nil, nil, 0); err != nil {
		return errors.Wrap(err, "sending header")
	}

	rights := unix.UnixRights(fds...)
	if err := unix.Sendmsg(int(ch.connFile.Fd()), data, rights, nil, 0); err != nil {
		return errors.Wrap(err, "sending data")
	}

	return nil
}

func (ch *Channel) Recv(slot *MessageSlot) error {
	hdrBuffer := newMessageHeaderBuffer()
	hdrLen, _, _, _, err := unix.Recvmsg(int(ch.connFile.Fd()), hdrBuffer, nil, 0)
	if err != nil {
		return errors.Wrap(err, "receiving size")
	}

	if hdrLen != messageHeaderSize {
		return errors.New("invalid header size")
	}

	hdr := parseMessageHeader(hdrBuffer[:hdrLen])

	data := make([]byte, hdr.size)
	controlBuffer := make([]byte, unix.CmsgSpace(int(fdSize*hdr.fds)))

	dataLen, _, _, _, err := unix.Recvmsg(int(ch.connFile.Fd()), data, controlBuffer, 0)
	if err != nil {
		return errors.Wrap(err, "receiving message")
	}

	if dataLen != int(hdr.size) {
		return errors.New("invalid data size")
	}

	if err := json.Unmarshal(data, &slot.Message); err != nil {
		return errors.Wrap(err, "unmarshalling message")
	}

	controlMessages, err := unix.ParseSocketControlMessage(controlBuffer)
	if err != nil {
		return errors.Wrap(err, "parsing control message")
	}

	if len(controlMessages) != 1 {
		return errors.Errorf("unexpected # of control messages: %d", len(controlMessages))
	}

	fds, err := unix.ParseUnixRights(&controlMessages[0])
	if err != nil {
		return errors.Wrap(err, "parsing unix rights")
	}

	for _, fd := range fds {
		slot.fds = append(slot.fds, UnixFd{fd: fd})
	}

	return nil
}

type Server struct {
	listener net.Listener
}

func NewServer(listener net.Listener) *Server {
	return &Server{listener: listener}
}

func Accept(server *Server) (*Channel, error) {
	conn, err := server.listener.Accept()
	if err != nil {
		return nil, err
	}

	return NewChannel(conn)
}
