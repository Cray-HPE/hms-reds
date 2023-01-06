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
 * - A manager for communicating to HMS when a node is done bootstrapping
 *   (BootstrapStateManager.go)
 *
 * API Version: 1.0.0
 */

package main

import (
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	base "github.com/Cray-HPE/hms-base"
	"gopkg.in/resty.v1"

	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-reds/internal/columbia"
	"github.com/Cray-HPE/hms-reds/internal/mapping"
	"github.com/Cray-HPE/hms-reds/internal/smdclient"
	"github.com/Cray-HPE/hms-reds/internal/storage"
	storage_factory "github.com/Cray-HPE/hms-reds/internal/storage/factory"
)

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
var scan_period int

var mainstorage storage.Storage

var debugLevel int = 0

func init() {
	rClient = resty.New().
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		SetTimeout(time.Duration(restTimeout * time.Second)).
		SetRetryCount(restRetry). // This uses a default backoff algorithm
		SetRESTMode()             // This enables automatic unmarshalling to JSON and no redirects
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
	flag.StringVar(&hsm, "hsm", "http://cray-smd/hsm/v2", "Hardware State Manager location as URI, e.g. [scheme]://[host[:port]][/path]")
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

	go columbia.StartColumbia(sls, hsm, syslogTarg, ntpTarg, defSSHKey, redfishNPSuffix, serviceName)

	// Load up the stored mapping file (if any) and send to SNMP
	mapping.SetStorage(mainstorage, nil)

	run_HTTPsrv()
}
