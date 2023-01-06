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
	"path"
	"strings"

	hmetcd "github.com/Cray-HPE/hms-hmetcd"
	"github.com/Cray-HPE/hms-reds/internal/storage"
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
