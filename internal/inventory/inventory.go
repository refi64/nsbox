/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package inventory

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
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
				log.Alertf("WARNING: failed to open %s: %v", item.Name(), err)
				continue
			}

			containers = append(containers, ct)
		} else {
			log.Debug("skipping non-file ", item.Name())
		}
	}

	return containers, nil
}

func DefaultContainer(usrdata *userdata.Userdata) (*container.Container, error) {
	path := paths.ContainerDefault(usrdata)

	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return nil, nil
	}

	target, err := os.Readlink(path)
	if err != nil {
		return nil, err
	}

	return container.OpenPath(path, filepath.Base(target))
}

func SetDefaultContainer(usrdata *userdata.Userdata, name string) error {
	if name != "" && name != "-" {
		ct, err := container.Open(usrdata, name)
		if err != nil {
			return err
		}

		defaultPath := paths.ContainerDefault(usrdata)
		defaultTmp := defaultPath + ".tmp"

		if err := os.Remove(defaultTmp); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to unlink old temporary default link")
		}

		if err := os.Symlink(ct.Path, defaultTmp); err != nil {
			return errors.Wrap(err, "failed to symlink new temporary default container")
		}

		if err := os.Rename(defaultTmp, defaultPath); err != nil {
			return errors.Wrap(err, "failed to rename temporary link")
		}
	} else {
		if err := os.Remove(paths.ContainerDefault(usrdata)); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to unlink old default container")
		}
	}

	return nil
}
