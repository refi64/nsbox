/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	crypt "github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/coreos/go-systemd/v22/machine1"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
)

type Auth int

const (
	AuthAuto Auth = iota
	AuthManual
)

var (
	authToString = map[Auth]string{
		AuthAuto:   "auto",
		AuthManual: "manual",
	}

	stringToAuth = map[string]Auth{
		"auto":   AuthAuto,
		"manual": AuthManual,
	}
)

func (auth Auth) String() string {
	return authToString[auth]
}

func (auth *Auth) Set(value string) error {
	newAuth, ok := stringToAuth[strings.ToLower(value)]
	if !ok {
		return errors.New("invalid auth value")
	}

	*auth = newAuth
	return nil
}

func (auth Auth) MarshalJSON() ([]byte, error) {
	return []byte(`"` + auth.String() + `"`), nil
}

func (auth *Auth) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return auth.Set(value)
}

type Config struct {
	Image             string
	Boot              bool
	Auth              Auth
	XdgDesktopExports []string
	XdgDesktopExtra   []string
	ExtraCapabilities []string
	SyscallFilters    []string
	ExtraBindMounts   []string
	PrivateDirs       []string
	ShareCgroupfs     bool
	VirtualNetwork    bool
}

type Container struct {
	Name   string
	Path   string
	Config *Config
}

type Lock struct {
	fds []int
}

func (l *Lock) Release() {
	for _, fd := range l.fds {
		if err := unix.Close(fd); err != nil {
			log.Alert("WARNING: lock.Release -> Close:", err)
		}
	}

	l.fds = nil
}

const configJson = "config.json"
const StageSuffix = ".stage"

func validateName(name string) error {
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name); !matched {
		return errors.Errorf("invalid container name: %s", name)
	}

	return nil
}

