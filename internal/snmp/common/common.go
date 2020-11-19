// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package common

import (
	"errors"
	"strconv"
	"strings"

	"stash.us.cray.com/HMS/hms-reds/internal/storage"
)

// action, switch, macaddr, portname
type Callback func(MappingAction, Implementation, string, string)

// A MappingAction is a change in the physical layout of the system that we
// care about and wish to convey somewhere.  For example, a new node being added
// is a mapping change.
type MappingAction int

// Contains the defined MappingActions as constants.  These shoudl be passed
// to the callback to tell which what changes have been made.
const (
	Action_Add MappingAction = iota
	Action_Remove
)

type Implementation interface {
	// Does core setup and initialization of everything needed to talk to the
	// switch.
	// Arguments:
	// - name (string) - a human-readable name for the the switch.  It is
	//   recommended these be unique accross a system.  The address is used
	//   if this is nil.
	// - storage (*storage.Storage) - an instance of the sotrage library.
	// Returns:
	// - error - an error object if an error occurred that will prevent the
	//   initialized switch object from working properly, otherwise nil.
	Init(string, *storage.Storage) error

	// Handle an incoming SNMP inform from this switch.  Do whatever is
	// necessary to get the mac address and port that changed, then call
	// Callback with that pair.
	// Arguments:
	// - Callback - a callback to call for any new MAC addresses located.  This
	//   function will remain valid long-term, so the implementation may start
	//   a goroutine to handle implementing the functionality (as some switches
	//   take a while to learn new mac addresses, even after they report linkup)
	// - map[string]string - a map from a string OID to a string representation
	//   of the value.  This is intended to allow the implementation to extract
	//   whatever information is necessary to handle the update.
	HandleMessage(Callback, map[string]string)

	// Perform a single instance of a periodic scan.
	// Arguments:
	// - Callback -  a callback to call for changed mac addresses.
	// Returns: none
	PeriodicScan(Callback)

	// Gets a human-usable name or other UID.  Mostly used in log messages.
	GetName() string
}

// Takes a full OID ending in a MAC address and returns the MAC address.
// Arguments:
// - OID (string) - the OID to convert.  Must have the mac address in the last
//   6 entries.
// Returns:
// - MAC (*string) - the MAC address that ended the string, or nil if an
//   error occurred.
// - error (error) - an error object.  nil if no error occurred
func MacAddressFromOID(OID string) (*string, error) {
	OIDParts := strings.Split(OID, ".")
	if len(OIDParts) < 6 {
		return nil, errors.New("OID has fewer than 6 parts; this cannot contain a MAC address.")
	}

	ret := ""
	for _, part := range OIDParts[len(OIDParts)-6:] {
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}

		if val > 255 || val < 0 {
			return nil, errors.New(part + " is >255 or <0, which is invalid in MAC addresses.")
		}

		str := strconv.FormatInt(int64(val), 16)
		if len(str) < 2 {
			str = "0" + str
		}

		ret += str
	}

	return &ret, nil
}
