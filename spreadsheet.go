/*-
 * Copyright 2014 Matthew Endsley
 * All rights reserved
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted providing that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
 * IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

package googleapi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const SpreadsheetScope = "https://spreadsheets.google.com/feeds https://docs.google.com/feeds"

// Get a mapping of spreadsheet titles -> ids
func (cl Client) GetSpreadsheets() (map[string][]string, error) {
	const base = "https://spreadsheets.google.com/feeds/spreadsheets/private/full/"

	resp, err := cl.Get(base)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.New(resp.Status)
	}

	// parse the feed XML
	var feed struct {
		Entry []struct {
			Id    string `xml:"id"`
			Title string `xml:"title"`
		} `xml:"entry"`
	}
	err = xml.NewDecoder(resp.Body).Decode(&feed)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("Failed to parse response XML: %v", err)
	}

	// convert the results to a title->id mapping
	results := make(map[string][]string)
	for _, entry := range feed.Entry {
		id := strings.TrimPrefix(entry.Id, base)
		results[entry.Title] = append(results[entry.Title], id)
	}

	return results, nil
}

// Get a spreadsheet as a CSV
func (cl Client) GetSpreadsheetAsCSV(id string) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://spreadsheets.google.com/feeds/download/spreadsheets/Export?key=%s&hl&exportFormat=csv", id)
	resp, err := cl.Get(url)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.New(resp.Status)
	}

	return resp.Body, nil
}
