/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"github.com/artyom/untar"
	"github.com/briandowns/spinner"
	"github.com/dustin/go-humanize"
	crename "github.com/google/go-containerregistry/pkg/name"
	crev1 "github.com/google/go-containerregistry/pkg/v1"
	cremutate "github.com/google/go-containerregistry/pkg/v1/mutate"
	creremote "github.com/google/go-containerregistry/pkg/v1/remote"
	cretarball "github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/refi64/go-lxtempdir"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/image"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"io"
	"os"
	"time"
)

type spinnerProgress struct {
	spinner *spinner.Spinner
	total   uint64
}

func (p *spinnerProgress) Write(data []byte) (int, error) {
	bytes := len(data)
	p.total += uint64(bytes)
	p.spinner.Suffix = " " + humanize.Bytes(p.total)
	return bytes, nil
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
		ref, err := crename.ParseReference(img.Remote)
		if err != nil {
			return errors.Wrap(err, "failed to parse Remote reference")
		}

		dockerImage, err = creremote.Image(ref)
		if err != nil {
			return errors.Wrap(err, "failed to load image from remote")
		}
	} else {
		tag, err := crename.NewTag(img.Target)
		if err != nil {
			return errors.Wrap(err, "failed to parse Remote as tag")
		}

		dockerImage, err = cretarball.ImageFromPath(tarOverride, &tag)
		if err != nil {
			return errors.Wrap(err, "failed to load image from tar")
		}
	}

	rd := cremutate.Extract(dockerImage)
	defer rd.Close()

	spinner := spinner.New(spinner.CharSets[43], 200 * time.Millisecond)
	spinner.Prefix = "Fetching image: "
	spinner.Start()
	defer spinner.Stop()

	tee := io.TeeReader(rd, &spinnerProgress{spinner: spinner})
	if err := untar.Untar(tee, ct.Storage()); err != nil {
		return errors.Wrap(err, "failed to untar image")
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

	if _, err := usrdata.ShadowLine(); err != nil {
		log.Debug("ShadowLine error:", err)
		log.Alert("WARNING: nsbox could not retrieve your user account information from the")
		log.Alert("					shadow database. If you are using SSSD or another remote auth system")
		log.Alert("					then please see: https://nsbox.dev/guide.html#custom-authentication")
	}

	log.Info("Done!")
	return nil
}
