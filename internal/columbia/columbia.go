// MIT License
//
// (C) Copyright [2019-2021] Hewlett Packard Enterprise Development LP
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


package columbia

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	base "stash.us.cray.com/HMS/hms-base"
	bmc_nwprotocol "stash.us.cray.com/HMS/hms-bmc-networkprotocol/pkg"
	compcreds "stash.us.cray.com/HMS/hms-compcredentials"
	"stash.us.cray.com/HMS/hms-certs/pkg/hms_certs"
	"stash.us.cray.com/HMS/hms-reds/internal/model"
	"stash.us.cray.com/HMS/hms-reds/internal/smdclient"
	sstorage "stash.us.cray.com/HMS/hms-securestorage"
)

const VaultURLPrefix = "vault://"

var hsm string
var rfNWPStatic bmc_nwprotocol.RedfishNWProtocol

// The HSM Credentials store
var hcs *compcreds.CompCredStore
var credStorage *model.RedsCredStore

type GenericHardware struct {
	Parent             string       `json:"Parent"`
	Children           []string     `json:"Children"`
	Xname              string       `json:"Xname"`
	Type               string       `json:"Type"`
	Class              string       `json:"Class"`
	TypeString         base.HMSType `json:"TypeString"`
	ExtraPropertiesRaw interface{}  `json:"ExtraProperties"`
}

type ComptypeRtrBmc struct {
	IP6Addr  string `json:"IP6addr,omitempty"`
	IP4Addr  string `json:"IP4addr,omitempty"`
	Username string `json:"Username,omitempty"`
	Password string `json:"Password,omitempty"`
}

var client *hms_certs.HTTPClientPair
var rfClient *hms_certs.HTTPClientPair
var rfClientLock sync.RWMutex
var forceInsec = false

// Keep track of if the list of Columbia switches was successfully read
// NOTE: should only be modified by StartColumbia but may be accessed by
//  others
var columbiaListRead = false

// ColumbiaListRead - find if the list of Columbia switches has been successfully read yet
func ColumbiaListRead() bool {
	return columbiaListRead
}

// ColumbiaLIstReadMockup - used only for testing to mock up columbia read status!!!
func ColumbiaListReadMockup(r bool) {
	columbiaListRead = r
}

func queryNetworkStatusViaAddress(address string) (bool, error) {
	log.Printf("DEBUG: GET from https://%s/redfish", address)

	rfClientLock.RLock()
	resp, err := rfClient.Get("https://" + address + "/redfish/v1/")
	rfClientLock.RUnlock()

	if err != nil {
		log.Printf("TRACE: Error reaching out to %s: %v", address, err)
		return false, nil
	}

	if resp.StatusCode == 200 {
		log.Printf("TRACE: %s is present", address)
		return true, nil
	}

	// else ...
	defer resp.Body.Close()
	strbody, _ := ioutil.ReadAll(resp.Body)
	log.Printf("TRACE: Unable to reach out to %s (%d): %v", address, resp.StatusCode, string(strbody))
	return false, nil
}

func queryNetworkStatus(addrs []string) (string, error) {
	var retErr error
	for _, addr := range addrs {
		res, err := queryNetworkStatusViaAddress(addr)
		if err != nil {
			log.Printf("DEBUG: Unable to contact %s: %s", addr, err)
			retErr = err
		}
		if res == true {
			return addr, nil
		}
	}
	return "", retErr
}

