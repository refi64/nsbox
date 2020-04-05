/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package selinux

import (
	"io"
	"strings"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
)

const systemRole = "system_r"
const spcType = "spc_t"

func Enforcing() bool {
	return selinux.EnforceMode() == selinux.Enforcing
}

func Enabled() bool {
	return selinux.EnforceMode() != selinux.Disabled
}

func GetCurrentLabel() (string, error) {
	label, err := selinux.ExecLabel()
	if err == io.EOF {
		label, err = selinux.CurrentLabel()
	}

	return label, err
}

func GetExecLabel(currentLabel string) (string, error) {
	parts := strings.Split(currentLabel, ":")
	if len(parts) != 4 && len(parts) != 5 {
		return "", errors.Errorf("invalid SELinux label: %s")
	}

	parts[1] = systemRole
	parts[2] = spcType

	return strings.Join(parts, ":"), nil
}

func SetExecProcessContextContainer() error {
	if !Enabled() {
		log.Debug("SELinux is disabled")
		return nil
	}

	currentLabel, err := GetCurrentLabel()
	if err != nil {
		return errors.Wrap(err, "get current label")
	}

	newLabel, err := GetExecLabel(currentLabel)
	log.Debug("SELinux exec transition", currentLabel, "->", newLabel)

	if err := selinux.SetExecLabel(newLabel); err != nil {
		return errors.Wrap(err, "failed to set label")
	}

	return nil
}
