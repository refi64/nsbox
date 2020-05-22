/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package userdata

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// Stored as a map to make membership tests fast.
var whitelistedEnvNames = map[string]interface{}{
	"COLORTERM":                nil,
	"DBUS_SESSION_BUS_ADDRESS": nil,
	"DBUS_SYSTEM_BUS_ADDRESS":  nil,
	"DESKTOP_SESSION":          nil,
	"DISPLAY":                  nil,
	"GDK_SCALE":                nil,
	"GDK_DPI_SCALE":            nil,
	"LANG":                     nil,
	"SHELL":                    nil,
	"SSH_AUTH_SOCK":            nil,
	"TERM":                     nil,
	"VTE_VERSION":              nil,
	"WAYLAND_DISPLAY":          nil,
	"XDG_CURRENT_DESKTOP":      nil,
	"XDG_DATA_DIRS":            nil,
	"XDG_MENU_PREFIX":          nil,
	"XDG_RUNTIME_DIR":          nil,
	"XDG_SEAT":                 nil,
	"XDG_SESSION_DESKTOP":      nil,
	"XDG_SESSION_ID":           nil,
	"XDG_SESSION_TYPE":         nil,
	"XDG_VTNR":                 nil,
}

// Encapsulates data about the user's session that we're representing.
type Userdata struct {
	User    *user.User
	Shell   string
	Groups  []*user.Group
	Environ map[string]string
}

// XXX: This sucks.
func getent(db string, usr *user.User) (string, error) {
	cmd := exec.Command("getent", db, usr.Username)
	outBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to call getent for %s database", db)
	}

	return string(outBytes), nil
}

func getUserShell(usr *user.User) (string, error) {
	out, err := getent("passwd", usr)
	if err != nil {
		return "", errors.Wrap(err, "failed to ask getent for shell")
	}

	idx := strings.LastIndex(out, ":")
	if idx == -1 || idx == len(out)-1 {
		return "", errors.New("failed to split getent output")
	}

	shell := strings.TrimSpace(out[idx+1:])
	return shell, nil
}

// Checks if the given environment variable is on the execution whitelist.
func IsWhitelisted(name string) bool {
	_, ok := whitelistedEnvNames[name]
	return ok
}

// Little helper to split environment variables.
func SplitEnv(env string) (string, string) {
	parts := strings.SplitN(env, "=", 2)
	return parts[0], parts[1]
}

// Like os.Environ, but only returns some whitelisted environment variables.
func WhitelistedEnviron() []string {
	result := make([]string, 0)

	for _, env := range os.Environ() {
		name, _ := SplitEnv(env)
		if IsWhitelisted(name) {
			result = append(result, env)
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

	var groups []*user.Group
	for _, gid := range groupIds {
		group, err := user.LookupGroupId(gid)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return &Userdata{
		User:    usr,
		Shell:   shell,
		Groups:  groups,
		Environ: parseEnviron(),
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

func (usrdata *Userdata) ShadowLine() (string, error) {
	out, err := getent("shadow", usrdata.User)
	if err != nil {
		return "", errors.Wrap(err, "getent")
	}

	out = strings.TrimSpace(out)
	if len(out) == 0 {
		return "", errors.New("shadow entry missing")
	}

	return out, nil
}

func (usrdata Userdata) EscapedUsername() string {
	escapeRe := regexp.MustCompile(`[^A-Za-z0-9]+`)
	return string(escapeRe.ReplaceAllStringFunc(usrdata.User.Username, func(match string) string {
		var builder strings.Builder
		for _, r := range match {
			fmt.Fprintf(&builder, "\\x%x", r)
		}
		return builder.String()
	}))
}