func notifyXnamePresent(node GenericHardware, extras ComptypeRtrBmc, address string) (err error) {
	// Send credentials to Vault instead of HSM, but only if there are no credentials in Vault
	// Examine passwords and determine if these are vault URLS.  Fetch real passwords if so
	cCred, err := hcs.GetCompCred(node.Xname)
	if err != nil {
		log.Printf("WARNING: Unable to retrieve key %s from vault: %s", node.Xname, err)
		return err
	}

	// If we get nothing back from Vault then we need to push something in.
	if cCred.Username == "" || cCred.Password == "" {
		log.Printf("CredsStore is: %v", credStorage)
		defaultsCredentails, err := credStorage.GetDefaultCredentials()
		if err != nil {
			log.Printf("ERROR: Unable to get default Columbia switch credentials: %s", err)
		} else {
			cCred.Xname = node.Xname

			// For each of these if we're provided the value we'll trust that's what we should use,
			// but if not use the default value.
			if cCred.Username == "" {
				if extras.Username != "" && !strings.HasPrefix(extras.Username, VaultURLPrefix) {
					cCred.Username = extras.Username
				} else {
					cCred.Username = defaultsCredentails["Cray"].Username
				}
			}

			if cCred.Password == "" {
				if extras.Password != "" && !strings.HasPrefix(extras.Password, VaultURLPrefix) {
					cCred.Password = extras.Password
				} else {
					cCred.Password = defaultsCredentails["Cray"].Password
				}
			}

			err = hcs.StoreCompCred(cCred)
			if err != nil {
				log.Printf("ERROR: Unable to store credentials for switch: %s", err)
			} else {
				log.Printf("INFO: Stored credential for %s", cCred.Xname)
			}
		}
	}

	hsmError := notifyHSMXnamePresent(node, address)
	// TODO: REDS doesn't really have the concept of per-node credentials.
	// We need to do something so that Columbias can have per-item creds set
	// including SSH keys.
	globalCreds, err := credStorage.GetGlobalCredentials()
	tmpBMCCreds := bmc_nwprotocol.CopyRFNetworkProtocol(&rfNWPStatic)
	nstError := bmc_nwprotocol.SetXNameNWPInfo(tmpBMCCreds, address, globalCreds.Username, globalCreds.Password)

	if (hsmError != nil) || (nstError != nil) {
		finalError := fmt.Errorf("%v %v", hsmError, nstError)
		return finalError
	}
	return nil
}

func notifyXnameGone(node string) error {
	smdclient.NotifyHSMRemoved(node)
	return nil
}

func notifyHSMXnamePresent(node GenericHardware, address string) error {
	// No longer include User and Password (set to blank) to signal HSM to pull from Vault
	payload := smdclient.HSMNotification{
		ID:        node.Xname,
		FQDN:      node.Xname,
		IPAddress: address,
		User:      "", // blank to pull from Vault
		Password:  "", // blank to pull from Vault
		//MACAddr:            node.mac,
		RediscoverOnUpdate: true,
	}

	smdclient.NotifyHSMDiscoveredWithGeolocation(payload)

	return nil
}

// Create a custom error type to help parse what happened
type slsConnectionError struct {
	s string
}

func newSlsConnectionError(msg string) error {
	return &slsConnectionError{msg}
}

func (e *slsConnectionError) Error() string {
	return e.s
}

func getColumbiaList(slsURL string) (retGH []GenericHardware, retBMC []ComptypeRtrBmc, err error) {

	log.Printf("TRACE: GET from http://" + slsURL + "/search/hardware?type=comptype_rtr_bmc")
	resp, err := client.InsecureClient.Get("http://" + slsURL + "/search/hardware?type=comptype_rtr_bmc")

	if err != nil {
		log.Printf("WARNING: Cannot retrieve Columbia switch list: %s", err)
		return nil, nil, newSlsConnectionError(err.Error())
	}

	strbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARNING: Couldn't read response body: %s", err)
		return nil, nil, newSlsConnectionError(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("WARNING: Invalid response from SLS. Code: %d, message: %s", resp.StatusCode, strbody)
		return nil, nil, newSlsConnectionError("SLS returned " + resp.Status)
	}

	// Okay, got body ok
	err = json.Unmarshal(strbody, &retGH)
	if err != nil {
		log.Printf("WARNING: Unable to unmarshall response from SLS: %s", err)
		return nil, nil, err
	}

	for i, _ := range retGH {
		ip6addr := ""
		ip4addr := ""
		user := ""
		pass := ""
		if retGH[i].ExtraPropertiesRaw != nil {
			rawInterface := retGH[i].ExtraPropertiesRaw.(map[string]interface{})
			if _, ok := rawInterface["IP4addr"]; ok {
				ip4addr = rawInterface["IP4addr"].(string)
			}
			if _, ok := rawInterface["IP6addr"]; ok {
				ip6addr = rawInterface["IP6addr"].(string)
			}
			if _, ok := rawInterface["Username"]; ok {
				user = rawInterface["Username"].(string)
			}
			if _, ok := rawInterface["Password"]; ok {
				pass = rawInterface["Password"].(string)
			}
		}
		crb := ComptypeRtrBmc{
			IP6Addr:  ip6addr,
			IP4Addr:  ip4addr,
			Username: user,
			Password: pass,
		}
		retBMC = append(retBMC, crb)
	}

	return
}

