/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/create"
	"github.com/refi64/nsbox/internal/daemon"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/kill"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/session"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/exec"
)

// Get the userdata so it can be stored as a top-level for kingpin's defaults.
func getUserdata() *userdata.Userdata {
	var usrdata *userdata.Userdata
	var err error

	if os.Getuid() == 0 {
		usrdata, err = userdata.BeneathSudo()
	} else {
		usrdata, err = userdata.Current()
	}

	if err != nil {
		panic(err)
	}

	return usrdata
}

// Get the default working directory, again for default argument purposes.
func getWorkdir() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return cwd
}

var (
	usrdata = getUserdata()
	defaultContainerName = "toolbox-" + usrdata.User.Username

	app = kingpin.New("nsbox", "A lightweight, systemd-nspawn-powered toolbox")
	verbose = app.Flag("verbose", "Enable verbose mode").Short('v').Bool()
	sudo = app.Flag("sudo", "Use sudo for privilege escalation").Bool()
	environ = app.Flag("environ", "").Hidden().String()
	workdir = app.Flag("workdir", "The working directory").Short('w').String()

	createCommand = app.Command("create", "Create a new container")
	createContainer = createCommand.Flag("container", "The container name").
		Short('c').Default(defaultContainerName).String()
	createVersion = createCommand.Flag("version", "The Fedora version to use").Int()
	createBoot = createCommand.Flag("boot", "Have the container run its own init").Bool()

	listCommand = app.Command("list", "List the installed containers")

	infoCommand = app.Command("info", "Show info about a container")
	infoContainer = infoCommand.Arg("container", "The container to show info about").String()

	runCommand = app.Command("run", "Run a container")
	runExec = runCommand.Arg("exec", "The command to run inside the container").Strings()
	runContainer = runCommand.Flag("container", "The container name").
		Short('c').Default(defaultContainerName).String()

	killCommand = app.Command("kill", "Kill a container")
	killContainer = killCommand.Arg("container", "The container name").String()
	killSignal = killCommand.Flag("signal", "The signal to kill with").
		Enum("SIGTERM", "SIGKILL", "POWEROFF", "REBOOT")
	killAll = killCommand.Flag("all", "Send to all processes, not just the leader").
		Short('a').Bool()
)

func reexecWithEscalatedPrivileges() {
	var redirector string
	if *sudo {
		redirector = "sudo"
	} else {
		redirector = "pkexec"
	}

	redirectorPath, err := exec.LookPath(redirector)
	if err != nil {
		log.Fatalf("failed to locate %s: %v", redirector, err)
	}

	self, err := paths.GetExecutablePath()
	if err != nil {
		log.Fatal("failed to get executable path: ", err)
	}

	redirect := []string{redirector, "env"}
	redirect = append(redirect, userdata.WhitelistedEnviron()...)
	redirect = append(redirect, self)

	// pkexec resets our working directory. Therefore, when re-exec'd, we need to pass our
	// current directory. However, if a working directory was already given, we'd be using
	// that anyway, so there's no need to be fancy.
	if *workdir == "" {
		redirect = append(redirect, "--workdir", getWorkdir())
	}

	redirect = append(redirect, os.Args[1:]...)
	err = unix.Exec(redirectorPath, redirect, os.Environ())
	log.Fatal("failed to redirect: ", err)
}

func main() {
	app.HelpFlag.Short('h')

	// We parse first, that way the user isn't entering any credentials just to get an
	// argument error.

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	log.SetVerbose(*verbose)

	if os.Getuid() != 0 {
		reexecWithEscalatedPrivileges()
	}

	var err error

	switch cmd {
	case createCommand.FullCommand():
		var version string
		if *createVersion == 0 {
			version = ""
		} else {
			version = string(*createVersion)
		}

		config := container.Config{
			Boot: *createBoot,
		}
		err = create.CreateContainer(usrdata, *createContainer, version, config)

	case listCommand.FullCommand():
		var containers []*container.Container
		containers, err := inventory.List(usrdata)
		if err == nil {
			for _, ct := range containers {
				log.Info(ct.Name)
			}
		}

	case infoCommand.FullCommand():
		err = container.OpenAndShowInfo(usrdata, *infoContainer)

	case runCommand.FullCommand():
		err = daemon.RunContainerViaTransientUnit(*runContainer, usrdata)
		if err == nil {
		 if *workdir == "" {
			 *workdir = getWorkdir()
		 }

		 var exitCode int
		 exitCode, err = session.EnterContainer(*runContainer, *runExec, usrdata, *workdir)

		 if err == nil {
			 os.Exit(exitCode)
		 }
		}

	case killCommand.FullCommand():
		err = kill.KillContainer(usrdata, *killContainer, *killSignal, *killAll)
	}

	if err != nil {
		log.Fatal(err)
	}
}
