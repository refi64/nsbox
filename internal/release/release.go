/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package release

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/paths"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Branch int

const (
	StableBranch Branch = iota
	EdgeBranch
)

func (branch Branch) String() string {
	switch branch {
	case StableBranch:
		return "stable"
	case EdgeBranch:
		return "edge"
	default:
		return "invalid"
	}
}

type ReleaseInfo struct {
	Branch	Branch
	Version string
}

func Read() (*ReleaseInfo, error) {
	releaseDir, err := paths.GetPathRelativeToInstallRoot(paths.Share, paths.ProductName, "release")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get release path")
	}

	var release ReleaseInfo
	var branchString string

	releaseFiles := map[string]*string{
		"BRANCH":  &branchString,
		"VERSION": &release.Version,
	}

	for releaseFile, target := range releaseFiles {
		file, err := os.Open(filepath.Join(releaseDir, releaseFile))
		if err != nil {
			return nil, err
		}

		defer file.Close()

		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}

		*target = strings.TrimSpace(string(bytes))
	}

	switch branchString {
	case "stable":
		release.Branch = StableBranch
	case "edge":
		release.Branch = EdgeBranch
	default:
		return nil, errors.Errorf("invalid release file branch: %s", branchString)
	}

	return &release, nil
}
