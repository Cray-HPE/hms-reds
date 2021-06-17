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

package mapping

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"stash.us.cray.com/HMS/hms-reds/internal/model"
	"stash.us.cray.com/HMS/hms-reds/internal/smdclient"

	base "stash.us.cray.com/HMS/hms-base"
	compcredentials "stash.us.cray.com/HMS/hms-compcredentials"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"

	sstorage "stash.us.cray.com/HMS/hms-securestorage"
)

// Used to notify that a new mapping is available.  Ensure thread safety!
type CallbackFunc func()

var Running = true

var slsURL string
var slsTransport *http.Transport
var slsClient http.Client
var compcreds *compcredentials.CompCredStore
var redsCreds *model.RedsCredStore
var slsSleepPeriod = 30

const VaultURLPrefix = "vault://"
const SLS_SEARCH_HARDWARE_ENDPOINT = "search/hardware"

var MGMTSwitchConnectorRegex = regexp.MustCompile("^x([0-9]{1,4})c([0-7])w([0-9]+)j([1-9][0-9]*)$")

var ErrNoSuchObject = errors.New("No object found with that name")

// A singleton for the master mapping datastructure.  Uninitialized until
// somebody loads a map
var mapping *privMapping = nil
var lock sync.Mutex

// A slice of functions to call whenever a new mapping is uploaded
var cbFuncs []CallbackFunc
var cbLock sync.Mutex

// Access to permanent storage
var modStorage storage.Storage = nil

// Service/instance name
var serviceName string


/* INTERNAL data structures for storing the mapping */
// Switch port has no private stuff

type privSwitch struct {
	Switch
	Ports       []SwitchPort             `json:"ports"`
	PortsByName map[string](*SwitchPort) `json:"-"`
	PortsByID   map[int](*SwitchPort)    `json:"-"`
}

type privMapping struct {
	Mapping
	Switches       []privSwitch             `json:"switches"`
	SwitchesByName map[string](*privSwitch) `json:"-"`
}

/* External data structures */
type SwitchPort struct {
	Id     int    `json:"id"`
	IfName string `json:"ifName"`
	PeerID string `json:"peerID"`
}

type Switch struct {
	Id               string `json:"id"`
	Address          string `json:"address"`
	SnmpUser         string `json:"snmpUser"`
	SnmpAuthPassword string `json:"snmpAuthPassword"`
	SnmpAuthProtocol string `json:"snmpAuthProtocol"`
	SnmpPrivPassword string `json:"snmpPrivPassword"`
	SnmpPrivProtocol string `json:"snmpPrivProtocol"`
	Model            string `json:"model"`
}

func (s Switch) String() string {
	return fmt.Sprintf("{ Xname: %s, Model: %s, Address: %s, SNMP User: %s, "+
		"SNMP Auth Password: <REDACTED>, SNMP Auth Protocol: %s, "+
		"SNMP Priv Password: <REDACTED>, SNMP Priv Protocol: %s }",
		s.Id, s.Model, s.Address, s.SnmpUser, s.SnmpAuthProtocol, s.SnmpPrivProtocol)
}

type Mapping struct {
	Version int `json:"version"`
}

type GenericHardware struct {
	Parent             string       `json:"Parent"`
	Children           []string     `json:"Children"`
	Xname              string       `json:"Xname"`
	Type               string       `json:"Type"`
	Class              string       `json:"Class"`
	TypeString         base.HMSType `json:"TypeString"`
	ExtraPropertiesRaw interface{}  `json:"ExtraProperties"`
}

/*
ConfigreSLSMode enables querying the mapping from SLS, rather than using
a local mapping.  Calling this function changes the mode of the mapping
module and cannot be undone, except by restarting REDS.
*/
func ConfigureSLSMode(mSlsUrl string, client *http.Client, secStorage *sstorage.SecureStorage, ccreds *compcredentials.CompCredStore, svcName string) {
	slsURL = mSlsUrl
	serviceName = svcName

	if client == nil {
		// Setup http client we'll reuse for every connection to this device
		slsTransport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		slsClient = http.Client{
			Transport: slsTransport,
			Timeout:   10 * time.Second,
		}
	} else {
		// user provided client
		slsClient = *client
	}

	var err error
	var ss sstorage.SecureStorage
	if secStorage == nil {
		ss, err = sstorage.NewVaultAdapter("")
		if err != nil {
			log.Printf("Error: %v\n", err)
			panic(err)
		}
	} else {
		// User provided secure storage
		ss = *secStorage
	}

	if ccreds == nil {
		compcreds = compcredentials.NewCompCredStore("secret/hms-creds", ss)
	} else {
		compcreds = ccreds
	}

	redsCreds = model.NewRedsCredStore("secret/reds-creds", ss)
}

