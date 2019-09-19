/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	crename "github.com/google/go-containerregistry/pkg/name"
	crev1 "github.com/google/go-containerregistry/pkg/v1"
	creremote "github.com/google/go-containerregistry/pkg/v1/remote"
	cretarball "github.com/google/go-containerregistry/pkg/v1/tarball"
	cretypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/image"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"github.com/refi64/nsbox/internal/webutil"
	"os"
	"path/filepath"
)

func fetchLayer(layer crev1.Layer, dest string) error {
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

func saveImageToContainer(img *image.Image, ct *container.Container, tarOverride string) error {
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

	log.Info("Looking up image...")

	var dockerImage crev1.Image

	if tarOverride == "" {
		ref, err := crename.ParseReference(img.RemoteTarget)
		if err != nil {
			return errors.Wrap(err, "failed to parse RemoteTarget reference")
		}

		dockerImage, err = creremote.Image(ref)
		if err != nil {
			return errors.Wrap(err, "failed to load image from remote")
		}
	} else {
		tag, err := crename.NewTag(img.RemoteTarget)
		if err != nil {
			return errors.Wrap(err, "failed to parse RemoteTarget as tag")
		}

		dockerImage, err = cretarball.ImageFromPath(tarOverride, &tag)
		if err != nil {
			return errors.Wrap(err, "failed to load image from tar")
		}
	}

	manifest, err := dockerImage.Manifest()
	if err != nil {
		return errors.Wrap(err, "failed to fetch image manifest")
	}

	log.Infof("Fetching %d layer(s)...", len(manifest.Layers))

	layerFiles := []string{}

	for _, descr := range manifest.Layers {
		layer, err := dockerImage.LayerByDigest(descr.Digest)
		if err != nil {
			return errors.Wrapf(err, "failed to fetch image layer %s", descr.Digest.String())
		}

		layerDest := filepath.Join(tmp.Path, descr.Digest.String()+".tar.gz")
		log.Debug("Fetch layer", layerDest)

		if err := fetchLayer(layer, layerDest); err != nil {
			return err
		}

		layerFiles = append(layerFiles, layerDest)
	}

	log.Info("Extracting layer(s)...")

	for _, file := range layerFiles {
		log.Debug("Extract", file)
		if err := webutil.ExtractFullTarArchive(file, ct.Storage()); err != nil {
			return errors.Wrapf(err, "failed to extract %s", file)
		}
	}

	return nil
}

func CreateContainer(usrdata *userdata.Userdata, name, tar string, config container.Config) error {
	img, err := image.Open(config.Image)
	if err != nil {
		return errors.Wrap(err, "failed to open image")
	}

	ct, err := container.CreateStaged(usrdata, name, config)
	if err != nil {
		return err
	}

	if err := saveImageToContainer(img, ct, tar); err != nil {
		return err
	}

	if err := ct.Unstage(); err != nil {
		return err
	}

	// Make this the new default container if there is none set.
	if def, err := inventory.DefaultContainer(usrdata); err == nil && def == nil {
		if err := inventory.SetDefaultContainer(usrdata, name); err != nil {
			return errors.Wrap(err, "failed to set new default container")
		}
	}

	log.Info("Done!")
	return nil
}
