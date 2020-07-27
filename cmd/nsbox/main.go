/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
)

type nsboxApp struct {
	usrdata *userdata.Userdata
	sudo    bool
	workdir string
}

func (app *nsboxApp) PreexecHook(cmd subcommands.Command, fs *flag.FlagSet) {
	// No auth needed for "version".
	if os.Getuid() == 0 || cmd.Name() == "version" {
		return
	}

	var redirector string
	if app.sudo || os.Getenv("NSBOX_USE_SUDO") == "1" {
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

	/*
		polkit will reset our cwd, so we need to pass -workdir in order to remain in the
		proper directory. However, if -workdir was already passed, then passing it twice
		will give an error, so we ensure it's only passed once by skipping it in Visit.

		Note that VisitAll must *not* be used, because it breaks the checks in config.go
		to only modify boolean settings if they were given on the CLI.
	*/

	visitor := func(f *flag.Flag) {
		if f.Name != "workdir" {
			redirect = append(redirect, fmt.Sprintf("-%s=%s", f.Name, f.Value.String()))
		}
	}

	flag.Visit(visitor)
	fs.Visit(visitor)

	redirect = append(redirect, fmt.Sprintf("-workdir=%s", app.workdir))

	redirect = append(redirect, "--")
	redirect = append(redirect, fs.Args()...)

	log.Debug(redirect)
	err = unix.Exec(redirectorPath, redirect, os.Environ())
	log.Fatal("failed to exec redirect", err)
}

func (app *nsboxApp) SetGlobalFlags(fs *flag.FlagSet) {
	fs.BoolVar(&app.sudo, "sudo", app.sudo, "Use sudo for privilege escalation instead of polkit")
	fs.StringVar(&app.workdir, "workdir", app.workdir, "Run from the given directory")
}

func main() {
	var usrdata *userdata.Userdata
	var err error

	if _, err := os.Stat("/run/host/nsbox"); err == nil {
		log.Fatal("nsbox cannot be run nested")
	}

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
	subcommands.Register(newConfigCommand(app), "")
	subcommands.Register(newCreateCommand(app), "")
	subcommands.Register(newDeleteCommand(app), "")
	subcommands.Register(newImagesCommand(app), "")
	subcommands.Register(newInfoCommand(app), "")
	subcommands.Register(newKillCommand(app), "")
	subcommands.Register(newListCommand(app), "")
	subcommands.Register(newRunCommand(app), "")
	subcommands.Register(newSetDefaultCommand(app), "")
	subcommands.Register(newVersionCommand(app), "")

	args.Execute(app)
}
