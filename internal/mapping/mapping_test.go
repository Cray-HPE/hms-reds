// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package mapping

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	compcredentials "stash.us.cray.com/HMS/hms-compcredentials"
	sstorage "stash.us.cray.com/HMS/hms-securestorage"
)

const SLS_BASE_HOSTNAME = "cray-sls"
const SLS_BASE_VERSION = "v1"
const SLS_BASE_URL = SLS_BASE_HOSTNAME + "/" + SLS_BASE_VERSION

var client *http.Client

// RoundTrip method override
type RTFunc func(req *http.Request) *http.Response

// Implement RoundTrip interface by implementing RoundTrip method
func (f RTFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(f RTFunc) *http.Client {
	return &http.Client{
		Transport: RTFunc(f),
	}
}

type MockSS struct {
	kvstore map[string]string
}

func (ms MockSS) Store(key string, value interface{}) error {
	jsonVal, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ms.kvstore[key] = string(jsonVal)
	return nil
}
func (ms MockSS) Lookup(key string, output interface{}) error {
	jVal, ok := ms.kvstore[key]
	if !ok {
		return errors.New("Key not found")
	}
	err := json.Unmarshal([]byte(jVal), output)
	return err
}
func (ms MockSS) Delete(key string) error {
	if _, ok := ms.kvstore[key]; !ok {
		return errors.New("Key not found")
	}
	delete(ms.kvstore, key)
	return nil
}
func (ms MockSS) LookupKeys(keyPath string) ([]string, error) { return nil, nil }

var mss sstorage.SecureStorage = MockSS{
	kvstore: map[string]string{
		"snmp-creds/x0c0w0": "{\"XName\": \"x0c0w0\",\"SNMPUser\": \"username\", " +
			"\"SnmpAuthPassword\": \"dummy1\",\"SnmpPrivPassword\": \"dummy2\"}",
		"snmp-creds/x0c0w1": "{\"XName\": \"x0c0w1\",\"SNMPUser\": \"nameuser\", " +
			"\"SnmpAuthPassword\": \"dummy3\",\"SnmpPrivPassword\": \"dummy4\"}",
	},
}

var payloadSLSSwitches = `[
	{
		"Parent": "x0c0",
		"Children": [
			"x0c0w0j1",
			"x0c0w0j2"
		],
		"XName": "x0c0w0",
		"Type": "comptype_mgmt_switch",
		"TypeString": "MgmtSwitch",
		"Class": "river",
		"ExtraProperties": {
			"IP6addr": "DHCPv6",
			"IP4addr": "10.1.1.1",
			"Username": "username",
			"Password": "vault://tok",
			"SNMPUsername": "username",
			"SNMPAuthPassword": "vault://hms-creds/x0c0w0",
			"SNMPAuthProtocol": "MD5",
			"SNMPPrivPassword": "vault://hms-creds/x0c0w0",
			"SNMPPrivProtocol": "DES",
			"Model": "Dell S3048-ON"
		}
	},
	{
		"Parent": "x0c0",
		"Children": [
			"x0c0w1j1",
			"x0c0w1j2"
		],
		"XName": "x0c0w1",
		"Type": "comptype_mgmt_switch",
		"TypeString": "MgmtSwitch",
		"Class": "river",
		"ExtraProperties": {
			"IP6addr": "fe80::48",
			"IP4addr": "10.1.1.1",
			"Username": "nameuser",
			"Password": "vault://tok",
			"SNMPUsername": "username",
			"SNMPAuthPassword": "vault://hms-creds/x0c0w1",
			"SNMPAuthProtocol": "MD5",
			"SNMPPrivPassword": "vault://hms-creds/x0c0w1",
			"SNMPPrivProtocol": "DES",
			"Model": "Dell S3048-ON"
		}
	}
]`

var payloadSLSSwitchByName = `{
	"Parent": "x0c0",
	"Children": [
		"x0c0w0j1",
		"x0c0w0j2"
	],
	"XName": "x0c0w0",
	"Type": "comptype_mgmt_switch",
	"TypeString": "MgmtSwitch",
	"Class": "river",
	"ExtraProperties": {
		"IP6addr": "DHCPv6",
		"IP4addr": "10.1.1.1",
		"Username": "username",
		"Password": "vault://tok",
		"SNMPUsername": "username",
		"SNMPAuthPassword": "vault://hms-creds/x0c0w0",
		"SNMPAuthProtocol": "MD5",
		"SNMPPrivPassword": "vault://hms-creds/x0c0w0",
		"SNMPPrivProtocol": "DES",
		"Model": "Dell S3048-ON"
	}
}`

var payloadSLSSwitchPorts = `[
	{
		"Parent": "x0c0w0",
		"XName": "x0c0w0j1",
		"Type": "comptype_mgmt_switch_connector",
		"TypeString": "MgmtSwitchConnector",
		"Class": "river",
		"ExtraProperties": {
			"NodeNics": [
				"x0c0s0b0i0",
				"x0c0s1b0"
			],
			"VendorName": "GigabitEthernet 1/31"
		}
	},
	{
		"Parent": "x0c0w0",
		"XName": "x0c0w0j2",
		"Type": "comptype_mgmt_switch_connector",
		"TypeString": "MgmtSwitchConnector",
		"Class": "river",
		"ExtraProperties": {
			"NodeNics": [
				"x0c0s2b0",
				"x0c0s3b0i0"
			],
			"VendorName": "GigabitEthernet 1/32"
		}
	}
]`

var payloadSLSSwitchPortByIFName = `[
	{
		"Parent": "x0c0w0",
		"XName": "x0c0w0j1",
		"Type": "comptype_mgmt_switch_connector",
		"TypeString": "MgmtSwitchConnector",
		"Class": "river",
		"ExtraProperties": {
			"NodeNics": [
				"x0c0s0b0i0",
				"x0c0s1b0"
			],
			"VendorName": "GigabitEthernet 1/31"
		}
	},
	{
		"Parent": "x0c0w0",
		"XName": "x0c0w0j1",
		"Type": "comptype_mgmt_switch_connector",
		"TypeString": "MgmtSwitchConnector",
		"Class": "river",
		"ExtraProperties": {
			"NodeNics": [
				"x0c0s0b0i0",
				"x0c0s1b0"
			],
			"VendorName": "GigabitEthernet 1/31"
		}
	}
]`

func BaseRTFunc(r *http.Request) *http.Response {
	switch r.URL.Path {
	case "/" + SLS_BASE_VERSION + "/" + SLS_SEARCH_HARDWARE_ENDPOINT:
		switch r.URL.Query().Encode() {
		case "class=River&type=comptype_mgmt_switch":
			return &http.Response{
				StatusCode: 200,
				// Send mock response for rpath
				Body:   ioutil.NopCloser(bytes.NewBufferString(payloadSLSSwitches)),
				Header: make(http.Header),
			}
		case "parent=x0c0w0&type=comptype_mgmt_switch_connector":
			return &http.Response{
				StatusCode: 200,
				// Send mock response for rpath
				Body:   ioutil.NopCloser(bytes.NewBufferString(payloadSLSSwitchPorts)),
				Header: make(http.Header),
			}
		case "parent=x0c0w0":
			return &http.Response{
				StatusCode: 200,
				// Send mock response for rpath
				Body:   ioutil.NopCloser(bytes.NewBufferString(payloadSLSSwitchPortByIFName)),
				Header: make(http.Header),
			}
		}
	case "/" + SLS_BASE_VERSION + "/hardware/x0c0w0":
		return &http.Response{
			StatusCode: 200,
			// Send mock response for rpath
			Body:   ioutil.NopCloser(bytes.NewBufferString(payloadSLSSwitchByName)),
			Header: make(http.Header),
		}
	}

	return &http.Response{
		StatusCode: 404,
		Body:       ioutil.NopCloser(bytes.NewBufferString("Unknown request for path " + r.URL.Path + ", query: " + r.URL.Query().Encode())),
		Header:     make(http.Header),
	}
}

func Test_SLS_GetSwitches(t *testing.T) {
	compcreds = compcredentials.NewCompCredStore(compcredentials.DefaultCompCredPath, mss)
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w0",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "dummy1",
		SNMPPrivPass: "dummy2",
	})
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w1",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "dummy3",
		SNMPPrivPass: "dummy4",
	})
	log.Printf("%v", compcreds)

	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(BaseRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go WatchSLSNewManagementNodes(nodeQuitChan)

	switches, err := GetSwitches()
	if err != nil {
		t.Fatalf("Unexpected error retreiving switches: %s", err)
	}

	if _, ok := (*switches)["x0c0w0"]; !ok {
		t.Fatalf("Couldn't find x0c0w0 in returned switch list")
	}

	expectedx0c0w0 := Switch{
		Id:               "x0c0w0",
		Address:          "10.1.1.1",
		SnmpUser:         "username",
		SnmpAuthPassword: "dummy1",
		SnmpAuthProtocol: "MD5",
		SnmpPrivPassword: "dummy2",
		SnmpPrivProtocol: "DES",
	}

	x0c0w0 := (*switches)["x0c0w0"]
	if x0c0w0.Id != expectedx0c0w0.Id {
		t.Fatalf("x0c0w0 has wrong Xname/Id.  Expected \"%s\" got \"%s\"", expectedx0c0w0.Id, x0c0w0.Id)
	}
	if x0c0w0.SnmpUser != expectedx0c0w0.SnmpUser {
		t.Fatalf("x0c0w0 has wrong username.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpUser, x0c0w0.SnmpUser)
	}
	if x0c0w0.SnmpAuthPassword != expectedx0c0w0.SnmpAuthPassword {
		t.Fatalf("x0c0w0 has wrong SNMP Auth password.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpAuthPassword, x0c0w0.SnmpAuthPassword)
	}
	if x0c0w0.SnmpPrivPassword != expectedx0c0w0.SnmpPrivPassword {
		t.Fatalf("x0c0w0 has wrong SNMP Priv password.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpPrivPassword, x0c0w0.SnmpPrivPassword)
	}
	if x0c0w0.Address != expectedx0c0w0.Address {
		t.Fatalf("x0c0w0 has wrong address.  Expected \"%s\" got \"%s\"", expectedx0c0w0.Address, x0c0w0.Address)
	}

	if _, ok := (*switches)["x0c0w1"]; !ok {
		t.Fatalf("Couldn't find x0c0w1 in returned switch list")
	}

	expectedx0c0w1 := Switch{
		Address:          "fe80::48",
		SnmpUser:         "username",
		SnmpAuthPassword: "dummy3",
		SnmpAuthProtocol: "MD5",
		SnmpPrivPassword: "dummy4",
		SnmpPrivProtocol: "DES",
	}

	x0c0w1 := (*switches)["x0c0w1"]
	if x0c0w1.SnmpUser != expectedx0c0w1.SnmpUser {
		t.Fatalf("x0c0w1 has wrong username.  Expected \"%s\" got \"%s\"", expectedx0c0w1.SnmpUser, x0c0w1.SnmpUser)
	}
	if x0c0w1.SnmpAuthPassword != expectedx0c0w1.SnmpAuthPassword {
		t.Fatalf("x0c0w1 has wrong SNMP Auth password.  Expected \"%s\" got \"%s\"", expectedx0c0w1.SnmpAuthPassword, x0c0w1.SnmpAuthPassword)
	}
	if x0c0w1.SnmpPrivPassword != expectedx0c0w1.SnmpPrivPassword {
		t.Fatalf("x0c0w1 has wrong SNMP Priv password.  Expected \"%s\" got \"%s\"", expectedx0c0w1.SnmpPrivPassword, x0c0w1.SnmpPrivPassword)
	}
	if x0c0w1.Address != expectedx0c0w1.Address {
		t.Fatalf("x0c0w1 has wrong address.  Expected \"%s\" got \"%s\"", expectedx0c0w1.Address, x0c0w1.Address)
	}

	switchQuitChan <- true
	nodeQuitChan <- true
}

func Test_SLS_GetSwitchByName(t *testing.T) {
	compcreds = compcredentials.NewCompCredStore(compcredentials.DefaultCompCredPath, mss)
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w0",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "dummy1",
		SNMPPrivPass: "dummy2",
	})
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w1",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "dummy3",
		SNMPPrivPass: "dummy4",
	})
	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(BaseRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go WatchSLSNewManagementNodes(nodeQuitChan)

	tswitch, err := GetSwitchByName("x0c0w0")
	if err != nil {
		t.Fatalf("Unexpected error retreiving switches: %s", err)
	}

	if tswitch == nil {
		t.Fatalf("Couldn't find x0c0w0 in returned switch list")
	}

	expectedx0c0w0 := Switch{
		Address:          "10.1.1.1",
		SnmpUser:         "username",
		SnmpAuthPassword: "dummy1",
		SnmpAuthProtocol: "MD5",
		SnmpPrivPassword: "dummy2",
		SnmpPrivProtocol: "DES",
	}

	x0c0w0 := tswitch
	if x0c0w0.SnmpUser != expectedx0c0w0.SnmpUser {
		t.Fatalf("x0c0w0 has wrong username.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpUser, x0c0w0.SnmpUser)
	}
	if x0c0w0.SnmpAuthPassword != expectedx0c0w0.SnmpAuthPassword {
		t.Fatalf("x0c0w0 has wrong SNMP Auth password.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpAuthPassword, x0c0w0.SnmpAuthPassword)
	}
	if x0c0w0.SnmpPrivPassword != expectedx0c0w0.SnmpPrivPassword {
		t.Fatalf("x0c0w0 has wrong SNMP Priv password.  Expected \"%s\" got \"%s\"", expectedx0c0w0.SnmpPrivPassword, x0c0w0.SnmpPrivPassword)
	}
	if x0c0w0.Address != expectedx0c0w0.Address {
		t.Fatalf("x0c0w0 has wrong address.  Expected \"%s\" got \"%s\"", expectedx0c0w0.Address, x0c0w0.Address)
	}

	switchQuitChan <- true
	nodeQuitChan <- true
}

func Test_SLS_GetSwitchPorts(t *testing.T) {
	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(BaseRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go WatchSLSNewManagementNodes(nodeQuitChan)

	switchPorts, err := GetSwitchPorts("x0c0w0")
	if err != nil {
		t.Fatalf("Unexpected error retreiving switches: %s", err)
	}

	if len(*switchPorts) == 0 {
		t.Fatalf("Couldn't find x0c0w0 in returned switch list")
	}

	expectedx0c0w0j1 := SwitchPort{
		Id:     0,
		IfName: "GigabitEthernet 1/31",
		PeerID: "x0c0s1b0",
	}

	actualx0c0w0j1 := (*switchPorts)[0]

	if expectedx0c0w0j1.IfName != actualx0c0w0j1.IfName {
		t.Fatalf("x0c0w0j1 has wrong IfName.  Expected %s, got %s", expectedx0c0w0j1.IfName, actualx0c0w0j1.IfName)
	}
	if expectedx0c0w0j1.PeerID != actualx0c0w0j1.PeerID {
		t.Fatalf("x0c0w0j1 has wrong PeerID.  Expected %s, got %s", expectedx0c0w0j1.PeerID, actualx0c0w0j1.PeerID)
	}

	expectedx0c0w0j2 := SwitchPort{
		Id:     0,
		IfName: "GigabitEthernet 1/32",
		PeerID: "x0c0s2b0",
	}

	actualx0c0w0j2 := (*switchPorts)[1]

	if expectedx0c0w0j2.IfName != actualx0c0w0j2.IfName {
		t.Fatalf("x0c0w0j2 has wrong IfName.  Expected %s, got %s", expectedx0c0w0j2.IfName, actualx0c0w0j2.IfName)
	}
	if expectedx0c0w0j2.PeerID != actualx0c0w0j2.PeerID {
		t.Fatalf("x0c0w0j2 has wrong PeerID.  Expected %s, got %s", expectedx0c0w0j2.PeerID, actualx0c0w0j2.PeerID)
	}

	switchQuitChan <- true
	nodeQuitChan <- true
}

func Test_SLS_GetSwitchPortByIFName(t *testing.T) {
	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(BaseRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go WatchSLSNewManagementNodes(nodeQuitChan)

	switchPort, err := GetSwitchPortByIFName("x0c0w0", `GigabitEthernet 1/31`)
	if err != nil {
		t.Fatalf("Unexpected error retreiving switches: %s", err)
	}

	if switchPort == nil {
		t.Fatalf("No data returned by SLS")
	}

	expected := SwitchPort{
		Id:     0,
		IfName: "GigabitEthernet 1/31",
		PeerID: "x0c0s1b0",
	}

	if expected.IfName != switchPort.IfName {
		t.Fatalf("returned result has wrong IfName.  Expected %s, got %s", expected.IfName, switchPort.IfName)
	}
	if expected.PeerID != switchPort.PeerID {
		t.Fatalf("returned result has wrong PeerID.  Expected %s, got %s", expected.PeerID, switchPort.PeerID)
	}

	switchQuitChan <- true
	nodeQuitChan <- true
}

var payloadTimedSLSSwitches0 = `[
	{
		"Parent": "x0c0",
		"Children": [
			"x0c0w0j1",
			"x0c0w0j2"
		],
		"XName": "x0c0w0",
		"Type": "comptype_mgmt_switch",
		"TypeString": "MgmtSwitch",
		"Class": "river",
		"ExtraProperties": {
			"IP6addr": "DHCPv6",
			"IP4addr": "10.1.1.1",
			"Username": "username",
			"Password": "vault://tok",
			"SNMPUsername": "username",
			"SNMPAuthPassword": "vault://snmp-creds/x0c0w0",
			"SNMPAuthProtocol": "MD5",
			"SNMPPrivPassword": "vault://snmp-creds/x0c0w0",
			"SNMPPrivProtocol": "DES",
			"Model": "Dell S3048-ON"
		}
	}
]`

var payloadTimedSLSSwitches1 = `[
	{
		"Parent": "x0c0",
		"Children": [
			"x0c0w0j1",
			"x0c0w0j2"
		],
		"XName": "x0c0w0",
		"Type": "comptype_mgmt_switch",
		"TypeString": "MgmtSwitch",
		"Class": "river",
		"ExtraProperties": {
			"IP6addr": "DHCPv6",
			"IP4addr": "10.1.1.1",
			"Username": "username",
			"Password": "vault://tok",
			"SNMPUsername": "username",
			"SNMPAuthPassword": "vault://snmp-creds/x0c0w0",
			"SNMPAuthProtocol": "MD5",
			"SNMPPrivPassword": "vault://snmp-creds/x0c0w0",
			"SNMPPrivProtocol": "DES",
			"Model": "Dell S3048-ON"
		}
	},
	{
		"Parent": "x0c0",
		"Children": [
			"x0c0w1j1",
			"x0c0w1j2"
		],
		"XName": "x0c0w1",
		"Type": "comptype_mgmt_switch",
		"TypeString": "MgmtSwitch",
		"Class": "river",
		"ExtraProperties": {
			"IP6addr": "fe80::48",
			"IP4addr": "10.1.1.1",
			"Username": "nameuser",
			"Password": "vault://tok",
			"SNMPUsername": "username",
			"SNMPAuthPassword": "vault://snmp-creds/x0c0w1",
			"SNMPAuthProtocol": "MD5",
			"SNMPPrivPassword": "vault://snmp-creds/x0c0w1",
			"SNMPPrivProtocol": "DES",
			"Model": "Dell S3048-ON"
		}
	}
]`

var TimedSwitchHitCount = 0

func TimedSwitchesRTFunc(r *http.Request) *http.Response {
	log.Printf("TimedSwitchesRTFunc called, number %d", TimedSwitchHitCount)
	switch r.URL.Path {
	case "/" + SLS_BASE_VERSION + "/" + SLS_SEARCH_HARDWARE_ENDPOINT:
		TimedSwitchHitCount++
		if TimedSwitchHitCount == 2 {
			log.Printf("TimedSwitchesRTFunc returns, number %d", TimedSwitchHitCount)
			return &http.Response{
				StatusCode: 200,
				// Send mock response for rpath
				Body:   ioutil.NopCloser(bytes.NewBufferString(payloadTimedSLSSwitches0)),
				Header: make(http.Header),
			}
		} else {
			log.Printf("TimedSwitchesRTFunc returns, number %d", TimedSwitchHitCount)
			return &http.Response{
				StatusCode: 200,
				// Send mock response for rpath
				Body:   ioutil.NopCloser(bytes.NewBufferString(payloadTimedSLSSwitches1)),
				Header: make(http.Header),
			}
		}
	}
	return BaseRTFunc(r)
}

var callbackHitCount = 0

func testCallback() {
	callbackHitCount++
}

func Test_SLS_watchSLSNewSwitches(t *testing.T) {
	slsSleepPeriod = 1
	callbackHitCount = 0
	TimedSwitchHitCount = 0

	OnNewMapping(testCallback)

	compcreds = compcredentials.NewCompCredStore(compcredentials.DefaultCompCredPath, mss)
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w0",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "abc123",
		SNMPPrivPass: "zyx987",
	})
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w1",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "abc123",
		SNMPPrivPass: "zyx987",
	})

	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(TimedSwitchesRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	time.Sleep(time.Duration(4*slsSleepPeriod) * time.Second)

	log.Printf("NewSwitches Callback count is %d", callbackHitCount)

	if callbackHitCount != 3 {
		t.Fatalf("Callback hit count was %d, should have been 3", callbackHitCount)
	}

	switchQuitChan <- true
}

var callbackHitCountNC = 0

func testCallbackNC() {
	callbackHitCountNC++
}

func Test_SLS_watchSLSNewSwitchesNoChange(t *testing.T) {
	slsSleepPeriod = 1
	callbackHitCountNC = 0

	OnNewMapping(testCallbackNC)

	compcreds = compcredentials.NewCompCredStore(compcredentials.DefaultCompCredPath, mss)
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w0",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "abc123",
		SNMPPrivPass: "zyx987",
	})
	compcreds.StoreCompCred(compcredentials.CompCredentials{
		Xname:        "x0c0w1",
		Username:     "groot",
		Password:     "termainl6",
		SNMPAuthPass: "abc123",
		SNMPPrivPass: "zyx987",
	})

	print("%v", mss)

	ConfigureSLSMode(SLS_BASE_URL, NewTestClient(BaseRTFunc), &mss, compcreds)

	switchQuitChan := make(chan bool)
	go WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go WatchSLSNewManagementNodes(nodeQuitChan)

	time.Sleep(time.Duration(3*slsSleepPeriod) * time.Second)

	log.Printf("NCCallback count is %d", callbackHitCountNC)

	if callbackHitCountNC != 1 {
		t.Fatalf("Callback hit count was %d, should have been 1 (for startup)", callbackHitCountNC)
	}

	switchQuitChan <- true
	nodeQuitChan <- true
}
