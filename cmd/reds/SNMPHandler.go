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
	"bufio"
	"log"
	"os"
	"regexp"
	"time"

	"stash.us.cray.com/HMS/hms-reds/internal/mapping"
	"stash.us.cray.com/HMS/hms-reds/internal/snmp"
	"stash.us.cray.com/HMS/hms-reds/internal/snmp/common"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
)

type switchinfo struct {
	address string
	model   string
}

var snmpchan chan SNMPReport
var ignoreStrings *regexp.Regexp = regexp.MustCompile("^(Created directory:|NET-SNMP)")
var extractKeyValuePairs *regexp.Regexp = regexp.MustCompile(`(?P<key>[.0-9]+) = (?P<value>[^,]+)(,|$)`)
var extractIPAddress *regexp.Regexp = regexp.MustCompile(`^([^ ]+) , (.*)$`)
var switches = make(map[string]common.Implementation)
var scanCancelList = make(map[string]chan bool)
var snmpstorage *storage.Storage

/*
Processes the data portion of a line of SNMP input into a map from key to
value.

Input:
input - a string in the form "Numeric-SNMP-MIB-ID = value, ..."

Return:
A map from string to string.  Keys are the Numeric-SNMP-MIB-ID, values are the
associated value, represented as a string.
*/
func SplitToValues(input string) map[string]string {
	var ret map[string]string = make(map[string]string)

	//log.Printf("Input string is %s\n", input)
	matches := extractKeyValuePairs.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		ret[match[1]] = match[2]
	}

	return ret
}

/*
Processes a line of text to extract the IP address at the start of the line
and the data that makes up the remainder of the line.

Inputs:
inline: The input line.  Expected to be formatted as "IPAddress , Data"

Returns:
A pair of strings.  The first is the IP address or host name that was at the
beginning of the line of text.  The second is the text of the SNMP payload
that made up the remainder of the line.
*/
func SplitSNMPLine(inline string) (string, string) {
	// Extract IP address from start of line
	ipex := extractIPAddress.FindAllStringSubmatch(inline, -1)
	IPAddr := ipex[0][1]
	LineData := ipex[0][2]

	return IPAddr, LineData
}

/*
Manages the overall processing of each input line.  This function checks
if the input line should be ignored (junk, bad format, daemon startup output)
and ignores it if so.  If a line is not ignored, it manages extracting
information in a useful format and triggering all the right kinds of updates.

Input:
inline - the line to process.

Returns: none
*/
func HandleIncomingLine(inline string) {
	// Strings to ignore if found in the input lines.
	if ignoreStrings.MatchString(inline) {
		log.Printf("SNMP: Ignoring string on ignore list: %s\n", inline)
		return
	}
	//log.Printf("SNMP scan received: %s\n", inline)

	IPAddr, LineData := SplitSNMPLine(inline)

	//log.Printf("IP Address: %s, Data: %s\n", IPAddr, LineData)

	// Okay, nex split the line into "key"/value pairs
	Data := SplitToValues(LineData)

	log.Printf("Got INFORM from %s: %s\n", IPAddr, Data)
}

/*
Scans stdin for lines and sends them for processing on finding them

No inputs or returns.
*/
func scanIncomingSNMP(c chan string) {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		c <- scanner.Text()
	}
}

// The implementation of the callback to be called by switch objects whenever
// they detect a change in the ports on the represented switch
func switchPortChangeCallback(action common.MappingAction, triggerSwitch common.Implementation, macAddress string, port string) {
	report := SNMPReport{
		switchName: triggerSwitch.GetName(),
		macAddr:    macAddress,
		port:       port,
		eventType:  action,
	}
	if action == common.Action_Add {
		log.Printf("DEBUG: %s: added %s on %s\n", triggerSwitch.GetName(), macAddress, port)
	} else if action == common.Action_Remove {
		log.Printf("DEBUG: %s: removed %s on %s\n", triggerSwitch.GetName(), macAddress, port)
	}
	snmpchan <- report
}

// Manages running periodic scans for a switch.  Started from run_SNMP
func doSwitchPeriodicScan(whichSwitch common.Implementation, cancel chan bool) {
	log.Printf("Starting periodic scan job for %s", whichSwitch.GetName())
	ticker := time.NewTicker(time.Duration(scan_period) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Printf("Running periodic scan for %s", whichSwitch.GetName())
			whichSwitch.PeriodicScan(switchPortChangeCallback)
		case <-cancel:
			log.Printf("Stopping periodic scan job for %s", whichSwitch.GetName())
			return
		}
	}
}

// Updates what we're running when a new mapping file is uploaded.
func HandleNewMapping() {
	// Kill off any scan threads from an old mapping
	for _, cancelChan := range scanCancelList {
		go func(cancelChan chan bool) {
			cancelChan <- true
		}(cancelChan)
	}
	// Clear just the SNMP portion of the MAC state.
	for id, _ := range switches {
		macMap, err := (*snmpstorage).GetSwitchState(id)
		if err != nil {
			continue
		}
		for mac, _ := range macMap {
			macState, err := (*snmpstorage).GetMacState(mac)
			if err != nil {
				log.Printf("INFO: Error retrieving state for MAC %s: %s", mac, err)
				continue
			}
			if macState == nil {
				log.Printf("INFO: Got no state mac for stored mac %s while resetting state; clearing", mac)
				macState = new(storage.MacState)
			}
			macState.DiscoveredSNMP = false
			macState.SwitchName = ""
			macState.SwitchPort = ""
			err = (*snmpstorage).SetMacState(mac, *macState)
		}
		// Clear switch state to force SNMP re-discovery.
		(*snmpstorage).SetSwitchState(id, map[string]string{})
	}

	// Clear the switches lists
	switches = make(map[string]common.Implementation)
	scanCancelList = make(map[string]chan bool)
	switchList, err := mapping.GetSwitches()
	if err != nil {
		log.Printf("WARNING: Unable to start new scan threads: can't get new switch list: %v", err)
		return
	}

	for _, thisSwitch := range *switchList {
		// TODO(spresser) Allow specification of port number?
		theSwitch, err := snmp.GetSwitch(thisSwitch.Id, snmpstorage)
		if err != nil {
			log.Printf("WARNING: Unable to initialize switch %s due to error: %v", thisSwitch.Id, err)
		} else {
			switches[thisSwitch.Id] = theSwitch
			log.Printf("Initialized switch %s", switches[thisSwitch.Id].GetName())
			scanCancelList[thisSwitch.Id] = make(chan bool)
			go doSwitchPeriodicScan(switches[thisSwitch.Id], scanCancelList[thisSwitch.Id])
		}
	}
	log.Printf("TRACE: Switches is: %s\n", switches)
}

/*
The "main" function of this module.  It is called by main() to start this
functionality and handles overall function.  Intended to be called as
goroutine.

Inputs:
ichan: the channel to use to communicate with main()

No returns.
*/
func run_SNMP(ichan chan SNMPReport, istorage *storage.Storage) {
	snmpchan = ichan
	snmpstorage = istorage

	snmpLineChan := make(chan string)
	go scanIncomingSNMP(snmpLineChan)

	for {
		select {
		case line := <-snmpLineChan:
			HandleIncomingLine(line)
		}
	}
}
