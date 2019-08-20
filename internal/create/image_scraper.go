/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package create

import (
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"strings"
)

func getImagesIndexUrl(version string) *url.URL {
	var urlString = "https://dl.fedoraproject.org/pub/fedora/linux/releases/" + version +
					"/Container/x86_64/images/"

	url, err := url.Parse(urlString)
	if err != nil {
		panic(errors.Wrap(err, "unexpected url parse failed"))
	}

	return url
}

func readIndexHtml(index *url.URL) (*html.Node, error) {
	resp, err := http.Get(index.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to read container index")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// We basically have to scrape the index.html where you can download the container base image
// in order to find the actual URL to download. :/

func findLatestContainerImageUrlStringInNode(node *html.Node) string {
	if node.Type == html.ElementNode && node.Data == "a" {
		for _, attr := range node.Attr {
			const containerFilenamePrefix = "Fedora-Container-Base-"

			if attr.Namespace == "" && attr.Key == "href" && strings.HasPrefix(attr.Val, containerFilenamePrefix) {
				return attr.Val
			}
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if url := findLatestContainerImageUrlStringInNode(child); url != "" {
			return url
		}
	}

	return ""
}

func findLatestContainerImageUrlInDocument(doc *html.Node) (*url.URL, error) {
	urlString := findLatestContainerImageUrlStringInNode(doc)
	if urlString == "" {
		return nil, errors.New("failed to find latest container image url in index")
	}

	url, err := url.Parse(urlString)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse located image url")
	}

	return url, nil
}

func scrapeLatestContainerImageUrl(version string) (*url.URL, error) {
	indexUrl := getImagesIndexUrl(version)

	index, err := readIndexHtml(indexUrl)
	if err != nil {
		return nil, err
	}

	imageUrl, err := findLatestContainerImageUrlInDocument(index)
	if err != nil {
		return nil, err
	}

	return indexUrl.ResolveReference(imageUrl), nil
}
