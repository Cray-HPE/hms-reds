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
	"flag"
	base "github.com/Cray-HPE/hms-base"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Cray-HPE/hms-certs/pkg/hms_certs"
	"github.com/Cray-HPE/hms-reds/internal/mapping"
	"github.com/Cray-HPE/hms-reds/internal/smdclient"
)

// Service/instance name

var serviceName string

// HTTP listen spec (ip:port)
var httpListen string

// URL to communicate with HSM
var hsm string

// Retry count for ReST call
const restRetry = 3

// Timeout for ReST calls in seconds
const restTimeout = 30

// URL to communicate with SLS
var sls string

var insecure bool

func main() {
	log.Print("Starting reds")

	var err error

	// First thing's first: Parse input options.
	flag.StringVar(&httpListen, "http-listen", ":8269", "HTTP server IP/port bind target")
	flag.StringVar(&hsm, "hsm", "http://cray-smd/hsm/v2", "Hardware State Manager location as URI, e.g. [scheme]://[host[:port]][/path]")
	flag.StringVar(&sls, "sls", "cray-sls/v1", "System Layout Service location as [host[:port]][/path]")
	flag.BoolVar(&insecure, "insecure", false, "If set, allow insecure connections to Hardware State Manager.")
	flag.Parse()

	serviceName, err = base.GetServiceInstanceName()
	if err != nil {
		log.Printf("WARNING: can't get service/instance name, using 'REDS'.")
		serviceName = "REDS"
	}

	log.Printf("Configuration: instance name: %s", serviceName)
	log.Printf("Configuration: http-listen: %s", httpListen)
	log.Printf("Configuration: hsm: %s", hsm)
	log.Print("Started reds")

	//Init the secure TLS stuff

	hms_certs.InitInstance(nil, serviceName)

	// Initialize our HSM interface
	err = smdclient.Init(restRetry, restTimeout, hsm, serviceName)
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

	// Load up the stored mapping file (if any) and send to SNMP
	mapping.SetStorage(nil)

	run_HTTPsrv()
}
