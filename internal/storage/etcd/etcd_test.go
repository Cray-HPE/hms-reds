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

package etcd

import (
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"testing"

	hmetcd "stash.us.cray.com/HMS/hms-hmetcd"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
)

func TestGetSwitchState_uninitialized(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)

	state, err := etcdi.GetSwitchState("test-switch")
	if err != nil {
		t.Errorf("error was not nil and should have been: %s", err.Error())
	}

	if len(state) != 0 {
		t.Errorf("returned state was not empty and should have been!")
	}
}

func TestGetSwitchState_initialized(t *testing.T) {
	fakeState := map[string]string{
		"1cedc0ffee00": "FastEthernet 1/1",
	}

	data, err := json.Marshal(fakeState)
	if err != nil {
		t.Errorf("Failed to encode data: %s", err.Error())
	}

	etcdi := new(Etcd)
	etcdi.Init("mem:", false)
	_ = etcdi.store.Store(etcdi.fixKey("switch-state/test-switch"), string(data))
	state, err := etcdi.GetSwitchState("test-switch")
	if err != nil {
		t.Errorf("error was not nil and should have been: %s", err.Error())
	}

	if len(state) != 1 {
		t.Errorf("returned state was not length 1 and should have been!")
	}

	val, ok := state["1cedc0ffee00"]
	if !ok {
		t.Errorf("Key didn't come back out! %s", state)
	}

	if val != "FastEthernet 1/1" {
		t.Errorf("Expected FastEthernet 1/1, got: %s", val)
	}
}

func TestSetSwitchState(t *testing.T) {
	state := map[string]string{
		"1cedc0ffee00": "FastEthernet 2/3",
	}
	jstate, _ := json.Marshal(state)

	etcdi := new(Etcd)
	etcdi.Init("mem:", false)
	err := etcdi.SetSwitchState("test-switch", state)
	if err != nil {
		t.Errorf("Setting switch state failed: %s", err.Error())
	}

	res, _, _ := etcdi.store.Get(etcdi.fixKey("/switch-state/test-switch"))

	if res != string(jstate) {
		t.Errorf("Data didn't ocme back out OK. Got:\n%s\nExpected:\n:%s", res, jstate)
	}
}

func TestGetMacState_empty(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)

	obj, err := etcdi.GetMacState("1cedc0ffee00")

	if err != nil {
		t.Errorf("Unexpected error occurred getting non-existant mac: %s", err.Error())
	}

	if obj != nil {
		t.Errorf("Returned object was not nil (as was expected)")
	}
}

func TestGetMacState_exists(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)

	inobj := new(storage.MacState)
	inobj.DiscoveredHTTP = false
	inobj.DiscoveredSNMP = false
	inobj.SwitchName = "SomeName"
	inobj.SwitchPort = "fortyGigabit 3/12"
	inobj.Username = "GdlqGFKE"
	inobj.Password = "EHFbesjfbE"
	inobj.IPAddress = ""

	inobjJSON, _ := json.Marshal(inobj)
	etcdi.store.Store(etcdi.fixKey("mac-state/1cedc0ffee00"), string(inobjJSON))

	outobj, err := etcdi.GetMacState("1cedc0ffee00")
	if err != nil {
		t.Errorf("Unexpected error occurred retreving object: %s", err.Error())
	}

	if !reflect.DeepEqual(inobj, outobj) {
		t.Errorf("Objects are not equal: In: %v\nOut:%v", inobj, outobj)
	}
}

func TestSetMacState(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)

	inobj := new(storage.MacState)
	inobj.DiscoveredHTTP = false
	inobj.DiscoveredSNMP = false
	inobj.SwitchName = "SomeName"
	inobj.SwitchPort = "fortyGigabit 3/12"
	inobj.Username = "GdlqGFKE"
	inobj.Password = "EHFbesjfbE"
	inobj.IPAddress = ""

	err := etcdi.SetMacState("1cedc0ffee00", *inobj)
	if err != nil {
		t.Errorf("UNexpected error storing state: %s", err.Error())
	}

	outJSON, _ := json.Marshal(inobj)
	outobj, _, err := etcdi.store.Get(etcdi.fixKey("mac-state/1cedc0ffee00"))
	if err != nil {
		t.Errorf("Unexpected error looking up object: %s", err.Error())
	}
	if string(outJSON) != outobj {
		t.Errorf("Returned obejct differs!\n Input: %s\nOutput: %s", outJSON, outobj)
	}
}

