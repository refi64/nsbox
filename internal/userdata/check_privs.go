/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
)

type cvtsudoersUser struct {
	Username  string
	Userid    uint
	Usergroup string
	Usergid   uint
}

type cvtsudoersCommand struct {
	Command string
}

type cvtsudoersOption map[string]interface{}

type cvtsudoersCommandSpec struct {
	Commands []cvtsudoersCommand `json:"Commands"`
	Options  []cvtsudoersOption  `json:"Options"`
}

type cvtsudoersUserSpec struct {
	UserList     []cvtsudoersUser        `json:"User_List"`
	CommandSpecs []cvtsudoersCommandSpec `json:"Cmnd_Specs"`
}

type cvtsudoersJson struct {
	UserSpecs []cvtsudoersUserSpec `json:"User_Specs"`
}

type SudoAccess int

const (
	NoSudo SudoAccess = iota
	CanSudo
	CanSudoNoPasswd
)

func (usrdata *Userdata) checkSudoAccess() (SudoAccess, error) {
	log.Debug("Checking sudo access")

	if _, err := exec.LookPath("cvtsudoers"); err != nil {
		log.Debugf("cvtsudoers not found (%v), skipping sudo access grant", err)
		return NoSudo, nil
	}

	// NOTE: we don't bother with paths.Config here because:
	// - It would lead to an import cycle.
	// - As far as I can tell, sudo does *not* let you change the path to the sudoers file.
	cmd := exec.Command("cvtsudoers", "-f", "json", "/etc/sudoers")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return NoSudo, errors.Wrap(err, "cvtsudoers")
	}

	var cvtsudoers cvtsudoersJson
	if err := json.Unmarshal(out, &cvtsudoers); err != nil {
		return NoSudo, errors.Wrap(err, "failed to parse cvtsudoers output")
	}

	sudoAccess := NoSudo

	for _, userSpec := range cvtsudoers.UserSpecs {
		canSudo := false
		canRunAnyCommand := false
		authenticated := true

		for _, commandSpec := range userSpec.CommandSpecs {
			for _, command := range commandSpec.Commands {
				if command.Command == "ALL" {
					canRunAnyCommand = true
					break
				}
			}

			for _, option := range commandSpec.Options {
				for k, v := range option {
					if k == "authenticate" {
						authenticated = v.(bool)
						break
					}
				}
			}
		}

		if !canRunAnyCommand {
			continue
		}

	userSpecSearch:
		for _, user := range userSpec.UserList {
			if user.Username == usrdata.User.Username || fmt.Sprint(user.Userid) == usrdata.User.Uid {
				canSudo = true
				break
			}

			for _, group := range usrdata.Groups {
				if user.Usergroup == group.Name || fmt.Sprint(user.Usergid) == group.Gid {
					canSudo = true
					break userSpecSearch
				}
			}
		}

		if canSudo {
			if authenticated {
				sudoAccess = CanSudo
			} else {
				sudoAccess = CanSudoNoPasswd
			}
		}
	}

	return sudoAccess, nil
}

func (usrdata *Userdata) GetSudoAccess() SudoAccess {
	access, err := usrdata.checkSudoAccess()
	if err != nil {
		log.Debug("failed to check sudo access:", err)
		return NoSudo
	}

	return access
}
