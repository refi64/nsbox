/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package webutil

import (
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"os"
	"path/filepath"

	"os/exec"
)

func ExtractItemFromArchiveWithProgress(archive, item, dest string) error {
	log.Infof("Extracting %s from %s...", item, filepath.Base(archive))

	return archiver.Walk(archive, func(file archiver.File) error {
		if file.Name() == item {
			destFile, err := os.Create(dest)
			if err != nil {
				return errors.Wrapf(err, "failed to create %s", item)
			}

			bar := pb.New(int(file.Size())).SetUnits(pb.U_BYTES)
			bar.Start()
			defer bar.Finish()

			reader := bar.NewProxyReader(file)

			if _, err := io.Copy(destFile, reader); err != nil {
				return errors.Wrapf(err, "failed to extract %s", item)
			}
		}

		return nil
	})
}

func ExtractFullTarArchive(archive, destdir string) error {
	cmd := exec.Command("tar", "-C", destdir, "-xf", archive)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
