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

package dell

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/k-sone/snmpgo"

	"stash.us.cray.com/HMS/hms-reds/internal/mapping"
	"stash.us.cray.com/HMS/hms-reds/internal/snmp/common"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
)

type DellONSwitchInfo struct {
	address string
	model   *string
	name    string

	needsInit     bool // If we need to requery this switch for SNMP parameters
	periodCounter int  // A counter for how many times we've run periodicUpdate(). We want to refresh every so often

	secLevel snmpgo.SecurityLevel
	authType snmpgo.AuthProtocol
	privType snmpgo.PrivProtocol

	snmpUser     string
	snmpAuthPass string
	snmpPrivPass string

	ifIndexPortNameMap   map[int]string
	ifIndexPortNumberMap map[int]int
	portNumberIfIndexMap map[int]int
	gsnmp                *snmpgo.SNMP
	storage              *storage.Storage

	// Stored map from string mac address to string port name
	// Used to update after periodic scans
	macMap map[string]string
}

const periodsToRefresh = 50

func (o DellONSwitchInfo) String() string {
	return fmt.Sprintf("Name: %s\n\tAddress: %s\n\tModel %s\n\tSNMP User: %s\n\t"+
		"SNMP Auth Password: <REDACTED>\n\tSNMP Auth Protocol: %s\n\t"+
		"SNMP Priv Password: <REDACTED>\n\tSNMP Priv Protocol: %s\n\t"+
		"IfIndexPortName map: %v\n\tIfIndexPortNumberMap:%v\n\tPortNumberIfIndexMap:%v",
		o.name, o.address, *(o.model), o.snmpUser, o.authType, o.privType,
		o.ifIndexPortNameMap, o.ifIndexPortNumberMap, o.portNumberIfIndexMap)
}

// The OID which has the model number of the switch
var OIDModelNumber string = "1.3.6.1.2.1.47.1.1.1.1.13.2"

// The OID which maps ifIndexes to human-readable names
var OIDifIndexPortNameMap string = "1.3.6.1.2.1.31.1.1.1.1"

// The OID which maps physical port numbers to ifIndexes
var OIDPortNumberifIndex string = "1.3.6.1.2.1.17.1.4.1.2"

// The OID for the mac address table (with VLANs - should be there on all switches)
var OIDMacAddressesWithVLAN string = "1.3.6.1.2.1.17.7.1.2.2.1.2"

// The OID for the source information for VLANs
var OIDMacAddressSourceWithVLAN string = "1.3.6.1.2.1.17.7.1.2.2.1.3"

// The OID for NON-VLAN mac address table.  Only valid if the switch is
// configured with "enable-dot1d-mibwalk" first!
var OIDMACAddressesNoVLAN = "1.3.6.1.2.1.17.4.3.1.2"

// The OID for non-VLAN learned-mac sources.  Also only valid if configured
// with "enable-dot1d-mibwalk".
var OIDMacAddressSourceNoVLAN = "1.3.6.1.2.1.17.4.3.1.3"

// OID returned if an authentication fialure occurs.
var OIDAuthFailure string = "1.3.6.1.6.3.15.1.1.5.0"

// OID for querying the full name and version identification of
//a switches software operating-system and networking software.
var OIDSysDescr string = "1.3.6.1.2.1.1.1.0"

//blacklist of known bad switch software versions
var switchSWVersionBlacklist = []string{"9.14(1.1)"}

//Uses SNMP to request the sysDescr of the switch which contains
//the full name and version identification of
//a switches software operating-system and networking software.
func getSysDescr(name string, conn *snmpgo.SNMP) (string, error) {
	result, err := snmpGet(conn, OIDSysDescr)
	if err != nil {
		log.Printf("WARNING: Failed to get sysDescr from OID %s: %s", name, err)
		return "", err
	}

	ret := result.VarBinds()[0].Variable.String()
	return ret, nil
}

//searches the sysDescr of a switch for known bad software versions
func verifySwitchSWVersion(sysDescr string, blacklist []string) error {
	for _, badVersion := range blacklist {
		if strings.Contains(sysDescr, badVersion) {
			return errors.New("unsupported switch software version: " + badVersion)
		}
	}
	return nil
}

