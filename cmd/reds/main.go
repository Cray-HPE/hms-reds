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

/*
 * reds - River Endpoint Discovery Service
 *
 * reds is a service to manage the discovery of new redfish endpoints (ie:
 * BMCs) in river-class (ie: commodity) hardware.  It consists of a server
 * (what you're looking at) and a client (found in ./client).  The server
 * has three major components:
 * - An HTTP interface (management and client communication - HTTPHandler.go)
 * - An SNMP interface (receiving SNMP INFORM messages - SNMPHandler.go)
 * - A manager for communicating to HMS when a node is done bootstrapping
 *   (BootstrapStateManager.go)
 *
 * API Version: 1.0.0
 */

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gopkg.in/resty.v1"

	"github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-reds/internal/columbia"
	"github.com/Cray-HPE/hms-reds/internal/mapping"
	"github.com/Cray-HPE/hms-reds/internal/smdclient"
	snmp "github.com/Cray-HPE/hms-reds/internal/snmp/common"
	"github.com/Cray-HPE/hms-reds/internal/storage"
	storage_factory "github.com/Cray-HPE/hms-reds/internal/storage/factory"
)

type HTTPReportType int

const (
	HTTPREPORT_DEFAULT         HTTPReportType = 0
	HTTPREPORT_CONFIG_COMPLETE HTTPReportType = 1
	HTTPREPORT_NEW_MAPPING     HTTPReportType = 2
	HTTPREPORT_MAX             HTTPReportType = 3
)

var HTTPReportTypeString = map[HTTPReportType]string{
	HTTPREPORT_DEFAULT:         "Not Set",
	HTTPREPORT_CONFIG_COMPLETE: "Configuration Complete",
	HTTPREPORT_NEW_MAPPING:     "New Port<->Xname Map",
	HTTPREPORT_MAX:             "Invalid",
}

// Service/instance name

var serviceName string

// HTTP listen spec (ip:port)
var httpListen string

// URL to communicate with HSM
var hsm string

// URL to communicate with SMNetManager

var smnetURL string

// Retry count for ReST call
const restRetry = 3

// Timeout for ReST calls in seconds
const restTimeout = 30

// A custom client for ReST calls to use instead of resty.DefaultClient
//   This will be created in the init() method
var rClient *resty.Client

// URL to communicate with BSS
var bss string

// URL to communicate with SLS
var sls string

// Base URL for conencting to our backing datastore
var datastore_base string

//
var insecure bool

// Inter-module channel -- used to communicate between modules
var snmpichan chan SNMPReport
var httpichan chan HTTPReport
var scan_period int

var mainstorage storage.Storage

var debugLevel int = 0

// A report from the SNMP module about a mac address being added or removed
type SNMPReport struct {
	// The switch this event is associated with (by name!)
	switchName string

	// The MAC address this report is about.  Hex-encoded, all lower case,
	// no seperators
	macAddr string

	// The event (add/remove)
	eventType snmp.MappingAction

	// The port (switch naming convention)
	port string
}

// A report from the HTTP module that ...
type HTTPReport struct {
	// Type of report it is (configComplete, etc)
	reportType HTTPReportType

	// The addresses for the BMC this report is about.  The MAC addresses are
	// hex-encoded, all lower case, no seperators.
	bmcAddrs []storage.BMCAddress

	// The confirmed good credentials for the BMC
	username string
	password string
}

func init() {
	rClient = resty.New().
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		SetTimeout(time.Duration(restTimeout * time.Second)).
		SetRetryCount(restRetry). // This uses a default backoff algorithm
		SetRESTMode()             // This enables automatic unmarshalling to JSON and no redirects
}

func isReadyForHSMAdd(state storage.MacState) bool {
	return (state.DiscoveredHTTP && state.DiscoveredSNMP)
}

func initMacState() *storage.MacState {
	state := new(storage.MacState)
	state.DiscoveredHTTP = false
	state.DiscoveredSNMP = false
	state.SwitchName = ""
	state.SwitchPort = ""
	state.Username = ""
	state.Password = ""
	state.IPAddress = ""
	return state
}

