/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package varlinkhost

import (
	devnsbox "github.com/refi64/nsbox/internal/varlink"
	"github.com/coreos/go-systemd/daemon"
)

type VarlinkHost struct {
	devnsbox.VarlinkInterface
}

func (host *VarlinkHost) NotifyStart(call devnsbox.VarlinkCall) error {
	if _, err := daemon.SdNotify(true, daemon.SdNotifyReady); err != nil {
		return err
	}

	return call.ReplyNotifyStart()
}

func New() *devnsbox.VarlinkInterface {
	host := VarlinkHost{}
	return devnsbox.VarlinkNew(&host)
}