// Diffs two mac address <-> port name tables to reveal new and removed
// addresses.
// Arguments:
// - oldTable (map[string]string) The older of two tables returned by
//   getMacPortNameTable().
// - newTable (map[string]string) The newer of two tables returned by
//   getMacPortNameTable().
// Returns:
// - newAddrs (map[string]string) A table of pairs which are new in newTable
// - delAddrs (map[string]string) A table of pairs that disappeared in the
//   new table
func diffTables(oldTable map[string]string, newTable map[string]string) (
	newAddrs map[string]string, delAddrs map[string]string) {
	// This function performs a diff by copying both input tables, then finding
	// the common elements.  If an element is common, it means it did not
	// change and it is removed from both tables.  What remains in each is
	// either new or removed.
	newRes := make(map[string]string)
	delRes := make(map[string]string)

	// Copy in the input parameter tables
	for key, val := range oldTable {
		delRes[key] = val
	}

	for key, val := range newTable {
		newRes[key] = val
	}

	// Now start going through the rewRes table
	for key, newVal := range newRes {
		if _, ok := delRes[key]; ok && newVal == delRes[key] {
			// if the key was also found in delRes _and_ the values are the
			// same
			delete(newRes, key)
			delete(delRes, key)
		}
	}

	return newRes, delRes
}

func (info *DellONSwitchInfo) HandleMessage(cb common.Callback, message map[string]string) {

}

// Run a single instance of a periodic scan for new or removed MAC addresses
func (info *DellONSwitchInfo) PeriodicScan(cb common.Callback) {
	if info.needsInit || info.periodCounter > periodsToRefresh {
		log.Printf("DEBUG: %s: Need to refetch mapping information", info.GetName())
		err := info.doSwitchStateInit()
		if err == nil {
			info.needsInit = false
			info.periodCounter = 0
		} else {
			log.Printf("WARNING: %s: Failed to rescan switch; not proceeding with updates",
				info.GetName())
			return // if there's a failure we don't actually want to try updates.
		}
	}
	info.periodCounter++

	macTable, err := info.getMacPortNameTable()
	if err != nil {
		log.Printf("WARNING: Error fetching Mac address table (pretending the table has not changed): %s", err)
		info.setupRescan() // This indicates a failure to update from the switch and likely means the
		// switch is offline or otherwise has problems.  We'll want to try rescanning it when we can
		// talk with it again.
		return
	}

	// Get list of new and removed mac addresses
	newAddr, delAddr := diffTables(info.macMap, macTable)

	// Store our new table for the next diff
	info.macMap = macTable
	// Also, store this table
	err = (*(info.storage)).SetSwitchState(info.GetName(), info.macMap)
	if err != nil {
		log.Printf("WARNING: Unable to store mac address map: %s", err.Error())
	}

	// Call the callback for each new and removed MAC address
	for key, val := range newAddr {
		cb(common.Action_Add, info, key, val)
	}
	for key, val := range delAddr {
		cb(common.Action_Remove, info, key, val)
	}
}

func (info *DellONSwitchInfo) GetName() string {
	return info.name
}

func snmpGet(conn *snmpgo.SNMP, oid string) (snmpgo.Pdu, error) {
	err := conn.Open()

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	oids, err := snmpgo.NewOids([]string{oid})
	if err != nil {
		return nil, err
	}

	result, err := conn.GetRequest(oids)

	if err != nil {
		return nil, err
	}

	if result.ErrorStatus() != snmpgo.NoError {
		return nil, errors.New(result.ErrorStatus().String())
	}

	return result, nil

}

func snmpGetBulk(conn *snmpgo.SNMP, oid string) (snmpgo.Pdu, error) {
	oids, err := snmpgo.NewOids([]string{oid})
	if err != nil {
		return nil, err
	}

	if err = conn.Open(); err != nil {
		return nil, err
	}

	defer conn.Close()

	var nonRepeaters int = 0
	var maxRepititions int = 10
	result, err := conn.GetBulkWalk(oids, nonRepeaters, maxRepititions)
	if err != nil {
		return nil, err
	}

	if result.ErrorStatus() != snmpgo.NoError {
		return nil, errors.New(result.ErrorStatus().String())
	}

	return result, nil
}

