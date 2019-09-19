/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package userdata

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// Encapsulates data about the user's session that we're representing.
type Userdata struct {
	User     *user.User
	Shell    string
	GroupIds []string
	Environ  map[string]string
}

func getUserShell(usr *user.User) (string, error) {
	// XXX: This sucks.
	cmd := exec.Command("getent", "passwd", usr.Uid)
	outBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to ask getent for shell")
	}

	out := string(outBytes)
	idx := strings.LastIndex(out, ":")
	if idx == -1 || idx == len(out)-1 {
		return "", errors.New("failed to split getent output")
	}

	shell := out[idx+1:]
	return shell, nil
}

// Like os.Environ, but only returns ssome whitelisted environment variables.
func WhitelistedEnviron() []string {
	whitelist := []string{
		"COLORTERM",
		"DBUS_SESSION_BUS_ADDRESS",
		"DBUS_SYSTEM_BUS_ADDRESS",
		"DESKTOP_SESSION",
		"DISPLAY",
		"LANG",
		"SHELL",
		"SSH_AUTH_SOCK",
		"TERM",
		"VTE_VERSION",
		"WAYLAND_DISPLAY",
		"XDG_CURRENT_DESKTOP",
		"XDG_DATA_DIRS",
		"XDG_MENU_PREFIX",
		"XDG_RUNTIME_DIR",
		"XDG_SEAT",
		"XDG_SESSION_DESKTOP",
		"XDG_SESSION_ID",
		"XDG_SESSION_TYPE",
		"XDG_VTNR",
	}

	result := make([]string, 0)

	for _, env := range os.Environ() {
		for _, whitelisted := range whitelist {
			if strings.HasPrefix(env, whitelisted+"=") {
				result = append(result, env)
			}
		}
	}

	return result
}

// Parses os.Environ-formatted environment variables into a map.
// (The fact that Go named os.Environ like Python's os.environ but it's not a map is a
// travesty.)
func parseEnviron() map[string]string {
	result := make(map[string]string)

	for _, env := range WhitelistedEnviron() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) < 2 {
			panic(fmt.Errorf("unexpected environ value: %s", env))
		}

		name := parts[0]
		value := parts[1]
		result[name] = value
	}

	return result
}

func userdataForUser(usr *user.User) (*Userdata, error) {
	shell, err := getUserShell(usr)
	if err != nil {
		return nil, err
	}

	groupIds, err := usr.GroupIds()
	if err != nil {
		return nil, err
	}

	return &Userdata{
		User:     usr,
		Shell:    shell,
		GroupIds: groupIds,
		Environ:  parseEnviron(),
	}, nil
}

func Current() (*Userdata, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	return userdataForUser(usr)
}

func BeneathSudo() (*Userdata, error) {
	if os.Getuid() != 0 {
		panic(errors.New("BeneathSudo run as non-root"))
	}

	var callingUid string
	envVars := []string{"PKEXEC_UID", "SUDO_UID"}

	for _, envVar := range envVars {
		if callingUid = os.Getenv(envVar); callingUid != "" {
			break
		}
	}

	if callingUid == "" {
		callingUid = "0"
	}

	usr, err := user.LookupId(callingUid)
	if err != nil {
		return nil, err
	}

	return userdataForUser(usr)
}
