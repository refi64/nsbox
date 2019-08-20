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
	verbose := flag.Bool("verbose", false, "verbose mode")
	flag.Parse()
	log.SetVerbose(*verbose)

	if flag.NArg() != 1 {
		log.Fatal("invalid arguments")
	}

	ct, err := container.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	usrdata, err := userdata.BeneathSudo()
	if err != nil {
		log.Fatal(err)
	}

	if err := daemon.RunContainerDirectNspawn(ct, usrdata); err != nil {
		log.Fatal(err)
	}
}
