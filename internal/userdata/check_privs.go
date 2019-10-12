/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"os"
	"os/exec"
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

type cvtsudoersCommandSpec struct {
	Commands []cvtsudoersCommand `json:"Commands"`
}

type cvtsudoersUserSpec struct {
	UserList     []cvtsudoersUser `json:"User_List"`
	CommandSpecs []cvtsudoersCommandSpec `json:"Cmnd_Specs"`
}

type cvtsudoersJson struct {
	UserSpecs []cvtsudoersUserSpec `json:"User_Specs"`
}

func (usrdata *Userdata) checkSudoAccess() (bool, error) {
	log.Debug("Checking sudo access")

	if _, err := exec.LookPath("cvtsudoers"); err != nil {
		log.Debugf("cvtsudoers not found (%v), skipping sudo access grant", err)
		return false, nil
	}

	// NOTE: we don't bother with paths.Config here because:
	// - It would lead to an import cycle.
	// - As far as I can tell, sudo does *not* let you change the path to the sudoers file.
	cmd := exec.Command("cvtsudoers", "-f", "json", "/etc/sudoers")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, "cvtsudoers")
	}

	var cvtsudoers cvtsudoersJson
	if err := json.Unmarshal(out, &cvtsudoers); err != nil {
		return false, errors.Wrap(err, "failed to parse cvtsudoers output")
	}

	userCanSudo := false

	userSpecSearch: for _, userSpec := range cvtsudoers.UserSpecs {
		canRunAnyCommand := false

		allCommandSearch: for _, commandSpec := range userSpec.CommandSpecs {
			for _, command := range commandSpec.Commands {
				if command.Command == "ALL" {
					canRunAnyCommand = true
					break allCommandSearch
				}
			}
		}

		if !canRunAnyCommand {
			continue
		}

		for _, user := range userSpec.UserList {
			if user.Username == usrdata.User.Username || string(user.Userid) == usrdata.User.Uid {
				userCanSudo = true
				break userSpecSearch
			}

			for _, group := range usrdata.Groups {
				if user.Usergroup == group.Name || string(user.Usergid) == group.Gid {
					userCanSudo = true
					break userSpecSearch
				}
			}
		}
	}

	return userCanSudo, nil
}

func (usrdata *Userdata) HasSudoAccess() bool {
	access, err := usrdata.checkSudoAccess()
	if err != nil {
		log.Debug("failed to check sudo access:", err)
		return false
	}

	return access
}
