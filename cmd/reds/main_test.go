/*
 * MIT License
 *
 * (C) Copyright [2019-2021] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

package main

import (
	"encoding/json"
	"testing"

	"stash.us.cray.com/HMS/hms-reds/internal/storage"
	storage_factory "stash.us.cray.com/HMS/hms-reds/internal/storage/factory"
)

func Test_isReadyForHSMAdd(t *testing.T) {
	var state storage.MacState

	if isReadyForHSMAdd(state) {
		t.Errorf("Returned true with (false,false)")
	}

	state.DiscoveredSNMP = true
	if isReadyForHSMAdd(state) {
		t.Errorf("Returned true with (true,false)")
	}

	state.DiscoveredHTTP = true
	if !isReadyForHSMAdd(state) {
		t.Errorf("Returned false with (true,true)")
	}

	state.DiscoveredSNMP = false
	if isReadyForHSMAdd(state) {
		t.Errorf("Returned true with (false, true)")
	}
}

func Test_handleSNMPAddAction(t *testing.T) {
	// This test needs to be updated with some kind of mock for the mapping module
	// Otherwise, it doesn't actually have a way to run this test wihtout
	// interacting with SLS
	/*var err error
	mainstorage, err = storage_factory.MakeStorage("etcd", "mem:", false)
	if err != nil {
		t.Errorf("Unable to connect to storage: %s", err)
	}

	mapping.SetMapping(`{
		"version": 1,
		"switches": [
			{
				"id": "TestSwitch",
				"address": "10.4.255.254",
				"snmpUser": "sdgdgs",
				"snmpAuthPassword": "sdgsdg",
				"snmpAuthProtocol": "MD5",
				"snmpPrivPassword": "dsgsdg",
				"snmpPrivProtocol": "DES",
				"ports": [
					{
						"id": 1,
						"ifName": "FastEthernet 1/10",
						"peerID": "x0c0s28b0"
					}
				]
			}
		]
	}`)

	var rpt SNMPReport = SNMPReport{
		macAddr:    "001cedc0ffee",
		switchName: "TestSwitch",
		port:       "FastEthernet 1/10",
		eventType:  snmp_common.Action_Add,
	}

	handleSNMPAddAction(rpt)

	resRaw, err := mainstorage.GetMacState("001cedc0ffee")
	if err != nil {
		t.Errorf("Unable to retrieve result: " + err.Error())
	}
	if resRaw == nil {
		t.Error("No result found")
	}
	res, err := json.Marshal(resRaw)
	if err != nil {
		t.Errorf("Unable to encode result: " + err.Error())
	}

	var expectedRaw = storage.MacState{
		DiscoveredHTTP: false,
		DiscoveredSNMP: true,
		SwitchName:     "TestSwitch",
		SwitchPort:     "FastEthernet 1/10",
		Username:       "",
		Password:       "",
		IPAddress:      "",
	}

	expected, _ := json.Marshal(expectedRaw)
	if string(res) != string(expected) {
		t.Errorf("Result mismatch.  Got:\n%s\nExpected:\n%s", res, expected)
	}
	*/
}

func Test_handleHTTPDiscovered(t *testing.T) {
	mstorage, err := storage_factory.MakeStorage("etcd", "mem:", false)
	mainstorage = mstorage
	if err != nil {
		t.Errorf("Unable to connect to storage: %s", err)
	}

	var rpt = HTTPReport{
		bmcAddrs: []storage.BMCAddress{
			storage.BMCAddress{
				MACAddress: "001cedc0ffff",
			},
		},
		username: "testuser",
		password: "12345678",
	}

	handleHTTPDiscovered(rpt)

	resRaw, err := mainstorage.GetMacState("001cedc0ffff")
	if err != nil {
		t.Errorf("Unable to retrieve result: " + err.Error())
	}
	if resRaw == nil {
		t.Error("No result found")
	}
	res, err := json.Marshal(resRaw)
	if err != nil {
		t.Errorf("Unable to encode result: " + err.Error())
	}

	var expectedRaw = storage.MacState{
		DiscoveredHTTP: true,
		DiscoveredSNMP: false,
		SwitchName:     "",
		SwitchPort:     "",
		Username:       "testuser",
		Password:       "12345678",
		IPAddress:      "",
	}

	expected, _ := json.Marshal(expectedRaw)
	if string(res) != string(expected) {
		t.Errorf("Result mismatch.  Got:\n%s\nExpected:\n%s", res, expected)
	}
}
