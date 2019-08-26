/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package inventory

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func List(usrdata *userdata.Userdata) ([]*container.Container, error) {
	containers := []*container.Container{}

	inventory := paths.ContainerInventory(usrdata)
	items, err := ioutil.ReadDir(inventory)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("container directory does not exist")
			return containers, nil
		}

		return nil, errors.Wrap(err, "failed to read container inventory")
	}

	for _, item := range items {
		if strings.HasSuffix(item.Name(), container.StageSuffix) {
			log.Debug("skipping item ", item.Name())
			continue
		}

		stat, err := os.Stat(filepath.Join(inventory, item.Name()))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to stat %s", item.Name())
		}

		if stat.Mode().IsDir() {
			ct, err := container.Open(usrdata, item.Name())
			if err != nil {
				log.Alertf("warning: failed to open %s: %v", item.Name(), err)
				continue
			}

			containers = append(containers, ct)
		} else {
			log.Debug("skipping non-file ", item.Name())
		}
	}

	return containers, nil
}