/*
Look for new switches appearing in SLS by periodically querying switch list and comparing
*/
func WatchSLSNewSwitches(quitChan chan bool) {
	switches := make(map[string]Switch)

	ticker := time.NewTicker(time.Duration(slsSleepPeriod) * time.Second)
	for {
		select {
		case <-quitChan:
			log.Printf("Info: Switch watcher shutting down")
			ticker.Stop()
			return
		case <-ticker.C:
			callCallbacks := false

			log.Printf("TRACE: Getting list of new switches")
			newSwitches, err := GetSwitches()
			if err != nil {
				log.Printf("WARNING: Unable to get new switch list: %s", err)
				continue
			}
			log.Printf("TRACE: Switches is %v", switches)
			if newSwitches != nil {
				log.Printf("TRACE: New switches: %v", *newSwitches)
			}
			for key, _ := range *newSwitches {
				log.Printf("TRACE: Checking if switch %s in new", key)
				if _, ok := switches[key]; !ok {
					// new switch
					log.Printf("INFO: Found new switch %s", key)
					callCallbacks = true
				}
			}

			switchesCopy := make(map[string]Switch)
			for key, val := range switches {
				switchesCopy[key] = val
			}

			for key, _ := range *newSwitches {
				delete(switchesCopy, key)
			}
			if len(switchesCopy) > 0 {
				// Anything left in the copy was removed
				log.Printf("INFO: Found removed switch %v", switchesCopy)
				callCallbacks = true
			}

			if callCallbacks {
				// save switch list
				switches = *newSwitches
				log.Printf("INFO: Switches changes, calling callbacks")
				for _, cb := range cbFuncs {
					go cb()
				}
			} else {
				log.Printf("TRACE: No switches changes, not calling callbacks")
			}
		}
	}
}

// Sets the conenction to the permenent storage.  Call once before anything
// else.  Will automatically try to reload the mapping.
func SetStorage(mstorage storage.Storage, ss sstorage.SecureStorage) {

	if ss == nil {
		for {
			log.Printf("Mapping connecting to secure storage (vault)")
			var err error
			ss, err = sstorage.NewVaultAdapter("")
			if err != nil {
				log.Printf("ERROR: Secure Store connection failed - %s", err)
				time.Sleep(5 * time.Second)
			} else {
				log.Printf("Connection to secure store (Vault) succeeded")
				break
			}
		}
	}

	compcreds = compcredentials.NewCompCredStore("secret/hms-creds", ss)
	return
}