// Handles recieiving an SNMPReport with eventType == Action_Add
func handleSNMPAddAction(in SNMPReport) {
	_, err := mapping.GetSwitchPortByIFName(in.switchName, in.port)
	if err != nil {
		log.Printf("Unable to retrieve switch %s port %s.  Ignoring SNMPReport: %s", in.switchName, in.port, err)
		return
	}

	state, err := mainstorage.GetMacState(in.macAddr)
	if err != nil {
		log.Printf("WARNING: Unable to retrieve state for %s: %s", in.macAddr,
			err.Error())
	}

	if state == nil {
		state = initMacState()
	}

	state.DiscoveredSNMP = true
	state.SwitchName = in.switchName
	state.SwitchPort = in.port

	err = mainstorage.SetMacState(in.macAddr, *state)
	if err != nil {
		log.Printf("WARNING: Unable to store state for %s: %s", in.macAddr,
			err.Error())
	}

	if isReadyForHSMAdd(*state) {
		xname, err := mapping.SwitchPortToXname(state.SwitchName, state.SwitchPort)
		if err != nil {
			log.Printf("WARNING: Could not determine xname for node at %s on %s: %v",
				state.SwitchPort, state.SwitchName, err)
			return
		}

		// DO this as a goroutine so it can do retries
		go smdclient.NotifyHSMDiscovered(in.macAddr, xname, *state)

		// Notify DNS/DHCP services, but only if smnet URL is set
		if smnetURL != "" {
			xname, err := mapping.SwitchPortToXname(state.SwitchName, state.SwitchPort)
			if err != nil {
				log.Println("ERROR mapping switch port to XName:", err,
					"Can't notify DNS/DHCP.")
			} else {
				if debugLevel > 0 {
					log.Printf("DEBUG: Notify DNS/DHCP for '%s' ip:mac '%s:%s'\n",
						*xname, state.IPAddress, in.macAddr)
				}
			}
		}

		// Clear HTTP state, leave other stuff intatct and re-store
		state.DiscoveredHTTP = false
		state.Username = ""
		state.Password = ""
		state.IPAddress = ""
		err = mainstorage.SetMacState(in.macAddr, *state)
		if err != nil {
			log.Printf("WARNING: Unable to clear state for %s: %s",
				in.macAddr, err.Error())
		}
	} else {
		log.Printf("TRACE: %s is not yet ready to report to HSM (No report from on-node image via HTTP yet)", in.macAddr)
	}
}

func getXNameMacFromHSM(xname *string) (string, error) {
	log.Printf("DEBUG: GET %s", hsm+"/Inventory/RedfishEndpoints/"+*xname)
	data := make(map[string]interface{})

	resp, err := rClient.
		R().
		SetResult(&data).
		SetHeader(base.USERAGENT, serviceName).
		Get(hsm + "/Inventory/RedfishEndpoints/" + *xname)
	if err != nil {
		log.Printf("WARNING: Unable to send information for %s: %v", *xname, err)
	}
	// TODO put error logic in switch stmt
	if resp.StatusCode() == 200 || resp.StatusCode() == 201 {
		if err != nil {
			return "", err
		}
		mac, ok := data["MACAddr"].(string)
		if !ok {
			return "", errors.New("MAC Address missing from smd response when retrieving " + *xname)
		}
		log.Printf("DEBUG: Returning mac address %s for %s", mac, *xname)
		return mac, nil
	} else if resp.StatusCode() == 404 {
		// XName isn't known, return empty mac address
		// (Not really an error, maybe INFO log is appropriate?)
		log.Printf("INFO: Couldn't get a MAC address for %s: XName not listed in smd", *xname)
		return "", nil
	} else {
		log.Printf("WARNING: An error occurred fetching information on %s: %s %v", *xname,
			resp.Status(), resp)
		//TODO why this return?
		return "", err
	}
}

func handleSNMPRemoveAction(in SNMPReport) {
	// This has to look up what node it is, so we can tell HSM.
	// Zero, wipe out any stored state for this MAC
	err := mainstorage.ClearMacState(in.macAddr)
	if err != nil {
		log.Printf("WARNING: Failed to clear state for MAC address %s after it left the network: %v",
			in.macAddr, err)
		// Do NOT return; we want to continue until we can't
	}

	// First, see what xname this gets.
	xname, err := mapping.SwitchPortToXname(in.switchName, in.port)
	if err != nil {
		log.Printf("WARNING: Could not determine xname for node at %s on %s: %v",
			in.port, in.switchName, err)
		return // Can't procede if we don't know what xname this might correspond to
	}

	// Look up this xname to see what MAC address we have recorded for it.
	xname_mac, err := getXNameMacFromHSM(xname)
	if err != nil {
		log.Printf("WARNING: Could not fetch RedfishEndpoint for xname %s while verifying node disappearance: %v",
			*xname, err)
		return // Can't get state info from HSM, can't continue
	}

	if in.macAddr == xname_mac {
		log.Printf("INFO: xname %s has disappeared from the network, but REDS no longer marks redfishEndpoints as disabled. This message is purely for your information; REDS is operating as expected.", *xname)
	} else {
		log.Printf("INFO: Not marking xname %s as gone from the network.  Disappeared MAC address (%s) does not match stored MAC address for %s (%s).",
			*xname, in.macAddr, *xname, xname_mac)
	}
}

