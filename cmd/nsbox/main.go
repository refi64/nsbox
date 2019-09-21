/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
)

type nsboxApp struct {
	usrdata *userdata.Userdata
	sudo    bool
	workdir string
}

func (app *nsboxApp) PreexecHook(cmd subcommands.Command, fs *flag.FlagSet) {
	if os.Getuid() == 0 {
		return
	}

	var redirector string
	if app.sudo {
		redirector = "sudo"
	} else {
		redirector = "pkexec"
	}

	redirectorPath, err := exec.LookPath(redirector)
	if err != nil {
		log.Fatalf("failed to locate %s: %v", redirector, err)
	}

	invokerPath, err := paths.GetPathRelativeToInstallRoot(paths.Libexec, paths.ProductName, "nsbox-invoker")
	if err != nil {
		log.Fatal("failed to get invoker path:", err)
	}

	redirect := []string{redirector, invokerPath, cmd.Name()}
	redirect = append(redirect, userdata.WhitelistedEnviron()...)
	redirect = append(redirect, "::")

	fs.VisitAll(func(f *flag.Flag) {
		redirect = append(redirect, fmt.Sprintf("-%s=%s", f.Name, f.Value.String()))
	})

	redirect = append(redirect, "--")
	redirect = append(redirect, fs.Args()...)

	err = unix.Exec(redirectorPath, redirect, os.Environ())
	log.Fatal("failed to exec redirect", err)
}

func (app *nsboxApp) SetGlobalFlags(fs *flag.FlagSet) {
	fs.BoolVar(&app.sudo, "sudo", app.sudo, "Use sudo for privilege escalation over polkit")
	fs.StringVar(&app.workdir, "workdir", app.workdir, "Run from the given directory")
}

func main() {
	var usrdata *userdata.Userdata
	var err error

	if os.Getuid() == 0 {
		usrdata, err = userdata.BeneathSudo()
	} else {
		usrdata, err = userdata.Current()
	}

	if err != nil {
		log.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("failed to get cwd:", err)
	}

	app := &nsboxApp{usrdata: usrdata, workdir: cwd}

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(newCreateCommand(app), "")
	subcommands.Register(newInfoCommand(app), "")
	subcommands.Register(newKillCommand(app), "")
	subcommands.Register(newListCommand(app), "")
	subcommands.Register(newRunCommand(app), "")
	subcommands.Register(newSetDefaultCommand(app), "")

	args.Execute(app)
}