func switchFromSLSReturn(gh GenericHardware) (*Switch, error) {
	rawInterface := gh.ExtraPropertiesRaw.(map[string]interface{})

	var ipaddr string
	var ip6addr string
	var ip4addr string
	if _, ok := rawInterface["IP6addr"]; ok {
		ip6addr = rawInterface["IP6addr"].(string)
	}
	if _, ok := rawInterface["IP4addr"]; ok {
		ip4addr = rawInterface["IP4addr"].(string)
	}
	if ip6addr != "" && strings.ToLower(ip6addr) != "dhcpv6" {
		ipaddr = ip6addr
	} else if ip4addr != "" && strings.ToLower(ip4addr) != "dhcp" {
		ipaddr = ip4addr
	} else {
		log.Printf("INFO: No IP found for %s in SLS, falling back to using DNS/hosts file",
			gh.Xname)
		ipaddr = gh.Xname
	}

	var snmpuser string
	var snmpauthpw string
	var snmpauthproto string
	var snmpprivpw string
	var snmpprivproto string
	var model string

	if _, ok := rawInterface["SNMPUsername"]; ok {
		snmpuser = rawInterface["SNMPUsername"].(string)
	}
	if _, ok := rawInterface["SNMPAuthPassword"]; ok {
		snmpauthpw = rawInterface["SNMPAuthPassword"].(string)
	}
	if _, ok := rawInterface["SNMPAuthProtocol"]; ok {
		snmpauthproto = rawInterface["SNMPAuthProtocol"].(string)
	}
	if _, ok := rawInterface["SNMPPrivPassword"]; ok {
		snmpprivpw = rawInterface["SNMPPrivPassword"].(string)
	}
	if _, ok := rawInterface["SNMPPrivProtocol"]; ok {
		snmpprivproto = rawInterface["SNMPPrivProtocol"].(string)
	}
	if _, ok := rawInterface["Model"]; ok {
		model = rawInterface["Model"].(string)
	}

	tmpSwitch := Switch{
		Id:               gh.Xname,
		Address:          ipaddr,
		SnmpUser:         snmpuser,
		SnmpAuthPassword: snmpauthpw,
		SnmpAuthProtocol: snmpauthproto,
		SnmpPrivPassword: snmpprivpw,
		SnmpPrivProtocol: snmpprivproto,
		Model:            model,
	}

	// Examine passwords and determine if these are vault URLS.  Fetch real passwords if so
	snmpCred, err := compcreds.GetCompCred(gh.Xname)
	if err != nil {
		log.Printf("WARNING: Unable to retrieve key %s from vault: %s", gh.Xname, err)
		return nil, err
	}

	// If we get nothing back from Vault then we need to push something in.
	if snmpCred.SNMPAuthPass == "" || snmpCred.SNMPPrivPass == "" || snmpCred.Username == "" {
		defaultsCredentails, err := redsCreds.GetDefaultSwitchCredentials()
		if err != nil {
			log.Printf("ERROR: Unable to get default switch credentials: %s", err)
		} else {
			snmpCred.Xname = gh.Xname

			// For each of these if we're provided the value we'll trust that's what we should use,
			// but if not use the default vaule.
			if snmpauthpw != "" && !strings.HasPrefix(tmpSwitch.SnmpAuthPassword, VaultURLPrefix) {
				snmpCred.SNMPAuthPass = snmpauthpw
			} else {
				snmpCred.SNMPAuthPass = defaultsCredentails.SNMPAuthPassword
			}

			if snmpprivpw != "" && !strings.HasPrefix(tmpSwitch.SnmpPrivPassword, VaultURLPrefix) {
				snmpCred.SNMPPrivPass = snmpprivpw
			} else {
				snmpCred.SNMPPrivPass = defaultsCredentails.SNMPPrivPassword
			}

			if snmpuser != "" {
				snmpCred.Username = snmpuser
			} else {
				snmpCred.Username = defaultsCredentails.SNMPUsername
				tmpSwitch.SnmpUser = snmpCred.Username
			}

			err = compcreds.StoreCompCred(snmpCred)
			if err != nil {
				log.Printf("ERROR: Unable to store credentials for switch: %s", err)
			} else {
				log.Printf("INFO: Stored credential for %s", snmpCred.Xname)
			}
		}
	}

	if strings.HasPrefix(tmpSwitch.SnmpAuthPassword, VaultURLPrefix) {
		tmpSwitch.SnmpAuthPassword = snmpCred.SNMPAuthPass
	}
	if strings.HasPrefix(tmpSwitch.SnmpPrivPassword, VaultURLPrefix) {
		tmpSwitch.SnmpPrivPassword = snmpCred.SNMPPrivPass
	}

	return &tmpSwitch, nil
}

func GetSwitches() (*(map[string](Switch)), error) {

	log.Printf("TRACE: GET from http://" + slsURL + "/" + SLS_SEARCH_HARDWARE_ENDPOINT + "?type=comptype_mgmt_switch&class=River")
	url := "http://" + slsURL + "/" + SLS_SEARCH_HARDWARE_ENDPOINT + "?type=comptype_mgmt_switch&class=River"
	req,qerr := http.NewRequest("GET",url,nil)
	if (qerr != nil) {
		log.Printf("WARNING: Can't create new HTTP request: %v",qerr)
		return nil,qerr
	}
	base.SetHTTPUserAgent(req,serviceName)
	resp, err := slsClient.Do(req)

	if err != nil {
		log.Printf("WARNING: Cannot retrieve switch list: %s", err)
		return nil, err
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, errors.New("SLS returned " + resp.Status)
	}

	var retGH []GenericHardware
	// Okay, got body ok
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, err
	}

	ret := make(map[string]Switch)

	for _, gh := range retGH {
		tmpSwitch, err := switchFromSLSReturn(gh)

		if err != nil {
			log.Printf("WARNING: Error unpacking switch object: %s", err)
			return nil, err
		}

		ret[gh.Xname] = *tmpSwitch
	}

	return &ret, nil
}

func GetSwitchByName(switchName string) (*Switch, error) {
	log.Printf("TRACE: GET from http://" + slsURL + "/hardware/" + switchName)
	url := "http://" + slsURL + "/hardware/" + switchName
	req,qerr := http.NewRequest("GET",url,nil)
	if (qerr != nil) {
		log.Printf("WARNING: Can't create new HTTP request: %v",qerr)
		return nil,qerr
	}
	base.SetHTTPUserAgent(req,serviceName)

	resp, err := slsClient.Do(req)

	if err != nil {
		log.Printf("WARNING: Cannot retrieve switch %s: %s", switchName, err)
		return nil, err
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, errors.New("SLS returned " + resp.Status)
	}

	var retGH GenericHardware
	// Okay, got body ok
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, err
	}
	return switchFromSLSReturn(retGH)
}

