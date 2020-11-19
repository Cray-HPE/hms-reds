// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package etcd

import (
	"encoding/json"
	"errors"
	"log"
	"path"
	"strings"

	hmetcd "stash.us.cray.com/HMS/hms-hmetcd"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
)

// The prefix to be prepended to all our keys.  Should begin and end with /
const keyPrefix = "/river-endpoint-discovery/"

// The prefix for switch state keys
const keyPrefixSwitchState = "switch-state/"

// The prefix for per-mac-address state keys
const keyPrefixMacState = "mac-state/"

const keyLiveness = "liveness"
const valueLiveness = "Look! It's moving. It's alive. It's alive... It's alive, it's moving, it's alive, it's alive, it's alive, it's alive, IT'S ALIVE!"

type Etcd struct {
	url    string
	prefix string
	store  hmetcd.Kvi
}

func (state *Etcd) fixKey(key string) string {
	// First check there's a leading slash
	if !strings.HasPrefix(key, "/") {
		key = path.Join("/", key)
	}

	// Then check that it includes our full prefix
	if !strings.HasPrefix(key, state.prefix) {
		key = path.Join(state.prefix, key)
	}
	return key
}

func (state *Etcd) makeSwitchKey(name string) string {
	return keyPrefix + keyPrefixSwitchState + name
}

func (state *Etcd) makeMacKey(name string) string {
	return keyPrefix + keyPrefixMacState + name
}

func (state *Etcd) Init(url string, insecure bool) error {
	var err error

	state.store, err = hmetcd.Open(url, "")
	state.prefix = keyPrefix

	if err != nil {
		return err
	}

	return nil
}

func (state *Etcd) GetSwitchState(name string) (map[string]string, error) {
	encoded, exists, err := state.store.Get(state.makeSwitchKey(name))
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	var ret map[string]string = make(map[string]string)
	err = json.Unmarshal([]byte(encoded), &ret)
	if err != nil {
		return make(map[string]string), errors.New("Unable to decode stored state for " + name + ": " + err.Error())
	}

	return ret, nil
}

func (state *Etcd) SetSwitchState(name string, switchState map[string]string) error {
	encoded, err := json.Marshal(switchState)
	if err != nil {
		return errors.New("Unable to encode state as JSON: " + err.Error())
	}

	err = state.store.Store(state.makeSwitchKey(name), string(encoded))
	if err != nil {
		return errors.New("Unable to store data for " + name + ": " + err.Error())
	}

	return nil
}

func (state *Etcd) GetMacState(mac string) (*storage.MacState, error) {
	encoded, exists, err := state.store.Get(state.makeMacKey(mac))
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	var ret *storage.MacState = new(storage.MacState)
	err = json.Unmarshal([]byte(encoded), &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (state *Etcd) SetMacState(mac string, macState storage.MacState) error {
	encoded, err := json.Marshal(macState)
	if err != nil {
		return errors.New("Unable to encode MacState: " + err.Error())
	}

	err = state.store.Store(state.makeMacKey(mac), string(encoded))

	if err != nil {
		return errors.New("Unable to store MacState: " + err.Error())
	}
	return nil
}

func (state *Etcd) ClearMacState(mac string) error {
	return state.store.Delete(state.makeMacKey(mac))
}

func (state *Etcd) CheckLiveness() bool {
	key := state.fixKey(keyLiveness)
	value := valueLiveness
	err := state.store.Store(key, value)
	if err != nil {
		log.Printf("WARNING: Unable to store key in etcd: %v", err)
		return false
	}

	rval, present, err := state.store.Get(key)
	if err != nil || present == false || rval != value {
		log.Printf("WARNING: Unable to retrieve key from etcd: %v", err)
		return false
	}

	err = state.store.Delete(key)
	if err != nil {
		log.Printf("WARNING: Unable to delete key in etcd: %v", err)
		return false
	}

	return true
}
