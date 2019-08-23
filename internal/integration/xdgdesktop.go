/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package integration

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func readExportsId(path string) (int, error) {
	target, err := os.Readlink(path)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, nil
		} else {
			return 0, err
		}
	}

	if strings.HasSuffix(target, ".0") {
		return 0, nil
	} else if strings.HasSuffix(target, ".1") {
		return 1, nil
	} else {
		return 0, errors.Errorf("%s does not have a valid export ID", path)
	}
}

func exportDesktopFile(ct *container.Container, targetDir, desktopFilePath string) error {
	source, err := os.Open(desktopFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open desktop file %s", desktopFilePath)
	}
	defer source.Close()

	target, err := os.Create(filepath.Join(targetDir, filepath.Base(desktopFilePath)))
	if err != nil {
		return errors.Wrapf(err, "failed to create exported desktop file of %s", desktopFilePath)
	}
	defer target.Close()

	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "#") && line != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				log.Alertf("%s had invalid line: %s", desktopFilePath, line)
			} else {
				if parts[0] == "Exec" {
					line = fmt.Sprintf("Exec=nsbox run -c %s -- %s", ct.Name, parts[1])
				}
			}

			// TODO: export icons
		}

		fmt.Fprintln(target, line)
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "failed to read %s", desktopFilePath)
	}

	return nil
}

func importDesktopFiles(ct *container.Container, targetRoot, sourcePrefix string) error {
	desktopFilesDir := ct.StorageChild(sourcePrefix, "share", "applications")
	target := filepath.Join(targetRoot, "share", "applications")

	if err := os.MkdirAll(target, 0755); err != nil {
		return errors.Wrapf(err, "failed to create target directory")
	}

	desktopFiles, err := ioutil.ReadDir(desktopFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug(desktopFilesDir, "does not exist")
			return nil
		} else {
			return err
		}
	}

	for _, file := range desktopFiles {
		if !strings.HasSuffix(file.Name(), ".desktop") {
			continue
		}

		path := filepath.Join(desktopFilesDir, file.Name())
		exportDesktopFile(ct, target, path)
	}

	return nil
}

func UpdateDesktopFiles(ct *container.Container) error {
	activeExportsLink := ct.ExportsLink(false)

	oldExportsInstanceId, err := readExportsId(activeExportsLink)
	if err != nil {
		return err
	}

	var newExportsInstanceId int
	if oldExportsInstanceId == 0 {
		newExportsInstanceId = 1
	} else {
		newExportsInstanceId = 0
	}

	newExportsInstanceDir := ct.ExportsInstance(newExportsInstanceId)

	if err := os.RemoveAll(newExportsInstanceDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to reset new exports instance dir %d", newExportsInstanceId)
	}

	if err := importDesktopFiles(ct, newExportsInstanceDir, "usr"); err != nil {
		return err
	}

	if err := importDesktopFiles(ct, newExportsInstanceDir, "usr/local"); err != nil {
		return err
	}

	tempExportsLink := ct.ExportsLink(true)
	if err := os.Symlink(newExportsInstanceDir, tempExportsLink); err != nil {
		return errors.Wrapf(err, "failed to symlink new instance dir")
	}

	if err := os.Rename(tempExportsLink, activeExportsLink); err != nil {
		return errors.Wrapf(err, "failed to rename onto new instance dir")
	}

	if oldExportsInstanceId != -1 {
		if err := os.RemoveAll(ct.ExportsInstance(oldExportsInstanceId)); err != nil {
			return errors.Wrapf(err, "failed to delete old instance dir")
		}
	}

	return nil
}