func watchColumbia(dev GenericHardware, dbmc ComptypeRtrBmc, funcs funcMap, stopChan chan bool) {
	present, err := smdclient.QueryHSMState(dev.Xname)
	log.Printf("INFO: %s in HSM: %t", dev.Xname, present)
	addresses := make([]string, 0)
	if dbmc.IP6Addr != "" && dbmc.IP6Addr != "DHCPv6" {
		addresses = append(addresses, dbmc.IP6Addr)
	}
	if dbmc.IP4Addr != "" && dbmc.IP4Addr != "DHCPv4" {
		addresses = append(addresses, dbmc.IP4Addr)
	}
	if err != nil {
		log.Printf("WARNING: Unable to get state for %s; assuming current network state: %s", dev.Xname, err)
		addr, err := funcs.CheckUp(addresses)
		if err != nil {
			log.Printf("WARNING: Unable to check up on %s: %s", dev.Xname, err)
		}
		present = addr != "" // set present to whether or not the BMC responded on any address
	}
	if len(addresses) == 0 {
		log.Printf("WARNING: No known address for %s; not monitoring it", dev.Xname)
		return
	}

	// Set up a ticker for our checkins
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	terminate := false
	for !terminate {
		select {
		case <-stopChan:
			log.Printf("INFO: Monitoring thread for %s received stop signal; terminating", dev.Xname)
			terminate = true
			break
		case <-ticker.C:
			newAddr, err := funcs.CheckUp(addresses)
			if err != nil {
				log.Printf("WARNING: Unable to check up on %s.  Assuming no status change: %s", dev.Xname, err)
				continue
			}
			if newAddr != "" && present == false {
				// BMC has appeared!
				funcs.NotifyHere(dev, dbmc, newAddr)
				present = true
			} else if newAddr == "" && present == true {
				// BMC disappeared
				funcs.NotifyGone(dev.Xname)
				present = false
			} else {
				log.Printf("TRACE: %s has not changed state", dev.Xname)
			}
		}
	}
}

type funcMap struct {
	// Returns which address if any responded to pings
	CheckUp func([]string) (string, error)
	// Manages notifying HSM of status change to present
	NotifyHere func(GenericHardware, ComptypeRtrBmc, string) error
	// Manages notifying HSM of status change to gone
	NotifyGone func(string) error
}

// Set up TLS-secured TLS transport.  Lots of variables can modify the 
// behavior of the Vault PKI -- this is mostly for testing.  The important
// ones are:
//
// REDS_CA_URI -- specifies whether to get the CA bundle from Vault or a
//                a configmap file.
// REDS_LOG_INSECURE_FAILOVER -- Reduce log clutter during prototyping.

