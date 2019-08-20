/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/run"
	"github.com/refi64/nsbox/internal/userdata"
	"os"
	"path/filepath"
)

func main() {
	verbose := flag.Bool("verbose", false, "verbose mode")
	flag.Parse()
	log.SetVerbose(*verbose)

	if flag.NArg() != 1 {
		log.Fatal("invalid arguments")
	}

	containerName := flag.Arg(0)
	containerPath := filepath.Join(paths.ContainerStorage, containerName)
	if _, err := os.Stat(containerPath); err != nil {
		log.Fatal(err)
	}

	usrdata, err := userdata.BeneathSudo()
	if err != nil {
		log.Fatal(err)
	}

	if err := run.RunContainerDirectNspawn(containerName, containerPath, usrdata); err != nil {
		log.Fatal(err)
	}
}