// Uses SNMP to request the model number of the switch.
// Note to future implementor: This _IS_ Dell specific, so if you're
// generalizing this, don't just assume this will work for this one too.
func getModelNumber(name string, conn *snmpgo.SNMP) (*string, error) {
	result, err := snmpGet(conn, OIDModelNumber)
	if err != nil {
		log.Printf("INFO: Failed to get model number from %s: %s", name, err)
		return nil, err
	}

	ret := result.VarBinds()[0].Variable.String()
	return &(ret), nil
}

// Use SNMP to get the map from ifIndex (an arbitrary int) to a port name
// (reflects that used in the management console). This is an initialization
// function; after init, look in ifIndexPortNameMap for the resulting map.
// Arguments:
// Returns:
func getPortNameMap(name string, conn *snmpgo.SNMP) (map[int]string, error) {
	result, err := snmpGetBulk(conn, OIDifIndexPortNameMap)
	if err != nil {
		log.Printf("WARNING: Failed to get ifIndex<->name map (%s) for %s: %s", OIDifIndexPortNameMap, name, err)
		return nil, err
	}

	ret := make(map[int]string)

	for _, res := range result.VarBinds() {
		//log.Printf("%s = %s: %s\n", res.Oid, res.Variable.Type(), res.Variable)
		oidParts := strings.Split(res.Oid.String(), ".")
		strIndex := oidParts[len(oidParts)-1]
		ifIndex, err := strconv.Atoi(strIndex)
		if err != nil {
			return nil, errors.New("Failed to convert ifIndex " + strIndex + " to integer: " + err.Error())
		}

		ret[ifIndex] = res.Variable.String()
		//log.Printf("[%d]: %s", ifIndex, ret[ifIndex])
	}
	return ret, nil
}

// Use SNMP to get the map from ifIndex (an arbitrary int) to a port name
// (reflects that used in the management console). This is an initialization
// function; after init, look in ifIndexPortNumberMap for the resulting map.
// The resulting map uses port numbers as keys, and ifIndexes as values
// Arguments:
// Returns:
func getPortNumberMap(name string, conn *snmpgo.SNMP) (map[int]int, error) {
	result, err := snmpGetBulk(conn, OIDPortNumberifIndex)
	if err != nil {
		log.Printf("WARNING: Failed to get portNumber<->ifIndex(%s) for %s: %s", OIDPortNumberifIndex, name, err)
		return nil, err
	}

	ret := make(map[int]int)

	for _, res := range result.VarBinds() {
		//log.Printf("%s = %s: %s\n", res.Oid, res.Variable.Type(), res.Variable)
		oidParts := strings.Split(res.Oid.String(), ".")
		strPortID := oidParts[len(oidParts)-1]
		portID, err := strconv.Atoi(strPortID)
		if err != nil {
			return nil, errors.New("Failed to convert PortID " + strPortID + " to integer: " + err.Error())
		}
		keyBI, err := res.Variable.BigInt()
		if err != nil {
			return nil, err
		}
		key := int(keyBI.Int64())

		ret[key] = portID
		//log.Printf("[%d]: %d", key, ret[key])
	}
	return ret, nil
}

// Fetches information to create a map from MAC address to port number
// Arguments:
// - name (string) The name of the switch being operated on
// - conn (*snmpgo.SNMP) A connection object to use in talking ot the switch
// - useVLANs (bool) Whether (true) or not (false) to handle the VLAN case as well
// Returns:
// - map[string] int - a map from the string MAC address (hex encoded, all
//   lower-case) to the port number the mac address is on.  Nil if an
//   error occurred
// - err - Any error which occurred or nil if no error occurred
func getDynamicMacs(name string, conn *snmpgo.SNMP, useVLANs bool) (map[string]int, error) {
	// This function is a bit funny; there are 2 tables we need to fetch and correlate.
	// First we get the list of sources
	var addressSource string
	if useVLANs {
		addressSource = OIDMacAddressSourceWithVLAN
	} else {
		addressSource = OIDMacAddressSourceNoVLAN
	}
	_, err := snmpGetBulk(conn, addressSource)
	if err != nil {
		log.Printf("WARNING: %s: Failed to get MAC Address table sources: %s", name, err)
		return nil, err
	}

	// Second, the list of ports they appear on
	var portSrc string
	if useVLANs {
		portSrc = OIDMacAddressesWithVLAN
	} else {
		portSrc = OIDMACAddressesNoVLAN
	}
	port, err := snmpGetBulk(conn, portSrc)
	if err != nil {
		log.Printf("WARNING: %s: Failed to get MAC Address table ports: %s", name, err)
		return nil, err
	}

	// Process the mac-> port list into a map
	macPortMap := make(map[string]int)
	for _, portEntry := range port.VarBinds() {
		portMac, err := common.MacAddressFromOID(portEntry.Oid.String())
		if err != nil {
			log.Printf("WARNING: %s: Failed to parse OID %s into a MAC address: %s",
				name, portEntry.Oid.String(), err)
			continue
		}

		portNum, err := portEntry.Variable.BigInt()
		if err != nil {
			log.Printf("WARNING: %s: Failed to turn port number %s into an integer: %s",
				name, portEntry.Variable.String(), err)
		}

		if int((*portNum).Int64()) != 0 {
			macPortMap[*portMac] = int((*portNum).Int64())
		}
	}

	// We used to verify the source was 3, but it turns out Dell OS10 v10.5.0
	// switches always give a value of 1.  Therefore this was swapped out
	// (on Oct 2, 2019) for checking the port number is non-zero.
	// - @spresser
	return macPortMap, nil
}

