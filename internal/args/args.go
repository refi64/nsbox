/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package args

import (
	"context"
	"flag"
	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"os"
)

type App interface {
	PreexecHook(cmd subcommands.Command, fs *flag.FlagSet)
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
	ParsePositional(fs *flag.FlagSet) error
	Execute(app App, fs *flag.FlagSet) subcommands.ExitStatus
}

type simpleCommandWrapper struct {
	app    App
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
	if err := wrapper.simple.ParsePositional(fs); err != nil {
		log.Alert(err)
		fs.Usage()
		return subcommands.ExitUsageError
	}

	wrapper.app.PreexecHook(wrapper, fs)
	return wrapper.simple.Execute(wrapper.app, fs)
}

func ExpectArgs(fs *flag.FlagSet, args ...*string) error {
	if fs.NArg() != len(args) {
		return errors.Errorf("expected %d arg(s), got %d", len(args), fs.NArg())
	}

	for i, arg := range fs.Args() {
		*args[i] = arg
	}

	return nil
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