func TestClearMacState(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)

	inobj := new(storage.MacState)
	inobj.DiscoveredHTTP = false
	inobj.DiscoveredSNMP = false
	inobj.SwitchName = "SomeName"
	inobj.SwitchPort = "fortyGigabit 3/12"
	inobj.Username = "GdlqGFKE"
	inobj.Password = "EHFbesjfbE"
	inobj.IPAddress = ""

	inobjJSON, _ := json.Marshal(inobj)
	etcdi.store.Store("mac-state/1cedc0ffee00", string(inobjJSON))

	err := etcdi.ClearMacState("1cedc0ffee00")
	if err != nil {
		t.Errorf("Unexpected error occurred retreving object: %s", err.Error())
	}

	outobj, err := etcdi.GetMacState("1cedc0ffee00")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	if outobj != nil {
		mysteryObj, _ := json.Marshal(outobj)
		t.Errorf("Unexpectedly got back an object!?: %s", string(mysteryObj))
	}
}

func TestEtcdLiveness(t *testing.T) {
	etcdi := new(Etcd)
	etcdi.Init("mem:", false)
	if etcdi.CheckLiveness() != true {
		t.Errorf("Etcd is not alive?!")
	}
}

/*
Test what happens when we can't store things
*/
type KVIMock struct {
	hmetcd.Kvi
	failGet       bool
	getCallNumber int
	failStore     bool
	failDelete    bool
}

func (state KVIMock) Get(key string) (string, bool, error) {
	if state.failGet {
		log.Printf("Mock GET() call # %d", state.getCallNumber)
		if state.getCallNumber == 1 {
			return "", true, nil
		} else if state.getCallNumber == 2 {
			return "", false, nil
		} else if state.getCallNumber == 3 {
			return "", false, errors.New("Test error")
		} else {
			return valueLiveness, true, nil
		}
	}
	return valueLiveness, true, nil
}

func (state KVIMock) Store(key string, value string) error {
	if state.failStore {
		return errors.New("Test error")
	}
	return nil
}

func (state KVIMock) Delete(key string) error {
	if state.failDelete {
		return errors.New("Test error")
	}
	return nil
}

func TestEtcdLiveness_noStore(t *testing.T) {
	etcdi := new(Etcd)
	mockStore := new(KVIMock)
	mockStore.failStore = true
	etcdi.Init("mem:", false)
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == true {
		t.Errorf("Etcd is not dead?!")
	}
}

func TestEtcdLiveness_noGet(t *testing.T) {
	etcdi := new(Etcd)
	mockStore := new(KVIMock)
	mockStore.failGet = true
	etcdi.Init("mem:", false)
	etcdi.store = *mockStore
	// Sequential calls to Get() in the Etcd mock shoudl result in
	// different conditions failing, hence the first 3 calls.
	// The fourth shoudl succeed.
	mockStore.getCallNumber = 1
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == true {
		t.Errorf("Etcd is not dead?! (test 1)")
	}
	mockStore.getCallNumber = 2
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == true {
		t.Errorf("Etcd is not dead?! (test 2)")
	}
	mockStore.getCallNumber = 3
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == true {
		t.Errorf("Etcd is not dead?! (test 3)")
	}
	mockStore.getCallNumber = 4
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == false {
		t.Errorf("Etcd is dead?! (test 4)")
	}
}

/*
Test what happens when we can't delete things
*/
func TestEtcdLiveness_noDelete(t *testing.T) {
	etcdi := new(Etcd)
	mockStore := new(KVIMock)
	mockStore.failDelete = true
	etcdi.Init("mem:", false)
	etcdi.store = *mockStore
	if etcdi.CheckLiveness() == true {
		t.Errorf("Etcd is not dead?!")
	}
}

/*
Test what happens if a real ETCD URL won't work, should fail cleanly.
*/
func TestMakeStorage(t *testing.T) {
	etcdi := new(Etcd)
	err := etcdi.Init("mem:", false)
	if (err != nil) {
		t.Errorf("ETCD Init() with 'mem:' failed: %v",err)
	}

	etcdi2 := new(Etcd)
	err2 := etcdi2.Init("localhost:9897",false)
	if (err2 == nil) {
		t.Errorf("ETCD Init() with bogus URL did not fail!")
	}
}

