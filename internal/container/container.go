/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
const StageSuffix = ".stage"

func validateName(name string) error {
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name); !matched {
		return errors.Errorf("invalid container name: %s", name)
	}

	return nil
}

func CreateStaged(usrdata *userdata.Userdata, name string, initialConfig Config) (*Container, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	path := paths.ContainerData(usrdata, name)
	if _, err := os.Stat(filepath.Join(path, configJson)); err == nil {
		return nil, errors.Errorf("container %s already exists", name)
	}

	stagedPath := path + StageSuffix

	if err := os.RemoveAll(stagedPath); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to remove old staged container")
	}

	if err := os.MkdirAll(stagedPath, 0700); err != nil {
		return nil, errors.Wrap(err, "failed to create container directory")
	}

	if err := os.MkdirAll(filepath.Join(stagedPath, "storage"), 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create container storage directory")
	}

	file, err := os.Create(filepath.Join(stagedPath, configJson))
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
		Path: stagedPath,
		Config: &initialConfig,
	}, nil
}

func OpenPath(path, name string) (*Container, error) {
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

func Open(usrdata *userdata.Userdata, name string) (*Container, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	path := paths.ContainerData(usrdata, name)
	return OpenPath(path, name)
}

type LockWaitRequest int
const (
	WaitForLock LockWaitRequest = iota
	NoWaitForLock
)

func (container Container) LockUntilProcessDeath(wait LockWaitRequest) error {
	fd, err := unix.Open(container.Path, unix.O_DIRECTORY, 0)
	if err != nil {
		return errors.Wrap(err, "failed to open container directory")
	}

	operation := unix.LOCK_EX
	if wait == NoWaitForLock {
		operation |= unix.LOCK_NB
	}

	if err := unix.Flock(fd, operation); err != nil {
		return errors.Wrap(err, "failed to lock container directory")
	}

	// Let the fd "leak"; Linux will close it anyway once we die, and it will let us
	// easily hold the lock until process death.
	return nil
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

func (container Container) Staged() bool {
	return strings.HasSuffix(container.Path, StageSuffix)
}

func (container Container) Rename(newname string) error {
	return os.Rename(container.Path, filepath.Join(filepath.Dir(container.Path), newname))
}

func (container Container) Unstage() error {
	if !container.Staged() {
		panic("cannot unstage unstaged container (?)")
	}

	return container.Rename(container.Name)
}

func (container Container) ExportsLink(temp bool) string {
	var name string
	if temp {
		name = "exports.tmp"
	} else {
		name = "exports"
	}

	return filepath.Join(container.Path, name)
}

func (container Container) ExportsInstance(n int) string {
	return filepath.Join(container.Path, fmt.Sprintf("exports.%d", n))
}
