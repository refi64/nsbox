/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package webutil

import (
	"github.com/refi64/nsbox/internal/log"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
)

func FetchHtmlPage(url *url.URL) (*html.Node, error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", url.String())
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("unexpected response code %d from %s", resp.StatusCode, url.String())
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func DownloadFileWithProgress(url *url.URL, dest string) error {
	log.Infof("Downloading %s...", url, dest)

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

type NodeScanner interface {
	ScanNode(node *html.Node) (interface{}, error)
}

func ScanDocument(node *html.Node, scanner NodeScanner) (result interface{}, err error) {
	result, err = scanner.ScanNode(node)
	if err != nil || result != nil {
		return
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		result, err = ScanDocument(child, scanner)
		if err != nil || result != nil {
			return
		}
	}

	return
}

type HrefMatchScanner struct {
	re *regexp.Regexp
}

func NewHrefMatchScanner(expr string) (*HrefMatchScanner, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}

	return &HrefMatchScanner{re: re}, nil
}

func (scanner *HrefMatchScanner) ScanNode(node *html.Node) (interface{}, error) {
	if node.Type == html.ElementNode && node.Data == "a" {
		for _, attr := range node.Attr {
			if attr.Namespace == "" && attr.Key == "href" && scanner.re.MatchString(attr.Val) {
				url, err := url.Parse(attr.Val)
				return url, err
			}
		}
	}

	return nil, nil
}
