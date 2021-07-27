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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cray-HPE/hms-reds/internal/columbia"
	"github.com/Cray-HPE/hms-reds/internal/mapping"
	"github.com/Cray-HPE/hms-reds/internal/model"
	"github.com/Cray-HPE/hms-reds/internal/storage"
	storage_factory "github.com/Cray-HPE/hms-reds/internal/storage/factory"
	"github.com/Cray-HPE/hms-reds/internal/storage/mock"
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
	// Service Unavailable
	store, err := storage_factory.MakeStorage("etcd", "mem:", false)
	globalStorage = store

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
	if !mainstorage.CheckLiveness() {
		t.Errorf("ERROR: Unable to write port mapping file! Error was: %v", err)
	}
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

func TestDoPostCredentials(t *testing.T) {
	// Pre-configuration: set up package global storage variable
	ss := mock.NewKvMock()
	credStorage = model.NewRedsCredStore(model.CredentialsKeyPrefix, ss)
	credDefaults := map[string]model.RedsCredentials{"Cray": {Username: "groot", Password: "terminal6"}, "Cray ACE": {Username: "ace", Password: "ace"}, "Gigabyte": {Username: "Administrator", Password: "superuser"}}
	ss.Store(model.CredentialsKeyPrefix+"/defaults", credDefaults)

	// Test no request body
	rr := GetHTTPResponse(t, doPostCredentials, "POST", "/v1/credentials", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test incorrectly formatted request body
	rr = GetHTTPResponse(t, doPostCredentials, "POST", "/v1/credentials", bytes.NewBuffer(json.RawMessage(`{"foo":"bar"`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test correctly formatted request body with invalid data
	rr = GetHTTPResponse(t, doPostCredentials, "POST", "/v1/credentials", bytes.NewBuffer(json.RawMessage(`{"addresses":[]}`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test correctly formatted request with MAC addresses
	rr = GetHTTPResponse(t, doPostCredentials, "POST", "/v1/credentials", bytes.NewBuffer(json.RawMessage(`{"addresses":[{"macAddress":"00beef151337","IPAddresses":[{"addressType":"IPv4","address":"0.0.0.0"},{"addressType":"IPv6","address":"1:20:300:4000:5:60:700:8000"}]},{"macAddress":"00d00d15af00","IPAddresses":[{"addressType":"IPv4","address":"1.2.3.4"},{"addressType":"IPv6","address":"8:70:600:5000:4:30:200:1000"}]}]}`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	} else {
		expected := string(json.RawMessage(`{"username":"groot","password":"terminal6"}`)) + "\n"
		if rr.Body.String() != expected {
			t.Errorf("Unexpected response body: got %v, want %v",
				rr.Body.String(), expected)
		}
	}
}

func TestDoPutDiscovery(t *testing.T) {
	var httpReportIn HTTPReport
	// Pre-configuration: set up package global storage variable
	ss := mock.NewKvMock()
	credStorage = model.NewRedsCredStore(model.CredentialsKeyPrefix, ss)

	// Test no request body
	rr := GetHTTPResponse(t, doPutDiscovery, "PUT", "/v1/discovery", nil, false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test incorrectly formatted request body
	rr = GetHTTPResponse(t, doPutDiscovery, "PUT", "/v1/discovery", bytes.NewBuffer(json.RawMessage(`{"foo":"bar"`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test correctly formatted request body with invalid data
	rr = GetHTTPResponse(t, doPutDiscovery, "PUT", "/v1/discovery", bytes.NewBuffer(json.RawMessage(`{"addresses":[]}`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Successful 'PUT /discovery' requests produce reports that need to be consumed or testing will hang.
	// Spin up a goRoutine to consume the report.
	// Create the HTTPHandler's internal channel
	imchan = make(chan HTTPReport)
	done := make(chan bool)
	go func() {
		httpReportIn = <-imchan
		done <- true
	}()

	// Store test data
	testAddrs := storage.SystemAddresses{
		Addresses: []storage.BMCAddress{
			storage.BMCAddress{
				MACAddress: "00beef151337",
			},
			storage.BMCAddress{
				MACAddress: "00d00d15af00",
			},
		},
	}
	testData := storage.BMCCredItem{
		Credentials: storage.BMCCredentials{
			Username: "foo",
			Password: "bar",
		},
		BMCAddrs: &testAddrs,
	}
	credStorage.AddMacCredentials("00beef151337", testData)
	// Test correctly formatted request with MAC addresses
	rr = GetHTTPResponse(t, doPutDiscovery, "PUT", "/v1/discovery", bytes.NewBuffer(json.RawMessage(`{"addresses":[{"macAddress":"00beef151337","IPAddresses":[]},{"macAddress":"00d00d15af00","IPAddresses":[]}]}`)), false, "", "")
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v(%s) want %v",
			status, rr.Body, http.StatusOK)
		// Allows the goRoutine to finish since there will not be a report.
		imchan <- HTTPReport{}
		// Wait for the thread to finish
		<-done
	} else {
		expected := ""
		if rr.Body.String() != expected {
			t.Errorf("Unexpected response body: got %v, want %v", rr.Body.String(), expected)
		}
		// Wait for the thread to finish
		<-done
		expectedReportType := HTTPREPORT_CONFIG_COMPLETE
		if httpReportIn.reportType != expectedReportType {
			t.Errorf("Unexpected HTTPReport: got '%s', want '%s'", HTTPReportTypeString[httpReportIn.reportType], HTTPReportTypeString[expectedReportType])
		}
	}
}

func Test405(t *testing.T) {
	router := GetRouter()

	var cases = []badReq{{"http://localhost:8080/v1/credentials", "GET"},
		{"http://localhost:8080/v1/credentials", "PUT"},
		{"http://localhost:8080/v1/credentials", "DELETE"},
		{"http://localhost:8080/v1/discovery", "GET"},
		{"http://localhost:8080/v1/discovery", "POST"},
		{"http://localhost:8080/v1/discovery", "DELETE"},
	}

	// /credentials GET, PUT, DELETE

	for _, test := range cases {
		//req,rerr := http.NewRequest("GET","http://localhost:8080/v1/credentials",nil)
		req, rerr := http.NewRequest(test.method, test.url, bytes.NewBuffer(json.RawMessage(`{}`)))
		if rerr != nil {
			t.Error("ERROR forming request:", rerr)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("ERROR, disallowed %s operation bad return, wanted %d got %d",
				test.method, http.StatusMethodNotAllowed, rec.Code)
		}
	}
}
