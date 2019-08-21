/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"os"
	"path/filepath"
)

type Config struct {
	Boot bool
}

type Container struct {
	Name string
	Path string
	Config *Config
}

const configJson = "config.json"

func Create(name string, initialConfig Config) (*Container, error) {
	path := paths.ContainerData(name)
	configPath := filepath.Join(path, configJson)

	if _, err := os.Stat(path); err != nil && os.IsExist(err) {
		return nil, errors.Errorf("container %s already exists", name)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create container directory")
	}

	file, err := os.Create(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container config")
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(initialConfig); err != nil {
		return nil, errors.Wrap(err, "failed to save container config")
	}

	return &Container{
		Name: name,
		Path: path,
		Config: &initialConfig,
	}, nil
}

func Open(name string) (*Container, error) {
	path := paths.ContainerData(name)
	configPath := filepath.Join(path, configJson)

	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read container config")
	}

	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.Wrap(err, "failed to parse container config")
	}

	return &Container{
		Name: name,
		Path: path,
		Config: &config,
	}, nil
}

func (container Container) ApplyEnvironFilter(usrdata *userdata.Userdata) {
	if container.Config.Boot {
		delete(usrdata.Environ, "XDG_VTNR")
	}
}

func (container Container) Storage() string {
	return filepath.Join(container.Path, "storage")
}

func (container Container) StorageChild(children ...string) string {
	parts := append([]string{container.Storage()}, children...)
	return filepath.Join(parts...)
}

func (container Container) TempStorageChild(children ...string) string {
	parts := append([]string{container.TempStorage()}, children...)
	return filepath.Join(parts...)
}

func (container Container) TempStorage() string {
	return filepath.Join(container.Path, "storage.tmp")
}
