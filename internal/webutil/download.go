/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package webutil

import (
	"github.com/refi64/nsbox/internal/log"
	"github.com/pkg/errors"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"net/http"
	"net/url"
	"os"
)

func SaveReaderWithProgress(size int64, reader io.Reader, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}

	defer file.Close()

	bar := pb.New64(size).SetUnits(pb.U_BYTES)
	bar.Start()
	defer bar.Finish()

	proxy := bar.NewProxyReader(reader)

	if _, err := io.Copy(file, proxy); err != nil {
		return errors.Wrap(err, "failed to save file")
	}

	return nil
}

func DownloadFileWithProgress(url *url.URL, dest string) error {
	log.Infof("Downloading %s...", url)

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

	return SaveReaderWithProgress(resp.ContentLength, resp.Body, dest)
}