func setupRFTransport() error {
	var err error
	var envstr string

	locCAUri := ""

	//Get some env vars that pertain to CA/TLS

	if (forceInsec == false) {
		envstr := os.Getenv("REDS_CA_URI")
		if (envstr != "") {
			locCAUri = envstr
		}
	}
	envstr = os.Getenv("REDS_LOG_INSECURE_FAILOVER")
	if (envstr != "") {
		yn,_ := strconv.ParseBool(envstr)
		if (yn == false) {
			//defaults to true
			hms_certs.ConfigParams.LogInsecureFailover = false
		}
	}

	//These is for testing, or if we somehow change the PKI URLs.

	envstr = os.Getenv("REDS_CA_PKI_URL")
	if (envstr != "") {
		log.Printf("INFO: Using CA PKI URL: '%s'",envstr)
		hms_certs.ConfigParams.VaultCAUrl = envstr
	}
	envstr = os.Getenv("REDS_VAULT_PKI_URL")
	if (envstr != "") {
		log.Printf("INFO: Using VAULT PKI URL: '%s'",envstr)
		hms_certs.ConfigParams.VaultPKIUrl = envstr
	}
	envstr = os.Getenv("REDS_VAULT_JWT_FILE")
	if (envstr != "") {
		log.Printf("INFO: Using Vault JWT file: '%s'",envstr)
		hms_certs.ConfigParams.VaultJWTFile = envstr
	}
	envstr = os.Getenv("REDS_K8S_AUTH_URL")
	if (envstr != "") {
		log.Printf("INFO: Using K8S AUTH URL: '%s'",envstr)
		hms_certs.ConfigParams.K8SAuthUrl = envstr
	}

	//Once acquired, all RF operations are blocked.

	rfClientLock.Lock()
	defer rfClientLock.Unlock()
	if (locCAUri != "") {
		log.Printf("INFO: Creating TLS-secured HTTP client pair for Redfish operations, CA URI: '%s'.",locCAUri)
	} else {
		log.Printf("INFO: Creating non-validated HTTP client pair for Redfish operations (no CA bundle).")
	}
	rfClient,err = hms_certs.CreateHTTPClientPair(locCAUri,10)
	if (err != nil) {
		emsg := fmt.Errorf("ERROR: can't create TLS cert-enabled HTTP transport: %v",
			err)
		return emsg
	}
	return nil
}

func caCB(caBundle string) {
	log.Printf("INFO: CA bundle changed, re-creating Redfish HTTP transport.")
	err := setupRFTransport()
	if (err != nil) {
		log.Printf("%v",err)
	} else {
		log.Printf("INFO: Redfish HTTP transports created with new CA bundle.")
	}
}

