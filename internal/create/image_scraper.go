/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"github.com/refi64/nsbox/internal/webutil"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"net/url"
)

func getImagesIndexUrl(version string) *url.URL {
	var urlString = "https://dl.fedoraproject.org/pub/fedora/linux/releases/" + version +
					"/Container/x86_64/images/"

	indexUrl, err := url.Parse(urlString)
	if err != nil {
		panic(errors.Wrap(err, "unexpected url parse failed"))
	}

	return indexUrl
}

// We basically have to scrape the index.html where you can download the container base image
// in order to find the actual URL to download. :/

func findLatestContainerImageUrlInDocument(doc *html.Node) (*url.URL, error) {
	scanner, err := webutil.NewHrefMatchScanner(`^Fedora-Container-Base-`)
	if err != nil {
		panic(err)
	}

	imageUrl, err := webutil.ScanDocument(doc, scanner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse located image url")
	} else if imageUrl == nil {
		return nil, errors.New("failed to find latest container image url in index")
	}

	return imageUrl.(*url.URL), nil
}

func scrapeLatestContainerImageUrl(version string) (*url.URL, error) {
	indexUrl := getImagesIndexUrl(version)

	index, err := webutil.FetchHtmlPage(indexUrl)
	if err != nil {
		return nil, err
	}

	imageUrl, err := findLatestContainerImageUrlInDocument(index)
	if err != nil {
		return nil, err
	}

	return indexUrl.ResolveReference(imageUrl), nil
}
