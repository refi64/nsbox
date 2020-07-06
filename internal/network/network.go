/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package network

import (
	"math/rand"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

type Firewall interface {
	TrustInterface(string) error
	UntrustInterface(string) error
	Close() error
}

func GetFirewall() Firewall {
	if fw := newFirewalld(); fw != nil {
		return fw
	}

	return nil
}

// Short prefix, because otherwise the name will be too long (IFNAMSIZ is only 16).
const nsboxPrefix = "nx-"

func GenerateUniqueLinkName(base string, assumedPrefix string) (string, error) {
	// XXX: This is a tad racy, as it's technically possible someone else
	// might claim this link name right before we do.
	// The "algorithm" is also stupid as heck, but it should generally work.

	links, err := netlink.LinkList()
	if err != nil {
		return "", errors.Wrap(err, "getting netlink list")
	}

	linkNames := map[string]interface{}{}
	for _, link := range links {
		linkNames[link.Attrs().Name] = nil
	}

	maxBaseLength := netlink.IFNAMSIZ - len(assumedPrefix) - len(nsboxPrefix) - 1

	name := nsboxPrefix
	if len(base) > maxBaseLength {
		name += base[:maxBaseLength]
	} else {
		name += base
	}

	unique := false

	for i := len(name) - 1; i >= len(nsboxPrefix); i-- {
		if _, exists := linkNames[name]; exists {
			randLetter := rune(rand.Intn(26) + 'a')
			name = name[:i] + string(randLetter) + name[i+1:]
		} else {
			unique = true
			break
		}
	}

	if !unique {
		return "", errors.Errorf("could not generate unique interface name for %s", base)
	}

	return name, nil
}