func handleHTTPDiscovered(in HTTPReport) {
	for _, addr := range in.bmcAddrs {
		state, err := mainstorage.GetMacState(addr.MACAddress)
		if err != nil {
			log.Printf("WARNING: Unable to retrieve state for %s: %s", addr.MACAddress,
				err.Error())
			// But we'd like to conitnue anyway, as best as we can
			state = new(storage.MacState)
		}

		if state == nil {
			state = initMacState()
		}

		ipAddr := ""
		if len(addr.IPAddresses) > 0 {
			ipAddr = addr.IPAddresses[0].Address
		}
		// Add parameters from HTTP report to the state obejct
		state.DiscoveredHTTP = true
		state.Username = in.username
		state.Password = in.password
		state.IPAddress = ipAddr

		err = mainstorage.SetMacState(addr.MACAddress, *state)
		if err != nil {
			log.Printf("WARNING: Unable to store state for %s: %s", addr.MACAddress,
				err.Error())
		}

		if isReadyForHSMAdd(*state) {
			log.Printf("Reporting ready for %v (%v)", addr.MACAddress, state)

			xname, err := mapping.SwitchPortToXname(state.SwitchName, state.SwitchPort)
			if err != nil {
				log.Printf("WARNING: Could not determine xname for node at %s on %s: %v",
					state.SwitchPort, state.SwitchName, err)
				return
			}

			// DO this as a goroutine so it can do retries
			go smdclient.NotifyHSMDiscovered(addr.MACAddress, xname, *state)

			// Notify DNS/DHCP services, but only if smnet URL is set
			if smnetURL != "" {
				xname, err := mapping.SwitchPortToXname(state.SwitchName, state.SwitchPort)
				if err != nil {
					log.Println("ERROR mapping switch port to XName:", err,
						"Can't notify DNS/DHCP.")
				} else {
					if debugLevel > 0 {
						log.Printf("DEBUG: Notify DNS/DHCP for '%s' ip:mac '%s:%s'\n",
							*xname, state.IPAddress, addr.MACAddress)
					}
				}
			}

			// Clear HTTP state, leave other stuff intatct and re-store
			state.DiscoveredHTTP = false
			state.Username = ""
			state.Password = ""
			state.IPAddress = ""
			err = mainstorage.SetMacState(addr.MACAddress, *state)
			if err != nil {
				log.Printf("WARNING: Unable to clear state for %s: %s",
					addr.MACAddress, err.Error())
			}
		} else {
			log.Printf("TRACE: %s is not yet ready to report to HSM (Not found on a switch via SNMP yet. Consider checking that SNMP discovery is working correctly)", addr.MACAddress)
		}
	}
}

// Do the dirty work of setting a parameter from an env var.

func __setenv_int(envval string, minval int, varp *int) {
	envstr := os.Getenv(envval)
	if envstr != "" {
		ival, err := strconv.Atoi(envstr)
		if err != nil {
			log.Println("ERROR converting env var", envval, ":", err,
				"-- setting unchanged.")
			return
		}
		*varp = ival
		if *varp < minval {
			*varp = minval
		}
	}
}

// Set parameters from env vars.

func getEnvVars() {
	__setenv_int("REDS_DEBUG", 0, &debugLevel)
}

