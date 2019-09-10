/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/daemon"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
)

func main() {
	log.SetFlags(flag.CommandLine)
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("invalid arguments")
	}

	usrdata, err := userdata.BeneathSudo()
	if err != nil {
		log.Fatal(err)
	}

	ct, err := container.Open(usrdata, flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	ct.ApplyEnvironFilter(usrdata)

	if err := daemon.RunContainerDirectNspawn(ct, usrdata); err != nil {
		log.Fatal(err)
	}
}
