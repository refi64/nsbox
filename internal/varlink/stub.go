/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

/*
	This is here for two reason:
		- So 'go mod' won't get confused by an empty directory.
		- To make IDE autocomplete / analysis work properly.
	It contains a stub subset of the methods in the full generated Varlink interface
*/

package devnsbox

import (
	"context"

	"github.com/varlink/go/varlink"
)

type NotifyStart_methods interface {
	Call(ctx context.Context, c *varlink.Connection) error
}

func NotifyStart() NotifyStart_methods

type NotifyReloadExports_methods interface {
	Call(ctx context.Context, c *varlink.Connection) error
}

func NotifyReloadExports() NotifyReloadExports_methods

type VarlinkCall interface {
	ReplyNotifyStart(ctx context.Context) error
	ReplyNotifyReloadExports(ctx context.Context) error
}

type iface interface {
	NotifyStart(ctx context.Context, c VarlinkCall) error
	NotifyReloadExports(ctx context.Context, c VarlinkCall) error
}

type VarlinkInterface struct {
	iface
}

func (VarlinkInterface) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error
func (VarlinkInterface) VarlinkGetName() string
func (VarlinkInterface) VarlinkGetDescription() string

func VarlinkNew(m iface) *VarlinkInterface { return nil }
