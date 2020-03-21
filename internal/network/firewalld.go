/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package network

import (
	godbus "github.com/godbus/dbus"
	"github.com/refi64/nsbox/internal/log"
)

type firewalld struct {
	systemBus *godbus.Conn
	firewalld godbus.BusObject
}

const (
	firewalldService       = "org.fedoraproject.FirewallD1"
	firewalldManagerObject = "/org/fedoraproject/FirewallD1"

	firewalldManagerIface = "org.fedoraproject.FirewallD1"
	firewalldZoneIface    = "org.fedoraproject.FirewallD1.zone"

	firewalldManagerVersionProp        = firewalldManagerIface + ".version"
	firewalldZoneChangeInterfaceMethod = firewalldZoneIface + ".changeZoneOfInterface"
	firewalldZoneRemoveInterfaceMethod = firewalldZoneIface + ".removeInterface"

	defaultCallFlags = godbus.FlagNoAutoStart

	trustedZone = "trusted"
)

func newFirewalld() *firewalld {
	systemBus, err := godbus.SystemBus()
	if err != nil {
		log.Debug("Failed to acquire system bus:", err)
		return nil
	}

	object := systemBus.Object(firewalldService, firewalldManagerObject)
	if _, err := object.GetProperty(firewalldManagerVersionProp); err != nil {
		log.Debug("Failed to get firewalld version:", err)
		return nil
	}

	// firewalld is now confirmed present.
	return &firewalld{systemBus: systemBus, firewalld: object}
}

func (fw *firewalld) TrustInterface(iface string) error {
	call := fw.firewalld.Call(firewalldZoneChangeInterfaceMethod, defaultCallFlags,
		trustedZone, iface)
	return call.Err
}

func (fw *firewalld) UntrustInterface(iface string) error {
	call := fw.firewalld.Call(firewalldZoneRemoveInterfaceMethod, defaultCallFlags,
		trustedZone, iface)
	return call.Err
}

func (fw *firewalld) Close() error {
	if err := fw.systemBus.Close(); err != nil {
		return err
	}

	return nil
}
