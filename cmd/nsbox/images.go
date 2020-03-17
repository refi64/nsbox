/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// XXX: This is really similar to list.go...

package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/image"
)

type imagesCommand struct {
	patterns []string
}

func newImagesCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &imagesCommand{})
}

func (*imagesCommand) Name() string {
	return "images"
}

func (*imagesCommand) Synopsis() string {
	return "list images"
}

func (*imagesCommand) Usage() string {
	return `list [<patterns>...]:
	Lists all the available images. If a pattern is given, list only containers whose names
	match one of the given patterns.
`
}

func (*imagesCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *imagesCommand) ParsePositional(fs *flag.FlagSet) error {
	cmd.patterns = fs.Args()
	return nil
}

func (cmd *imagesCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	var images []*image.Image
	images, err := image.List()
	if err != nil {
		return args.HandleError(err)
	}

	for _, img := range images {
		if len(cmd.patterns) != 0 {
			var match bool

			for _, arg := range fs.Args() {
				match, err = filepath.Match(arg, filepath.Base(img.RootPath))
				if match {
					break
				}

				if err != nil {
					return args.HandleError(err)
				}
			}

			if !match {
				continue
			}
		}

		name := filepath.Base(img.RootPath)

		if len(img.ValidTags) != 0 {
			fmt.Printf("%s:%s\n", name, strings.Join(img.ValidTags, ","))
		} else {
			fmt.Println(name)
		}
	}

	return subcommands.ExitSuccess
}
