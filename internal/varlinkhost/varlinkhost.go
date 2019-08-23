/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package varlinkhost

import (
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
	"github.com/coreos/go-systemd/daemon"
)

type VarlinkHost struct {
	devnsbox.VarlinkInterface

	ct *container.Container
}

func (host *VarlinkHost) NotifyStart(call devnsbox.VarlinkCall) error {
	log.Debug("received NotifyStart()")

	if _, err := daemon.SdNotify(true, daemon.SdNotifyReady); err != nil {
		return err
	}

	return call.ReplyNotifyStart()
}

func (host *VarlinkHost) NotifyDesktopUpdate(call devnsbox.VarlinkCall) error {
	log.Debug("received NotifyDesktopUpdate() STUB")

	return call.ReplyNotifyDesktopUpdate()
}

func New(ct *container.Container) *devnsbox.VarlinkInterface {
	host := VarlinkHost{ct: ct}
	return devnsbox.VarlinkNew(&host)
}