// Returns map[MAC address] -> port name
func (info *DellONSwitchInfo) getMacPortNameTable() (map[string]string, error) {
	// Try to get mappings without VLANs
	portMap, err := getDynamicMacs(info.GetName(), info.gsnmp, false)
	if err != nil {
		log.Printf("WARNING: %s:Getting Mac address table failed (no VLANs): %s",
			info.GetName(), err)
		return nil, err
	}
	// Now try with VLANs
	portMap2, err := getDynamicMacs(info.GetName(), info.gsnmp, true)
	if err != nil {
		log.Printf("WARNING: %s:Getting Mac address table failed (VLANs): %s",
			info.GetName(), err)
		return nil, err
	}

	// Merge the two port maps together
	for key, val := range portMap2 {
		if _, ok := portMap[key]; !ok {
			// key does not appear in portMap; let's add it
			portMap[key] = val
		}
	}

	// Convert numeric port numbers into real names.
	ret := make(map[string]string)
	for key, value := range portMap {
		ifIndex, ok := info.portNumberIfIndexMap[value]
		if !ok {
			log.Printf("WARNING: %s: Failed to map port %d to ifIndex",
				info.GetName(), value)
			info.setupRescan() // Set us up to rescan the switch; this
			// could indicate the mapping on the switch changed.
			continue
		}

		name, ok := info.ifIndexPortNameMap[ifIndex]
		if !ok {
			log.Printf("WARNING: %s: Failed to map ifIndex %d to port name.",
				info.GetName(), ifIndex)
			info.setupRescan() // Set us up to rescan the switch; this
			// could indicate the mapping on the switch changed.
		}

		ret[key] = name
	}
	return ret, nil
}

func (info *DellONSwitchInfo) setupRescan() {
	info.ifIndexPortNameMap = nil
	info.ifIndexPortNumberMap = nil
	info.portNumberIfIndexMap = nil
	info.needsInit = true
}

func (info *DellONSwitchInfo) doSwitchStateInit() error {
	var firstError error

	log.Printf("DEBUG: %s: Fetching mappings from the switch", info.GetName())

	// verify that the SW version of this switch is not on the blacklist
	sysDescr, err := getSysDescr(OIDSysDescr, info.gsnmp)
	if err != nil {
		log.Printf("WARNING: %s: Failed to fetch sysDescr", err)
		if firstError == nil {
			firstError = err
		}
	} else {
		err := verifySwitchSWVersion(sysDescr, switchSWVersionBlacklist)
		if err != nil {
			log.Printf("ERROR: %s: Found known bad switch software version in sysDescr: %s\n", err, sysDescr)
			return err
		} else {
			log.Printf("INFO: verified that switch SW version is not on the blacklist for switch %s\n", info.GetName())
		}
	}

	// Next, grab a mapping of ifIndex->name and cache that.
	log.Printf("DEBUG: %s: beginning fetch of switch port mappings.", info.GetName())
	info.ifIndexPortNameMap, err = getPortNameMap(info.GetName(), info.gsnmp)
	if err != nil {
		if firstError == nil {
			firstError = err
		}
		log.Printf("WARNING: Failed to get interface->name map for %s: %s", info.GetName(), err)
	}

	// Next, grab a mapping of ifIndex->number and cache that.
	info.ifIndexPortNumberMap, err = getPortNumberMap(info.GetName(), info.gsnmp)
	if err != nil {
		if firstError == nil {
			firstError = err
		}
		log.Printf("WARNING: Failed to get interface->portNumber map for %s: %s", info.GetName(), err)
	}

	// Populate portNumberIfindexMap by reversing keys and values in ifIndexPortNumberMap
	info.portNumberIfIndexMap = make(map[int]int)
	for key, val := range info.ifIndexPortNumberMap {
		info.portNumberIfIndexMap[val] = key
	}
	log.Printf("DEBUG: %s: Done fetching mappings", info.GetName())

	if firstError == nil {
		info.needsInit = false
	}

	return firstError
}

