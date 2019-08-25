/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"bufio"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"github.com/refi64/nsbox/internal/webutil"
	"os"
	"path/filepath"
	"strings"
)

func getCurrentFedoraVersion() (string, error) {
	const versionIdPrefix = "VERSION_ID="

	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", errors.Wrap(err, "failed to open /etc/os-release")
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, versionIdPrefix) {
			return line[len(versionIdPrefix):], nil
		}
	}

	return "", errors.New("failed to locate VERSION_ID= field in /etc/os-release")
}

func extractLayers(archive, tmpdir string, ct *container.Container) error {
	// We need to extract layer.tar, then extract its contents.

	tmpContainerPath := ct.TempStorage()

	if _, err := os.Stat(tmpContainerPath); err == nil {
		log.Info("Deleting old container data...")
		if err := os.RemoveAll(tmpContainerPath); err != nil {
			log.Fatal(err)
		}
	}

	const layerTar = "layer.tar"
	layerDest := filepath.Join(tmpdir, layerTar)

	if err := webutil.ExtractItemFromArchiveWithProgress(archive, layerTar, layerDest); err != nil {
		return err
	}

	log.Info("Extracting container contents (please wait, this may take a while)...")
	return webutil.ExtractFullArchive(layerDest, tmpContainerPath)
}

func unmaskServices(ct *container.Container) error {
	log.Info("Unmasking required services...")

	maskedServices := []string{"console-getty.service", "systemd-logind.service"}

	for _, service := range maskedServices {
		path := ct.TempStorageChild("etc", "systemd", "system", service)
		log.Debug("unmask", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to unmask %s", service)
		}
	}

	return nil
}

func CreateContainer(usrdata *userdata.Userdata, name, version string, config container.Config) error {
	ct, err := container.Create(usrdata, name, config)
	if err != nil {
		return err
	}

	if version == "" {
		var err error
		version, err = getCurrentFedoraVersion()
		if err != nil {
			return err
		}
	}

	imageUrl, err := scrapeLatestContainerImageUrl(version)
	if err != nil {
		return err
	}

	tmp, err := lxtempdir.Create("", "nsbox-")
	if err != nil {
		return err
	}

	defer func() {
		if err := os.RemoveAll(tmp.Path); err != nil {
			log.Info("failed to remove temporary directory: ", err)
		}

		if err := tmp.Close(); err != nil {
			log.Info("failed to close temporary directory: ", err)
		}
	}()

	imageDest := filepath.Join(tmp.Path, filepath.Base(imageUrl.Path))

	if err := webutil.DownloadFileWithProgress(imageUrl, imageDest); err != nil {
		return err
	}

	if err := extractLayers(imageDest, tmp.Path, ct); err != nil {
		return err
	}

	if ct.Config.Boot {
		if err := unmaskServices(ct); err != nil {
			return err
		}
	}

	if err := os.Rename(ct.TempStorage(), ct.Storage()); err != nil {
		return errors.Wrap(err, "failed to rename temporary storage")
	}

	log.Info("Done!")

	return nil
}