func main() {
	log.Print("Starting reds")

	var enableSLSMapping bool
	var enableColumbia bool
	var defSSHKey string
	var syslogTarg, ntpTarg string
	var redfishNPSuffix string
	var err error

	// First thing's first: Parse input options.
	flag.StringVar(&httpListen, "http-listen", ":8269", "HTTP server IP/port bind target")
	flag.StringVar(&hsm, "hsm", "http://cray-smd/hsm/v1", "Hardware State Manager location as URI, e.g. [scheme]://[host[:port]][/path]")
	flag.StringVar(&bss, "bss", "http://cray-bss/boot/v1", "Boot Script service location as URI, e.g. [scheme]://[host[:port]][/path]")
	flag.StringVar(&sls, "sls", "cray-sls/v1", "System Layout Service location as [host[:port]][/path]")
	flag.StringVar(&datastore_base, "datastore", "http://cray-reds-etcd-client:2379", "Datastore Service location as URI")
	flag.IntVar(&scan_period, "scan_period", 60, "How frequently each switch should be rescanned for new and removed hardware (seconds).")
	flag.BoolVar(&insecure, "insecure", false, "If set, allow insecure connections to Hardware State Manager and Boot Script Service.")
	flag.StringVar(&syslogTarg, "syslog", "", "Server:Port of the syslog aggregator to set on Columbia switches")
	flag.StringVar(&ntpTarg, "ntp", "", "Server:Port of the NTP service to set on Columbia switches")
	flag.StringVar(&redfishNPSuffix, "np-rf-url", "/redfish/v1/Managers/BMC/NetworkProtocol", "URL path for network options Redfish endpoint (Columbia switches only)")
	flag.Parse()

	getEnvVars()
	serviceName, err = base.GetServiceInstanceName()
	if err != nil {
		log.Printf("WARNING: can't get service/instance name, using 'REDS'.")
		serviceName = "REDS"
	}

	log.Printf("Configuration: instance name: %s", serviceName)
	log.Printf("Configuration: http-listen: %s", httpListen)
	log.Printf("Configuration: hsm: %s", hsm)
	log.Printf("Configuration: smnet: %s", smnetURL)
	log.Printf("Configuration: datastore URL: %s", datastore_base)
	log.Printf("Configuration: Syslog target: %s", syslogTarg)
	log.Printf("Configuration: NTP Target: %s", ntpTarg)
	log.Printf("Configuration: SLS Mapping enabled: %t", enableSLSMapping)
	log.Printf("Configuration: Columbia discovery enabled: %t", enableColumbia)
	log.Printf("Configuration: Columbia config target: %s", redfishNPSuffix)
	log.Print("Started reds")

	log.Printf("DEBUG: Connecting to storage")
	mainstorage, err = storage_factory.MakeStorage("etcd", datastore_base, insecure)
	if err != nil {
		log.Printf("FATAL: Can't connect to ETCD backing storage!")
		panic(err)
	}
	log.Printf("DEBUG: Connected to storage")

	//Init the secure TLS stuff

	hms_certs.InitInstance(nil, serviceName)

	// Initialize our HSM interface
	err = smdclient.Init(restRetry, restTimeout, hsm, bss, serviceName)
	if err != nil {
		panic(err)
	}

	// Actually create the channel for inter-module communication
	// TODO: Need to define a spec for this internal interface
	snmpichan = make(chan SNMPReport)
	httpichan = make(chan HTTPReport)

	mapping.ConfigureSLSMode(sls, nil, nil, nil, serviceName)

	switchQuitChan := make(chan bool)
	go mapping.WatchSLSNewSwitches(switchQuitChan)

	nodeQuitChan := make(chan bool)
	go mapping.WatchSLSNewManagementNodes(nodeQuitChan)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c

		switchQuitChan <- true
		nodeQuitChan <- true
	}()

	// Set up SNMP to get calls when the mapping updates
	mapping.OnNewMapping(HandleNewMapping)

	go run_SNMP(snmpichan, &mainstorage)
	go run_HTTPsrv(httpichan, mainstorage)

	go columbia.StartColumbia(sls, hsm, syslogTarg, ntpTarg, defSSHKey, redfishNPSuffix, serviceName)

	// Load up the stored mapping file (if any) and send to SNMP
	mapping.SetStorage(mainstorage, nil)

	for {
		// Watch for incoming messages from HTTP or SNMP
		select {
		case snmpReportIn := <-snmpichan:
			// Do something
			log.Printf("Main received SNMPReport %s on %v/%v: %d", snmpReportIn.switchName, snmpReportIn.macAddr, snmpReportIn.port, snmpReportIn.eventType)
			if snmpReportIn.eventType == snmp.Action_Add {
				handleSNMPAddAction(snmpReportIn)
			} else if snmpReportIn.eventType == snmp.Action_Remove {
				handleSNMPRemoveAction(snmpReportIn)
			}
		case httpReportIn := <-httpichan:
			// Do something (else?)
			if httpReportIn.reportType == HTTPREPORT_CONFIG_COMPLETE {
				log.Printf("Main received '%s' HTTPReport with credentials for %v", HTTPReportTypeString[httpReportIn.reportType], httpReportIn.bmcAddrs)
				handleHTTPDiscovered(httpReportIn)
			}
		}
	}
}