func GetSwitchPorts(switchName string) (*([](SwitchPort)), error) {
	log.Printf("TRACE: GET from http://" + slsURL + "/" + SLS_SEARCH_HARDWARE_ENDPOINT + "?parent=" + switchName + "&type=comptype_mgmt_switch_connector")
	url := "http://" + slsURL + "/" + SLS_SEARCH_HARDWARE_ENDPOINT + "?parent=" + switchName + "&type=comptype_mgmt_switch_connector"
	req,qerr := http.NewRequest("GET",url,nil)
	if (qerr != nil) {
		log.Printf("WARNING: Can't create new HTTP request: %v",qerr)
		return nil,qerr
	}
	base.SetHTTPUserAgent(req,serviceName)
	resp, err := slsClient.Do(req)

	if err != nil {
		log.Printf("WARNING: Cannot retrieve switch port for %s: %s", switchName, err)
		return nil, err
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, errors.New("SLS returned " + resp.Status)
	}

	var retGH []GenericHardware
	// Okay, got body ok
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, err
	}

	// Need to turn this list of children into something useful...
	ret := new([](SwitchPort))
	for i, child := range retGH {
		log.Printf("ExtraProperties are: %v", child.ExtraPropertiesRaw)
		thisPort := SwitchPort{
			Id:     i,
			IfName: child.ExtraPropertiesRaw.(map[string]interface{})["VendorName"].(string),
		}
		for _, peer := range child.ExtraPropertiesRaw.(map[string]interface{})["NodeNics"].([]interface{}) {
			tpeer := peer.(string)
			if base.GetHMSType(tpeer) == base.NodeBMC {
				thisPort.PeerID = tpeer
				break
			}
		}
		if thisPort.PeerID != "" {
			(*ret) = append(*ret, thisPort)
		}
	}

	return ret, nil
}

func GetSwitchPortByIFName(switchName string, port string) (*SwitchPort, error) {
	ports, err := GetSwitchPorts(switchName)
	if err != nil {
		return nil, err
	}

	for _, item := range *ports {
		if item.IfName == port {
			return &item, nil
		}
	}

	return nil, errors.New("No port " + port + " on switch " + switchName)
}

// A function that translates from (switch, port) to the xname of the device
// on the other end.
// Arguments:
// - switchName (string): the name of the switch to look up
// - port (string): the name of the port on the switch to look up
// Returns:
// - *string: The xname of the device or nil if an error occurred
// - error: any error that occurred during lookup (or nil)
func SwitchPortToXname(switchName string, port string) (*string, error) {
	tport, err := GetSwitchPortByIFName(switchName, port)

	if err != nil {
		return nil, err
	}
	return &(tport.PeerID), nil
}

func OnNewMapping(cb CallbackFunc) {
	log.Printf("INFO: Appending %v to mapping callbacks", cb)
	cbLock.Lock()
	defer cbLock.Unlock()
	log.Printf("DEBUG: Acquired lock to append %v to callbacks", cb)
	cbFuncs = append(cbFuncs, cb)
	log.Printf("INFO: Done appending %v to mapping callbacks", cb)
}

func GetManagementNodes() ([]GenericHardware, error) {
	url := fmt.Sprintf("http://%s/%s?type=comptype_node&class=River&extra_properties.Role=Management",
		slsURL, SLS_SEARCH_HARDWARE_ENDPOINT)
	log.Printf("TRACE: GET from %s", url)
	req,qerr := http.NewRequest("GET",url,nil)
	if (qerr != nil) {
		log.Printf("WARNING: Can't create new HTTP request: %v",qerr)
		return nil,qerr
	}
	base.SetHTTPUserAgent(req,serviceName)

	resp, err := slsClient.Do(req)

	if err != nil {
		log.Printf("WARNING: Cannot retrieve management node list: %s", err)
		return nil, err
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, errors.New("SLS returned " + resp.Status)
	}

	var retGH []GenericHardware
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, err
	}

	return retGH, nil
}