// StartColumbia - gathers list of Columbia switches and starts watching them for hardware
func StartColumbia(slsUrl string, hsmUrl string, syslogTarg string, ntpTarg string, sshKey string, redfishNPSuffix string, svcName string) {
	// NOTE: this sits and waits for sls to return a valid list of Columbia switches so
	//  make sure to run this in a separate thread or be prepared to wait forever.
	var err error
	var ss sstorage.SecureStorage
	var columbiaGoRoutines map[string]chan bool
	hsm = hsmUrl

	columbiaGoRoutines = make(map[string]chan bool)

	log.Printf("Columbia: Connecting to HSM secure store (Vault)...")
	// Start a connection to Vault
	if ss, err = sstorage.NewVaultAdapter(""); err != nil {
		log.Printf("Error: HSM Secure Store connection failed - %s", err)
	} else {
		log.Printf("Columbia: Connection to HSM secure store (Vault) succeeded")
		hcs = compcreds.NewCompCredStore("secret/hms-creds", ss)
	}

	credStorage = model.NewRedsCredStore("secret/reds-creds", ss)

	//Set up TLS-cert-enabled RF transport, insecure for SLS,HSM.

	log.Printf("INFO: Creating insecure HTTP client for non-Redfish operation.")
	hms_certs.InitInstance(nil,svcName)
	client,_ = hms_certs.CreateHTTPClientPair("",10)

	//Open TLS secured HTTP transport.  Fail over if we can't get it to work
	//securely.

	var ix int
	for ix = 1; ix <= 10; ix ++ {
		err = setupRFTransport()
		if (err == nil) {
			log.Printf("INFO: Successfully set up secure Redfish transport.")
			break
		}
		log.Printf("RF Secure Transport create attempt %d: %v", ix,err)
		time.Sleep(3 * time.Second)
	}

	if (ix > 10) {
		log.Printf("ERROR: exhausted all retries creating TLS-secured Redfish transport, failing over insecure.")
		forceInsec = true
		err = setupRFTransport()
		if (err != nil) {
			log.Printf("ERROR: can't create any RF HTTP transport!!!!!  No columbia monitoring will take place.")
			return
		}
	}

	caURI := os.Getenv("REDS_CA_URI")
	if (caURI != "") {
		log.Printf("Setting up CA bundle watcher for '%s'.",caURI)
		err = hms_certs.CAUpdateRegister(caURI,caCB)
		if (err != nil) {
			log.Printf("ERROR setting up CA bundle watcher: %v",err)
			log.Printf("    CA bundle changes will not be applied!")
		} else {
			log.Printf("CA bundle watcher running.")
		}
	}

	//Set up DNS/DHCP and NW protocol stuff

	nwp := bmc_nwprotocol.NWPData{CAChainURI: caURI, NTPSpec: ntpTarg, SyslogSpec: syslogTarg,
		SSHKey: sshKey, SSHConsoleKey: sshKey}

	rfNWPStatic, err = bmc_nwprotocol.Init(nwp, redfishNPSuffix)
	if err != nil {
		log.Println("ERROR setting up NW protocol handling:", err)
		//TODO: should we exit??
	}

	// loop forever here until we get a valid list back from sls
	maxWait := 300 * time.Second
	backoff := 5 * time.Second
	currWait := 0 * time.Second
	successWait := 30 * time.Second
	for {
		// see if sls will get us the list of columbia switches
		columbiaGeneric, columbiaRtrBmc, err := getColumbiaList(slsUrl)
		if err != nil {
			// if there is a problem that looks like a communication issue
			//  or sls not ready issue, keep trying
			if err, ok := err.(*slsConnectionError); ok {
				log.Printf("ERROR: while attempting to get Columbia switches, retrying: %s", err.Error())
				currWait += backoff
				if currWait > maxWait {
					currWait = maxWait
				}
				time.Sleep(currWait)
				continue
			}
			// not an sls connection/readiness error - bail without retry
			log.Printf("ERROR: problem retrieving list of Columbia switches!: %s", err)
			return
		}

		// mark the columbia switch list as having been read
		// NOTE- do this asap so readiness probe doesn't restart
		//  the pod just as this succeeds
		columbiaListRead = true

		// Reset the wait time, since we had a successful read
		currWait = successWait

		// found something, process and break
		// NOTE: ok to be empty, just needed the good 'get' call
		funcs := funcMap{
			CheckUp:    queryNetworkStatus,
			NotifyHere: notifyXnamePresent,
			NotifyGone: notifyXnameGone,
		}

		remainderThreads := make(map[string]chan bool)
		for i, e := range columbiaGoRoutines {
			remainderThreads[i] = e
		}

		for i := range columbiaGeneric {
			// Do some filtering -- what's new, what's old, what didn't change.
			if _, ok := columbiaGoRoutines[columbiaGeneric[i].Xname]; ok {
				// We already knew about this one.
				// Remove it from the remainder list
				delete(remainderThreads, columbiaGeneric[i].Xname)
			} else {
				// Must be something new, so start a new thread
				log.Printf("INFO: Columbia switch %s is new; starting monitoring thread", columbiaGeneric[i].Xname)

				// Create stop channel
				stopChan := make(chan bool)

				// Start the thread
				go watchColumbia(columbiaGeneric[i], columbiaRtrBmc[i], funcs, stopChan)

				// And add this to our list of running threads
				columbiaGoRoutines[columbiaGeneric[i].Xname] = stopChan
			}
		}

		// Whatever is left in our remainder list is now gone, so signal the monitoring threads to quit
		for k, v := range remainderThreads {
			log.Printf("Xname %s is no longer in SLS; terminating scan thread.", k)
			v <- true
			delete(columbiaGoRoutines, k)
		}

		// wait and restart
		time.Sleep(currWait)
	}
}