func (info *DellONSwitchInfo) Init(name string, storage *storage.Storage) error {
	// Start by trying to init the connection.  If we can't, error
	var err error

	storedInfo, err := mapping.GetSwitchByName(name)

	if err != nil {
		log.Printf("WARNING: Couldn't fetch information on switch %s from mapping, not monitoring switch.",
			name)
		return err
	}

	info.address = storedInfo.Address
	info.name = name
	info.storage = storage
	info.snmpUser = storedInfo.SnmpUser
	info.snmpAuthPass = storedInfo.SnmpAuthPassword
	info.snmpPrivPass = storedInfo.SnmpPrivPassword

	if storedInfo.Model != "" {
		info.model = &(storedInfo.Model)
	}

	// Check that the address ends in a port nubber (required by goSNMP)
	if !strings.Contains(info.address, ":") {
		info.address = info.address + ":161"
	}

	if strings.ToLower(storedInfo.SnmpAuthProtocol) == "none" {
		info.secLevel = snmpgo.NoAuthNoPriv
	} else if strings.ToLower(storedInfo.SnmpPrivProtocol) == "none" {
		info.secLevel = snmpgo.AuthNoPriv
	} else {
		info.secLevel = snmpgo.AuthPriv
	}

	if info.secLevel != snmpgo.NoAuthNoPriv {
		if strings.ToLower(storedInfo.SnmpAuthProtocol) == "md5" {
			info.authType = snmpgo.Md5
		} else if strings.ToLower(storedInfo.SnmpAuthProtocol) == "sha" {
			info.authType = snmpgo.Sha
		}
	}

	if info.secLevel == snmpgo.AuthPriv {
		if strings.ToLower(storedInfo.SnmpPrivProtocol) == "aes" {
			info.privType = snmpgo.Aes
		} else if strings.ToLower(storedInfo.SnmpPrivProtocol) == "des" {
			info.privType = snmpgo.Des
		}
	}

	info.gsnmp, err = snmpgo.NewSNMP(snmpgo.SNMPArguments{
		Version:       snmpgo.V3,
		Address:       info.address,
		Retries:       1,
		UserName:      info.snmpUser,
		SecurityLevel: info.secLevel,
		AuthPassword:  info.snmpAuthPass,
		AuthProtocol:  info.authType,
		PrivPassword:  info.snmpPrivPass,
		PrivProtocol:  info.privType,
	})

	if err != nil {
		return err
	}

	if info.model != nil {
		log.Printf("INFO: %s: Switch hinted as model %s", info.GetName(), *(info.model))
	} else {
		// Get model from switch via SNMP
		log.Printf("DEBUG: %s: Fetching switch model", info.GetName())
		value, err := getModelNumber(info.GetName(), info.gsnmp)
		if err != nil {
			log.Printf("WARNING: %s: Failed to fetch model", err)
			info.model = nil
		} else {
			info.model = value
		}
	}
	log.Printf("DEBUG: %s: Switch is model %v", info.GetName(), info.model)

	err = info.doSwitchStateInit()
	if err != nil {
		info.setupRescan() // We failed to set up... do it during a scan
	}

	// Set the initial mac address map from storage.
	info.macMap, err = (*info.storage).GetSwitchState(info.GetName())
	if err != nil {
		log.Printf("WARNING: Could not retrieve saved state for switch %s: %s",
			info.GetName(), err.Error())
	}

	return nil
}