func writeConfigToNewFile(config Config, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return err
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

	if err := os.MkdirAll(stagedPath, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create container directory")
	}

	if err := os.MkdirAll(filepath.Join(stagedPath, "storage"), 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create container storage directory")
	}

	stagedConfigPath := filepath.Join(stagedPath, configJson)
	if err := writeConfigToNewFile(initialConfig, stagedConfigPath); err != nil {
		return nil, errors.Wrap(err, "failed to write container config")
	}

	return &Container{
		Name:   name,
		Path:   stagedPath,
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

	if config.Image == "" {
		log.Alertf("WARNING: container has no image set; assuming legacy fedora:30")
		config.Image = "fedora:30"
	}

	return &Container{
		Name:   name,
		Path:   path,
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

func checkArrayItemsAgainstRegex(items []string, regexStr, errprefix string) error {
	regex := regexp.MustCompile(regexStr)
	for _, item := range items {
		if !regex.MatchString(item) {
			return errors.Errorf("%s: %s", errprefix, item)
		}
	}

	return nil
}

func (container Container) MachineName(usrdata *userdata.Userdata) string {
	return fmt.Sprintf("%s-%s", usrdata.EscapedUsername(), container.Name)
}

func (container Container) UpdateConfig() error {
	if err := checkArrayItemsAgainstRegex(container.Config.ExtraBindMounts,
		`^.+(:.+)?$`, "invalid bind mount"); err != nil {
		return err
	}

	if err := checkArrayItemsAgainstRegex(container.Config.SyscallFilters,
		`^@[a-z\-]+$|[a-z0-9_]+$`, "invalid syscall filter"); err != nil {
		return err
	}

	// No absolute paths or ones with '..' inside.
	if err := checkArrayItemsAgainstRegex(container.Config.PrivateDirs,
		`^[^/]`, "private dirs must be relative"); err != nil {
		return err
	}
	if err := checkArrayItemsAgainstRegex(container.Config.PrivateDirs,
		`^([^.]{2}|[^.]|\.([^.]|$))+$`, "private dirs must not contain .."); err != nil {
		return err
	}

	if container.Config.VirtualNetwork && !container.Config.Boot {
		return errors.New("cannot use private networking on a non-booted container")
	}

	configPath := filepath.Join(container.Path, configJson)
	tempConfigPath := configPath + ".tmp"

	if err := writeConfigToNewFile(*container.Config, tempConfigPath); err != nil {
		return errors.Wrap(err, "failed to write temporary config")
	}

	if err := os.Rename(tempConfigPath, configPath); err != nil {
		return errors.Wrap(err, "failed to overwrite config")
	}

	return nil
}

func (container Container) Shell(usrdata *userdata.Userdata) string {
	fullShellPath := container.StorageChild(usrdata.Shell)

	if _, err := os.Stat(fullShellPath); err != nil {
		log.Debugf("Failed to stat shell %s: %v", fullShellPath, err)
		return "/bin/bash"
	} else {
		return usrdata.Shell
	}
}

func (container Container) Leader(usrdata *userdata.Userdata) (uint32, error) {
	machined, err := machine1.New()
	if err != nil {
		return 0, err
	}

	props, err := machined.DescribeMachine(container.MachineName(usrdata))
	if err != nil {
		return 0, err
	}

	return props["Leader"].(uint32), nil
}

type LockWaitRequest int

const (
	WaitForLock LockWaitRequest = iota
	NoWaitForLock
)

type LockLevel int

// Order is significant! In order to take a full container lock, all levels below it must be locked.
const (
	RunLock LockLevel = iota
	ExportsLock
	ConfigLock
	FullContainerLock
)

func (container Container) Lock(level LockLevel, wait LockWaitRequest) (*Lock, error) {
	var name string
	mode := unix.O_RDONLY

	switch level {
	case RunLock:
		name = "run"
		break
	case ExportsLock:
		name = "exports"
		break
	case ConfigLock:
		name = "config"
	case FullContainerLock:
		// Leave the name empty to lock the entire container.
		mode = unix.O_DIRECTORY
		break
	}

	path := container.Path
	if level != FullContainerLock {
		path = filepath.Join(path, name+".lock")

		file, err := os.Create(path)
		if err != nil {
			return nil, errors.Wrapf(err, "create %s lock file", name)
		}
		file.Close()
	}

	fd, err := unix.Open(path, mode, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open container directory")
	}

	operation := unix.LOCK_EX
	if wait == NoWaitForLock {
		operation |= unix.LOCK_NB
	}

	if err := unix.Flock(fd, operation); err != nil {
		if errno, ok := err.(unix.Errno); ok && errno == unix.EWOULDBLOCK {
			return nil, errors.New("container is locked")
		}

		return nil, errors.Wrap(err, "failed to lock container directory")
	}

	var lock Lock
	lock.fds = []int{fd}

	// If this is a full lock, then it means it implicitly gains all lower levels as well.
	if level == FullContainerLock {
		for i := 0; i < int(FullContainerLock); i++ {
			sublock, err := container.Lock(LockLevel(i), wait)
			if err != nil {
				lock.Release()
				return nil, errors.Wrapf(err, "locking lower level lock %d", i)
			}

			lock.fds = append(lock.fds, sublock.fds...)
			sublock.fds = nil
		}
	}

	return &lock, nil
}

func (container Container) LockUntilProcessDeath(level LockLevel, wait LockWaitRequest) error {
	// Let the lock "leak"; Linux will close it anyway once we die, and it will let us
	// easily hold the lock until process death.
	_, err := container.Lock(level, wait)
	return err
}

func (container Container) LockAndDelete(wait LockWaitRequest) error {
	lock, err := container.Lock(FullContainerLock, wait)
	if err != nil {
		return err
	}
	defer lock.Release()

	if err := os.RemoveAll(container.Path); err != nil {
		return errors.Wrap(err, "failure during container deletion")
	}

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

func (container Container) PrivateHomeStorage(usrdata *userdata.Userdata) string {
	return filepath.Join(usrdata.User.HomeDir, ".var", "nsbox", container.Name)
}

func (container Container) PrivateHomeStorageChild(usrdata *userdata.Userdata,
	children ...string) string {
	parts := append([]string{container.PrivateHomeStorage(usrdata)}, children...)
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

func (container Container) UpdateManualPassword(pass []byte) error {
	if container.Config.Auth != AuthManual {
		panic("container auth must be AuthManual to set a password")
	}

	cr := crypt.New()
	hashed, err := cr.Generate(pass, []byte{})
	if err != nil {
		panic(errors.Wrap(err, "crypt"))
	}

	passPath := container.StorageChild(paths.InContainerPrivPath, "shadow-custom-pass")
	passFile, err := os.Create(passPath)
	if err != nil {
		return errors.Wrap(err, "failed to create shadow password file")
	}

	defer passFile.Close()

	if err := passFile.Chmod(0); err != nil {
		return errors.Wrap(err, "failed to modify shadow password file permissions")
	}

	fmt.Fprint(passFile, hashed)
	return nil
}
