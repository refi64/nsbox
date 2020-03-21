/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package nspawn

import (
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

const NetworkZonePrefix = "vz-"

type BindMount struct {
	Host      string
	Dest      string
	Recursive bool
}

// Builds a systemd-nspawn command line.
type Builder struct {
	nspawn string

	Quiet            bool
	AsPid2           bool
	Boot             bool
	KeepUnit         bool
	NetworkVeth      bool
	NetworkZone      string
	MachineDirectory string
	LinkJournal      string
	MachineName      string
	Hostname         string
	Capabilities     []string
	SystemCallFilter string
	Binds            []BindMount
	Command          []string
}

func NewBuilder() (*Builder, error) {
	nspawn, err := exec.LookPath("systemd-nspawn")
	if err != nil {
		return nil, errors.New("systemd-nspawn is unavailable")
	}

	return &Builder{
		nspawn: nspawn,
	}, nil
}

func (builder *Builder) AddBind(path string) {
	builder.AddBindTo(path, path)
}

func (builder *Builder) AddRecursiveBind(path string) {
	builder.AddRecursiveBindTo(path, path)
}

func (builder *Builder) AddBindTo(host string, dest string) {
	builder.AddBindFull(host, dest, false)
}

func (builder *Builder) AddRecursiveBindTo(host string, dest string) {
	builder.AddBindFull(host, dest, true)
}

func (builder *Builder) AddBindFull(host string, dest string, recursive bool) {
	builder.Binds = append(builder.Binds, BindMount{
		Host:      host,
		Dest:      dest,
		Recursive: recursive,
	})
}

func addArg(target *[]string, arg string) {
	*target = append(*target, "--"+arg)
}

func addArgValue(target *[]string, arg, value string) {
	addArg(target, arg+"="+value)
}

func maybeAddArgValue(target *[]string, arg string, value string) {
	if value != "" {
		addArgValue(target, arg, value)
	}
}

func escapeMountPath(path string) (result string) {
	result = path
	result = strings.ReplaceAll(result, `\`, `\\`)
	result = strings.ReplaceAll(result, `:`, `\:`)
	return
}

func (builder *Builder) Build() []string {
	if builder.MachineDirectory == "" {
		panic(errors.New("MachineDirectory must be set"))
	}

	args := []string{builder.nspawn}

	if builder.Quiet {
		addArg(&args, "quiet")
	}

	if builder.AsPid2 {
		addArg(&args, "as-pid2")
	}

	if builder.Boot {
		addArg(&args, "boot")
	}

	if builder.KeepUnit {
		addArg(&args, "keep-unit")
	}

	if builder.NetworkVeth {
		addArg(&args, "network-veth")
	}

	addArgValue(&args, "directory", builder.MachineDirectory)
	maybeAddArgValue(&args, "link-journal", builder.LinkJournal)
	maybeAddArgValue(&args, "machine", builder.MachineName)
	maybeAddArgValue(&args, "hostname", builder.Hostname)
	maybeAddArgValue(&args, "network-zone", builder.NetworkZone)
	maybeAddArgValue(&args, "system-call-filter", builder.SystemCallFilter)

	for _, capability := range builder.Capabilities {
		addArgValue(&args, "capability", capability)
	}

	for _, bind := range builder.Binds {
		dest := escapeMountPath(bind.Dest)
		host := escapeMountPath(bind.Host)

		var opts string
		if bind.Recursive {
			opts = "rbind"
		} else {
			opts = "norbind"
		}

		spec := strings.Join([]string{host, dest, opts}, ":")
		addArgValue(&args, "bind", spec)
	}

	return append(args, builder.Command...)
}
