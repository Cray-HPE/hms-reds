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

package model

import (
	"errors"
	"fmt"

	"github.com/Cray-HPE/hms-reds/internal/storage"
	sstorage "github.com/Cray-HPE/hms-securestorage"
)

const CredentialsKeyPrefix = "secret/reds-cred"

type RedsCredStore struct {
	CCPath string
	SS     sstorage.SecureStorage
}

type RedsCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Due to the sensitive nature of the data in RedsCredentials, make a custom String function
// to prevent passwords from being printed directly (accidentally) to output.
func (redsCred RedsCredentials) String() string {
	return fmt.Sprintf("Username: %s, Password: <REDACTED>", redsCred.Username)
}

type SwitchCredentials struct {
	SNMPUsername     string
	SNMPAuthPassword string
	SNMPPrivPassword string
}

func (switchCredentials SwitchCredentials) String() string {
	return fmt.Sprintf("SNMPUsername: %s, SNMPAuthPassword: <REDACTED>, SNMPPrivPassword: <REDACTED>",
		switchCredentials.SNMPUsername)
}

// Create a new RedsCredStore struct that uses a SecureStorage backing store.
func NewRedsCredStore(keyPath string, ss sstorage.SecureStorage) *RedsCredStore {
	ccs := &RedsCredStore{
		CCPath: keyPath,
		SS:     ss,
	}
	return ccs
}

// Stores a BMCCredItem for the given mac address
// Arguments:
// - mac (string): The mac address to set the object for
// - creds (BMCCredItem): The object to store
// Returns:
// - error: Any error that occurred or nil
func (ccs *RedsCredStore) AddMacCredentials(mac string, creds storage.BMCCredItem) error {

	err := ccs.SS.Store(ccs.CCPath+"/"+mac, creds)

	if err != nil {
		return errors.New("Unable to store BMCCredItem: " + err.Error())
	}
	return nil
}

// Retrieves a BMCCredItem for the given mac address
// Arguments:
// - mac (string): The mac address to retrieve the object for
// Returns:
// - BMCCredItem: the object corresponding to the mac address
//   or nil if no object was found.
// - error: Any error that occurred (or nil).  Note that "not found" is not
//   considered an error
func (ccs *RedsCredStore) FindMacCredentials(mac string) (redsCred storage.BMCCredItem, err error) {
	err = ccs.SS.Lookup(ccs.CCPath+"/"+mac, &redsCred)
	// if err != nil {
	// 	if storage.KeyNotFound.MatchString(err.Error()) || storage.KeyNotFound2.MatchString(err.Error()) {
	// 		// We just haven't seen this before, return nil
	// 		return nil, nil
	// 	}
	// 	return nil, err
	// }
	return
}

// Clears stored credentials for a mac address
// Arguments:
// - mac (string): the mac address to clear stored credentials for
// Returns:
// - err (string): any error that occurred

func (ccs *RedsCredStore) ClearMacCredentials(mac string) (err error) {
	err = ccs.SS.Delete(ccs.CCPath + "/" + mac)
	return
}

// Stores the global credentials
// Arguments:
// - creds (BMCCredentials): The credentials to store
// Returns:
// - error: Any error that occurred

func (ccs *RedsCredStore) SetGlobalCredentials(creds storage.BMCCredentials) error {

	err := ccs.SS.Store(ccs.CCPath+"/global/ipmi", creds)

	if err != nil {
		return errors.New("Unable to store global credentials: " + err.Error())
	}
	return nil
}

// Retrieves the global credentials
// Arguments:
// Returns:
// - BMCCredentials: The object containing the global credentials
//   or nil if no object was found.
// - error: Any error that occurred (or nil). Note that "not found" is not
//   considered an error
func (ccs *RedsCredStore) GetGlobalCredentials() (redsCred storage.BMCCredentials, err error) {
	err = ccs.SS.Lookup(ccs.CCPath+"/global/ipmi", &redsCred)
	// if err != nil {
	// 	if storage.KeyNotFound.MatchString(err.Error()) {
	// 		// We just haven't seen this before, return nil
	// 		return nil, nil
	// 	}
	// 	return nil, err
	// }

	return
}

// Clears the stored global credentials
// Arguments:
// Returns:
// - error: Any error that occurred
func (ccs *RedsCredStore) ClearGlobalCredentials() (err error) {
	err = ccs.SS.Delete(ccs.CCPath + "/global/ipmi")
	return
}

// GetDefaultCredentials retrieves a map of default credentials, keyed by vendor,
//  from a secure credentials store.
func (ccs *RedsCredStore) GetDefaultCredentials() (map[string]RedsCredentials, error) {
	credMapRtn := make(map[string]RedsCredentials)
	err := ccs.SS.Lookup(ccs.CCPath+"/defaults", &credMapRtn)

	return credMapRtn, err
}

// StoreDefaultCredentials stores a map of default credentials, keyed by vendor.
func (ccs *RedsCredStore) StoreDefaultCredentials(credentials map[string]RedsCredentials) error {
	err := ccs.SS.Store(ccs.CCPath+"/defaults", credentials)

	if err != nil {
		return errors.New("unable to store default credentials: " + err.Error())
	}
	return nil
}

// StoreDefaultCredentials stores a map of default credentials, keyed by vendor.
func (ccs *RedsCredStore) StoreDefaultSwitchCredentials(credentials SwitchCredentials) error {
	err := ccs.SS.Store(ccs.CCPath+"/switch_defaults", credentials)

	if err != nil {
		return errors.New("unable to store default switch credentials: " + err.Error())
	}
	return nil
}

// StoreDefaultCredentials stores a map of default credentials, keyed by vendor.
func (ccs *RedsCredStore) GetDefaultSwitchCredentials() (credentials SwitchCredentials, err error) {
	err = ccs.SS.Lookup(ccs.CCPath+"/switch_defaults", &credentials)

	return
}
