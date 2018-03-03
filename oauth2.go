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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const OAuth2JWTAudience = "https://accounts.google.com/o/oauth2/token"

// Create an http.Client that authenticates requests with an OAuth token
// obtained from a signed JWT
func ClientForJWT(jwt string, client *http.Client) Client {
	if client == nil {
		client = http.DefaultClient
	}

	// build HTTP client to add OAuth credentials
	rt := &oauthTransport{
		rt: client.Transport,
	}

	rt.authenticate = func() error {
		params := url.Values{
			"grant_type": []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
			"assertion":  []string{jwt},
		}

		resp, err := client.PostForm("https://accounts.google.com/o/oauth2/token", params)
		if err != nil {
			return fmt.Errorf("Failed to contact OAuth2 service: %v", err)
		}

		var result struct {
			Error       string        `json:"error"`
			AccessToken string        `json:"access_token"`
			ExpiresIn   time.Duration `json:"expires_in"`
		}

		// parse incoming response
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("Failed to decode response: %v", err)
		} else if result.Error != "" {
			return fmt.Errorf("Failed: %s", result.Error)
		} else if result.AccessToken == "" {
			return errors.New("No access token received")
		}

		rt.expiration = time.Now().Add(result.ExpiresIn * time.Second)
		rt.token = result.AccessToken
		return nil
	}

	if rt.rt == nil {
		rt.rt = http.DefaultTransport
	}

	newClient := new(http.Client)
	*newClient = *client
	newClient.Transport = rt

	return Client{Client: newClient}
}

type Client struct {
	*http.Client
}

type oauthTransport struct {
	rt           http.RoundTripper
	authenticate func() error
	token        string
	expiration   time.Time
}

// Process a request, authenticating via OAuth2 if needed
func (t *oauthTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// clone the request header
	newHeader := make(http.Header)
	for k, v := range req.Header {
		newHeader[k] = v
	}

	// swap headers for this call
	oldHeader := req.Header
	req.Header = newHeader
	defer func() {
		req.Header = oldHeader
	}()

	now := time.Now()

	// allow 1 retry, in the case authentication fails
	for tries := 0; tries < 2; tries++ {
		// do we need a new token?
		if t.token == "" || now.After(t.expiration) {
			if err := t.authenticate(); err != nil {
				return nil, fmt.Errorf("Authorization failed: %v", err)
			}
		}

		// add the oauth token
		newHeader.Add("Authorization", "Bearer "+t.token)

		resp, err = t.rt.RoundTrip(req)

		// stop processing if we didn't receive an Unauthorized error
		if resp.StatusCode != http.StatusUnauthorized {
			break
		}
	}

	return
}
