/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
	"os"
)

func main() {
	// Usage: nsbox-invoker <command> <env vars...> :: <command args...>

	args := os.Args[1:]
	if len(args) < 3 {
		log.Fatal("This is an internal tool!!")
	}

	command := args[0]
	args = args[1:]

	environ := os.Environ()

	for idx, env := range args {
		if env == "::" {
			args = args[idx+1:]
			break
		}

		name, _ := userdata.SplitEnv(env)
		if userdata.IsWhitelisted(name) {
			environ = append(environ, env)
		} else {
			log.Fatal("non-whitelisted environment variable:", env)
		}
	}

	if len(args) == 0 {
		log.Fatal("end of environment not found")
	}

	nsbox, err := paths.GetPathRelativeToInstallRoot(paths.Bin, "nsbox")
	if err != nil {
		log.Fatal("failed to find nsbox binary:", err)
	}

	cmd := []string{nsbox, command}
	cmd = append(cmd, args...)

	err = unix.Exec(cmd[0], cmd, environ)
	log.Fatal("exec:", err)
}