func GetConnectorsByBMC(xname string) ([]GenericHardware, error) {
	url := fmt.Sprintf("http://%s/%s?node_nics=%s",
		slsURL, SLS_SEARCH_HARDWARE_ENDPOINT, xname)
	log.Printf("TRACE: GET from %s", url)
	req,qerr := http.NewRequest("GET", url, nil)
	if (qerr != nil) {
		log.Printf("WARNING: Can't create new HTTP request: %v", qerr)
		return nil, qerr
	}
	base.SetHTTPUserAgent(req, serviceName)

	resp, err := slsClient.Do(req)

	if err != nil {
		log.Printf("WARNING: Cannot retrieve management node list: %s", err)
		return nil, err
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, errors.New("SLS returned " + resp.Status)
	}

	var retGH []GenericHardware
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, err
	}

	return retGH, nil
}

/*
Look for new management nodes appearing in SLS by periodically querying node list and comparing.
*/
func WatchSLSNewManagementNodes(quitChan chan bool) {
	// In the interest of not hammering HSM with queries to figure out what it currently knows about, just use a
	// local cache of the nodes that we've told HSM about.
	managementNodes := make(map[string]GenericHardware)

	ticker := time.NewTicker(time.Duration(slsSleepPeriod) * time.Second)
	for {
		select {
		case <-quitChan:
			log.Printf("Info: Management nodes watcher shutting down")
			ticker.Stop()
			return
		case <-ticker.C:
			log.Printf("TRACE: Getting list of new management nodes")
			newNodes, err := GetManagementNodes()
			if err != nil {
				log.Printf("WARNING: Unable to get new node list: %s", err)
				continue
			}

			for _, node := range newNodes {
				// The xname field is the node iself, we actually care about the parent which is the BMC.
				if _, ok := managementNodes[node.Parent]; ok {
					// Node already exists.
					continue
				}

				log.Printf("INFO: Found new management node %+v", node)

				conns, err := GetConnectorsByBMC(node.Parent)
				if err != nil {
					log.Printf("ERROR: Unable to get node connector info from SLS, not adding "+
						"nodes in %s for now.", node.Parent)
					continue
				}

				// First check to see if there are credentials in Vault for this xname. If there are we won't
				// re-set them in case they've been changed from the defaults.
				credentials, err := compcreds.GetCompCred(node.Parent)
				if err != nil {
					log.Printf("ERROR: Unable to check Vault for xname credentials, not adding "+
						"node %s for now.", node.Parent)
					continue
				}

				if credentials.Username == "" || credentials.Password == "" {
					defaultCreds, err := redsCreds.GetDefaultCredentials()
					if err != nil {
						log.Printf("ERROR: Unable to get defualt credentials, not adding node %s for now.",
							node.Parent)
						continue
					}

					credentials := compcredentials.CompCredentials{
						Xname:    node.Parent,
						Username: defaultCreds["Cray"].Username,
						Password: defaultCreds["Cray"].Password,
					}

					err = compcreds.StoreCompCred(credentials)
					if err != nil {
						log.Printf("ERROR: Unable to set credentials, not adding node %s for now.",
							node.Parent)
						continue
					} else {
						log.Printf("DEBUG: Set credentials for %s", node.Parent)
					}
				}

				// Add Master Management nodes to HSM under /State/Components
				// to account for cases where their BMC is not connected to
				// the cluster. These nodes will not have MgmtSwitchConnectors
				if len(conns) == 0 {
					var (
						role    string
						subrole string
						nid     json.Number
					)
					if val, ok := node.ExtraPropertiesRaw.(map[string]interface{})["Role"]; ok {
						role = base.VerifyNormalizeRole(val.(string))
					}
					if val, ok := node.ExtraPropertiesRaw.(map[string]interface{})["SubRole"]; ok {
						subrole = base.VerifyNormalizeSubRole(val.(string))
					}
					if role == base.RoleManagement.String() && subrole == base.SubRoleMaster.String() {
						if val, ok := node.ExtraPropertiesRaw.(map[string]interface{})["NID"]; ok {
							nid = json.Number(strconv.FormatFloat(val.(float64), 'f', 0, 64))
						}
						hsmCompNotification := smdclient.HSMCompNotification{
							Components: []base.Component{{
								ID:      node.Xname,
								State:   base.StatePopulated.String(),
								Role:    role,
								SubRole: subrole,
								NID:     nid,
								NetType: base.NetSling.String(),
								Arch:    base.ArchX86.String(),
								Class:   node.Class,
							}},
						}
						smdclient.HSMCreateComponent(hsmCompNotification)
					}
				}

				// Now build the HSM notification and send it.
				hsmNotification := smdclient.HSMNotification{
					ID:                 node.Parent,
					RediscoverOnUpdate: true,
				}

				added := smdclient.NotifyHSMDiscoveredWithGeolocation(hsmNotification)
				if added {
					// Now add this node to the cache map so we don't send it again.
					managementNodes[node.Parent] = node
				}
			}
		}
	}
}
