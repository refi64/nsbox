/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"bufio"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	crename "github.com/google/go-containerregistry/pkg/name"
	crev1 "github.com/google/go-containerregistry/pkg/v1"
	creremote "github.com/google/go-containerregistry/pkg/v1/remote"
	cretypes "github.com/google/go-containerregistry/pkg/v1/types"
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

func unmaskServices(ct *container.Container) error {
	log.Info("Unmasking required services...")

	maskedServices := []string{"console-getty.service", "systemd-logind.service"}

	for _, service := range maskedServices {
		path := ct.StorageChild("etc", "systemd", "system", service)
		log.Debug("unmask", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to unmask %s", service)
		}
	}

	return nil
}

func downloadLayer(layer crev1.Layer, dest string) error {
	mediaType, err := layer.MediaType()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve layer media type")
	}

	if mediaType != cretypes.OCILayer && mediaType != cretypes.DockerLayer {
		return errors.Errorf("unexpected layer type %s", mediaType)
	}

	size, err := layer.Size()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve layer size")
	}

	reader, err := layer.Compressed()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve compressed layer")
	}

	defer reader.Close()

	return webutil.SaveReaderWithProgress(size, reader, dest)
}

func retrieveImage(refstr string, ct *container.Container) error {
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

	ref, err := crename.ParseReference(refstr)
	if err != nil {
		return errors.Wrap(err, "failed to parse image source reference")
	}

	image, err := creremote.Image(ref)
	if err != nil {
		return errors.Wrap(err, "failed to fetch image metadata")
	}

	manifest, err := image.Manifest()
	if err != nil {
		return errors.Wrap(err, "failed to fetch image manifest")
	}

	log.Infof("Downloading %d layer(s)...", len(manifest.Layers))

	layerFiles := []string{}

	for _, descr := range manifest.Layers {
		layer, err := image.LayerByDigest(descr.Digest)
		if err != nil {
			return errors.Wrapf(err, "failed to fetch image layer %s", descr.Digest.String())
		}

		layerDest := filepath.Join(tmp.Path, descr.Digest.String() + ".tar.gz")
		if err := downloadLayer(layer, layerDest); err != nil {
			return err
		}

		layerFiles = append(layerFiles, layerDest)
	}

	log.Info("Extracting layer(s)...")

	for _, file := range layerFiles {
		webutil.ExtractFullArchive(file, ct.Storage())
	}

	return nil
}

func CreateContainer(usrdata *userdata.Userdata, name, version string, config container.Config) error {
	ct, err := container.CreateStaged(usrdata, name, config)
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

	if err := retrieveImage("registry.fedoraproject.org/fedora:" + version, ct); err != nil {
		return err
	}

	if ct.Config.Boot {
		if err := unmaskServices(ct); err != nil {
			return err
		}
	}

	if err := ct.Unstage(); err != nil {
		return err
	}

	log.Info("Done!")
	return nil
}
