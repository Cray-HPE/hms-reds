// MIT License
//
// (C) Copyright [2019, 2021] Hewlett Packard Enterprise Development LP
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cray-HPE/hms-reds/internal/columbia"
	"github.com/Cray-HPE/hms-reds/internal/mapping"
	storage_factory "github.com/Cray-HPE/hms-reds/internal/storage/factory"
	sstorage "github.com/Cray-HPE/hms-securestorage"
)

type badReq struct {
	url    string
	method string
}

func GetHTTPResponse(
	t *testing.T,
	h func(http.ResponseWriter, *http.Request),
	method string, path string, body io.Reader,
	auth bool, username string, password string) *httptest.ResponseRecorder {

	handler := http.HandlerFunc(h)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to
	// record the response.
	rr := httptest.NewRecorder()

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatal(err)
	}

	if auth {
		req.SetBasicAuth(username, password)
	}

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	return rr
}

func TestVersions(t *testing.T) {
	rr := GetHTTPResponse(t, Versions, "GET", "/", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "/v1\n"
	if rr.Body.String() != expected {
		t.Errorf("Unexpected response body: got %v, want %v",
			rr.Body.String(), expected)
	}
}

func TestDoReadinessCheck(t *testing.T) {

	rr := GetHTTPResponse(t, doReadinessCheck, "GET", "/v1/readiness", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusServiceUnavailable)
	}

	// Service Ready
	mainstorage, err := storage_factory.MakeStorage("etcd", "mem:", false)
	if err != nil {
		t.Errorf("Unable to connect to storage: %s", err)
	}
	ss, _ := sstorage.NewMockAdapter()

	// Set up storage to load the mapping
	mapping.SetStorage(mainstorage, ss)
	rr = GetHTTPResponse(t, doReadinessCheck, "GET", "/v1/readiness", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}

	// Mock up the columbia list having been read
	columbia.ColumbiaListReadMockup(true)
	rr = GetHTTPResponse(t, doReadinessCheck, "GET", "/v1/readiness", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}
}

func TestDoLivenessCheck(t *testing.T) {
	rr := GetHTTPResponse(t, doLivenessCheck, "GET", "/v1/Liveness", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}
}
