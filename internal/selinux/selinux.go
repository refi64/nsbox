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

func SetExecProcessContextContainer() error {
	if selinux.EnforceMode() == selinux.Disabled {
		log.Debug("SELinux is disabled")
		return nil
	}

	label, err := selinux.ExecLabel()
	if err == io.EOF {
		label, err = selinux.CurrentLabel()
	}

	if err != nil {
		return errors.Wrap(err, "failed to get label")
	}

	parts := strings.Split(label, ":")
	if len(parts) != 4 && len(parts) != 5 {
		return errors.Errorf("invalid SELinux label: %s")
	}

	parts[1] = systemRole
	parts[2] = spcType

	newLabel := strings.Join(parts, ":")
	log.Debug("SELinux exec transition", label, "->", newLabel)

	if err := selinux.SetExecLabel(newLabel); err != nil {
		return errors.Wrap(err, "failed to set label")
	}

	return nil
}
