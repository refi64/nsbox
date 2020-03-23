/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package log

import (
	"flag"
	"fmt"
	"os"
)

// Why not use another logger? Well, we have to specific requirements we want to settle:
// - We don't want a timestamp prefix, since logs will ever only go to the CLI (where the prefix
// 	 is insignificant) or to the journal (where timestamps are already added).
// - We want basic leveled logs.
// - We *don't* need overly fancy functionality.
// glog doesn't allow disabling the timestamps, logrus outputs weird stuff when logging to a
// non-TTY (the journal), etc.

var verbose bool

func SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&verbose, "v", verbose, "Be verbose")
}

func Verbose() bool {
	return verbose
}

func SetVerbose(newVerbose bool) {
	verbose = newVerbose
}

func Info(args ...interface{}) {
	fmt.Println(args...)
}

func Infof(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func Debug(args ...interface{}) {
	if verbose {
		Alert(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if verbose {
		Alertf(format, args...)
	}
}

func Alert(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func Alertf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func Fatal(args ...interface{}) {
	Alert(args...)
	os.Exit(1)
}

func Fatalf(format string, args ...interface{}) {
	Alertf(format, args...)
	os.Exit(1)
}
