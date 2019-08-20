/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"bufio"
	"gopkg.in/cheggaaa/pb.v1"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"io"
	"net/http"
	"net/url"
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

func downloadFile(url *url.URL, dest string) error {
	log.Infof("Downloading %s...\n", url, dest)

	file, err := os.Create(dest)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}

	defer file.Close()

	resp, err := http.Get(url.String())
	if err != nil {
		return errors.Wrap(err, "failed to open download connection")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("unexpected response code %d", resp.StatusCode)
	}

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	defer bar.Finish()

	reader := bar.NewProxyReader(resp.Body)

	if _, err := io.Copy(file, reader); err != nil {
		return errors.Wrap(err, "failed to download file")
	}

	return nil
}

func extractLayers(archive string, tmpdir string, ct *container.Container) error {
	// We need to extract layer.tar, then extract its contents.

	containerPath := ct.Storage()
	tmpContainerPath := ct.TempStorage()

	if _, err := os.Stat(tmpContainerPath); err == nil {
		log.Info("Deleting old container data...")
		if err := os.RemoveAll(tmpContainerPath); err != nil {
			log.Fatal(err)
		}
	}

	log.Info("Extracting layer.tar...")

	const layerTar = "layer.tar"
	layerDest := filepath.Join(tmpdir, layerTar)

	err := archiver.Walk(archive, func(file archiver.File) error {
		if file.Name() == layerTar {
			dest, err := os.Create(layerDest)
			if err != nil {
				return errors.Wrap(err, "failed to create layer.tar")
			}

			bar := pb.New(int(file.Size())).SetUnits(pb.U_BYTES)
			bar.Start()
			defer bar.Finish()

			reader := bar.NewProxyReader(file)

			if _, err := io.Copy(dest, reader); err != nil {
				return errors.Wrap(err, "failed to extract layer.tar from image")
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Extracting container contents (please wait, this may take a while)...")
	archiver.Unarchive(layerDest, tmpContainerPath)
	os.Rename(tmpContainerPath, containerPath)

	log.Info("Done!")

	return nil
}

func CreateContainer(name string, version string) error {
	ct, err := container.Create(name, container.Config{})
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

	if err := downloadFile(imageUrl, imageDest); err != nil {
		log.Fatal(err)
	}

	if err := extractLayers(imageDest, tmp.Path, ct); err != nil {
		log.Fatal(err)
	}

	return nil
}
