// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package storage

import "fmt"

// The base object to be stored.  This object represents the merged
// state from both the HTTP and SNMP modules.
type MacState struct {
	DiscoveredHTTP bool   // Whether the HTTP module has reported this mac
	DiscoveredSNMP bool   // Whether the SNMP module has reported this mac
	SwitchName     string // The switch this mac is connected to. Set by SNMP
	SwitchPort     string // The port this mac is connected to. Set by SNMP
	Username       string // The BMC username set. Set by HTTP
	Password       string // The BMC password. Set by HTTP
	IPAddress      string // Should this be a different type? Set by HTTP
}

// String() used to suppress Username/Password in log output
//   and tag the data for clarity.
func (state MacState) String() string {
	return fmt.Sprintf("MacState - HTTP:%t, SNMP:%t. Switch:%s[%s] IP:%s",
		state.DiscoveredHTTP,
		state.DiscoveredSNMP,
		state.SwitchName,
		state.SwitchPort,
		state.IPAddress)
}

type IPAddress struct {
	AddressType string `json:"addressType"`
	Address     string `json:"address"`
}

type BMCAddress struct {
	MACAddress  string      `json:"macAddress"`
	IPAddresses []IPAddress `json:"IPAddresses"`
}

type BMCCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SystemAddresses struct {
	Addresses []BMCAddress `json:"addresses"`
}

type BMCCredItem struct {
	BMCAddrs    *SystemAddresses
	Credentials BMCCredentials
}

// The interface to the sotrage modules.
type Storage interface {
	// Initializes the object, including connecting if required
	// Arguments:
	// - url (stirng): The URL of the data store to connect to
	// - insecure (bool): Whether or not to allow insecure connections
	// Returns:
	// - error on error, nil otherwise
	Init(url string, insecure bool) error

	// Retrieves saved state for a particular switch
	// Arguments:
	// - name (string): the name of the switch to retrieve the saved state for
	// Returns:
	// - map[string]string: A map from mac address to port name fo the mac
	//   addresses known by this switch.  Returned empty on any sort of error
	// - error: the error returned if the data couldn't be retrieved.  Nil on
	//   success
	GetSwitchState(name string) (map[string]string, error)

	// Saves state for a particular switch
	// Arguments:
	// - name (string): The name of the switch to save the state for
	// - state (map[string]string): The state to be saved.  It is assumed this
	//   is a map from mac address to port name.
	// Returns:
	// - error: any error that occurred.  Nil on success
	SetSwitchState(name string, state map[string]string) error

	// Retrieves a MacState object for a given mac address.
	// Arguments:
	// - mac (string): the mac address for which to retrieve the corresponding
	//   object
	// Returns:
	// - MacState: the retrieved MacState or nil on error
	// - error: any error that occurred (or nil)
	GetMacState(mac string) (*MacState, error)

	// Saves a MacState object.
	// Arguments:
	// - mac (string): the mac address of the object which should be saved
	// - state (MacState): the state to save
	// Returns:
	// - error: Any error that occurred (or nil)
	SetMacState(mac string, state MacState) error

	// Clears the sate for a mac address
	// Arguments:
	// - mac (string): the mac address to clear stored state for
	// Returns:
	// - error: any error that occurred (or nil)
	ClearMacState(mac string) error

	// Checks the liveness of our conenction to storage
	// Returns:
	// - bool: true if the connection is live, false otherwise
	CheckLiveness() bool
}
