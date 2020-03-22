/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package varlinkhost

import (
	"context"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/integration"
	"github.com/refi64/nsbox/internal/log"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
)

type VarlinkHost struct {
	devnsbox.VarlinkInterface

	container *container.Container
}

func (host *VarlinkHost) NotifyStart(ctx context.Context, call devnsbox.VarlinkCall) error {
	log.Debug("received NotifyStart()")

	if _, err := daemon.SdNotify(true, daemon.SdNotifyReady); err != nil {
		log.Alert("notifying systemd of start", err)
		return err
	}

	return call.ReplyNotifyStart(ctx)
}

func (host *VarlinkHost) NotifyReloadExports(ctx context.Context, call devnsbox.VarlinkCall) error {
	log.Debug("received NotifyReloadExports()")

	if err := integration.UpdateDesktopFiles(host.container); err != nil {
		log.Alert("updating desktop files", err)
		return err
	}

	return call.ReplyNotifyReloadExports(ctx)
}

func New(ct *container.Container) *devnsbox.VarlinkInterface {
	host := VarlinkHost{container: ct}
	return devnsbox.VarlinkNew(&host)
}
