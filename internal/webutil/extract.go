/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package webutil

import (
	"gopkg.in/cheggaaa/pb.v1"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"io"
	"os"
	"path/filepath"
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

func ExtractFullArchive(archive, destdir string) error {
	return archiver.Unarchive(archive, destdir)
}
