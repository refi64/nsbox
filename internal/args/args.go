/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package args

import (
	"flag"
	"context"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/log"
	"os"
)

type App interface {
	SetGlobalFlags(fs *flag.FlagSet)
}

func setGlobalFlags(app App, fs *flag.FlagSet) {
	log.SetFlags(fs)
	app.SetGlobalFlags(fs)
}

// A wrapper over subcommands.Command with a slightly simplified API.
type SimpleCommand interface {
	Name() string
	Synopsis() string
	Usage() string
	SetFlags(fs *flag.FlagSet)
	Execute(fs *flag.FlagSet) subcommands.ExitStatus
}

type simpleCommandWrapper struct {
	app App
	simple SimpleCommand
}

func WrapSimpleCommand(app App, simple SimpleCommand) subcommands.Command {
	return &simpleCommandWrapper{app, simple}
}

func (wrapper *simpleCommandWrapper) Name() string {
	return wrapper.simple.Name()
}

func (wrapper *simpleCommandWrapper) Synopsis() string {
	return wrapper.simple.Synopsis()
}

func (wrapper *simpleCommandWrapper) Usage() string {
	return wrapper.simple.Usage()
}

func (wrapper *simpleCommandWrapper) SetFlags(fs *flag.FlagSet) {
	setGlobalFlags(wrapper.app, fs)
	wrapper.simple.SetFlags(fs)
}

func (wrapper *simpleCommandWrapper) Execute(_ context.Context, fs *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	return wrapper.simple.Execute(fs)
}

func ExpectArgs(fs *flag.FlagSet, args ...*string) bool {
	if fs.NArg() != len(args) {
		log.Alertf("expected %d arg(s), got %d", len(args), fs.NArg())
		fs.Usage()
		return false
	}

	for i, arg := range fs.Args() {
		*args[i] = arg
	}

	return true
}

func HandleError(err error) subcommands.ExitStatus {
	if err != nil {
		log.Alert(err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func Execute(app App) {
	setGlobalFlags(app, flag.CommandLine)

	flag.Parse()

	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
