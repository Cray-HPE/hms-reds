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

package smdclient

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	base "github.com/Cray-HPE/hms-base"
	compcreds "github.com/Cray-HPE/hms-compcredentials"
	sstorage "github.com/Cray-HPE/hms-securestorage"
	"gopkg.in/resty.v1"
)

// HSMNotification is used to send newly discovered devices to HSM
//   via a REST call. The User and Password fields are deprecated in
//   favor of sending credentials to the HSM Vault store.
type HSMNotification struct {
	ID                 string `json:"ID"`
	FQDN               string `json:"FQDN"`
	IPAddress          string `json:"IPAddress"`
	User               string `json:"User"`
	Password           string `json:"Password"`
	MACAddr            string `json:"MACAddr"`
	RediscoverOnUpdate bool   `json:"RediscoverOnUpdate"`
	Enabled            *bool  `json:"Enabled,omitempty"` //need to set a default
}

// HSMStateNotification is used to create component entries directly in HSM
//   under /State/Components to bypass the HSM discovery process.
type HSMCompNotification struct {
	Components []base.Component `json:"Components"`
	Force      bool             `json:"Force,omitempty"`
}

// Have we initialized this module?
var doneInit = false

// A custom client for ReST calls to use instead of resty.DefaultClient
//   This will be created in the init() method
var rClient *resty.Client

// The URL to use to talk to HSM
var hsm string

// The HSM Credentials store
var hcs *compcreds.CompCredStore

// The instance name of this running service instance
var serviceName string

// Custom String function to prevent passwords from being printed directly (accidentally) to output.
func (n HSMNotification) String() string {
	// Make pretty strings for these ones to print out
	rou := "FALSE"
	if n.RediscoverOnUpdate {
		rou = "TRUE"
	}
	en := "NIL"
	if n.Enabled != nil {
		if *n.Enabled {
			en = "TRUE"
		} else {
			en = "FALSE"
		}
	}
	return fmt.Sprintf("ID: %s, FQDN: %s, IPAddress: %s, User: %s, Password: <REDACTED>, "+
		"MACAddr: %s, RediscoverOnUpdate: %s, Enabled: %s",
		n.ID, n.FQDN, n.IPAddress, n.User, n.MACAddr, rou, en)
}

// Init initializes constants and state for this module.
func Init(restRetry int, restTimeout int, hsmURL string, svcName string) error {
	serviceName = svcName
	// Setup connection to HSM Vault
	log.Printf("Connecting to HSM secure store (Vault)...")
	// Start a connection to Vault
	if ss, err := sstorage.NewVaultAdapter(""); err != nil {
		log.Printf("Error: HSM Secure Store connection failed - %s", err)
	} else {
		log.Printf("Connection to HSM secure store (Vault) succeeded")
		hcs = compcreds.NewCompCredStore("secret/hms-creds", ss)
	}

	rClient = resty.New().
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		SetTimeout(time.Duration(time.Duration(restTimeout) * time.Second)).
		SetRetryCount(restRetry). // This uses a default backoff algorithm
		SetRESTMode()             // This enables automatic unmarshalling to JSON and no redirects

	hsm = hsmURL

	return nil
}

// NotifyHSMDiscoveredWithGeolocation performs the task of adding discovered items
// to HSM once they've been geolocated and put in an HSMNotification struct
func NotifyHSMDiscoveredWithGeolocation(payload HSMNotification) bool {
	log.Printf("INFO: Notifying HSM we discovered %s:\n\t"+
		"BMC IP %s\n\tBMC MAC: %s\n\tBMC Username: %s\n\tBMC Password: ***",
		payload.ID, payload.IPAddress, payload.MACAddr, payload.User)
	_, err := json.Marshal(payload)
	if err != nil {
		log.Printf("WARNING: Could not encode JSON for %s: %v (%s)", payload.ID,
			err, payload.String())
	}

	log.Printf("DEBUG: POST to %s with %s", hsm+"/Inventory/RedfishEndpoints",
		payload.String())

	resp, err := rClient.
		R().
		SetBody(payload).
		SetHeader(base.USERAGENT, serviceName).
		Post(hsm + "/Inventory/RedfishEndpoints")
	if err != nil {
		log.Printf("WARNING: Unable to send information for %s: %v", payload.ID, err)
		log.Printf("WARNING: Errors occured and %s was not added to HSM.", payload.ID)
		return false
	}

	if resp.StatusCode() == http.StatusCreated {
		log.Printf("INFO: Successfully added %s to HSM", payload.ID)
		return true
	} else if resp.StatusCode() == http.StatusConflict {
		log.Printf("INFO: %s alredy present; patching instead", payload.ID)
		enableResult, _ := SetHSMXnameEnabled(payload.ID, true)
		return enableResult
	} else {
		log.Printf("WARNING: An error occurred uploading %s: %s %v", payload.ID,
			resp.Status(), resp)
		log.Printf("WARNING: Errors occured and %s was not added to HSM.", payload.ID)
		return false
	}
	// TODO put error logic in switch stmt
}

func SetHSMXnameEnabled(xname string, enabled bool) (bool, error) {
	payload := HSMNotification{
		ID:      xname,
		Enabled: &enabled,
		// Match the 'enabled' bool so HSM will rediscover only when
		// we are setting the redfishEndpoint to 'Enabled'.
		RediscoverOnUpdate: enabled,
	}

	log.Printf("DEBUG: PATCH to %s/Inventory/RedfishEndpoints/%s", hsm, xname)

	req := rClient.R()
	req.SetHeader("Content-Type", "application/json")
	req.SetHeader(base.USERAGENT, serviceName)
	req.SetBody(payload)
	resp, err := req.Patch(hsm + "/Inventory/RedfishEndpoints/" + xname)
	if err != nil {
		log.Printf("WARNING: Unable to patch %s: %v", xname, err)
		return false, err
	}

	if resp.StatusCode() == http.StatusOK {
		log.Printf("INFO: Successfully patched %s", xname)
	} else {
		strbody := string(resp.Body())
		log.Printf("WARNING: An error occurred patching %s: %s %v", xname, resp.Status(), string(strbody))
		rerr := errors.New("Unable to patch information for " + xname + " to HSM: " + string(resp.StatusCode()) + "\n" + string(strbody))
		return false, rerr
	}
	return true, nil
}

// HSMCreateComponent performs the task of adding a discovered component
//   directly into HSM under /State/Components to bypass the HSM discovery
//   process. This is typically to add a Master node that is not being added
//   to the management network. This will never fail on conflict. Instead HSM
//   will skip changes to already existing components unless we set Force=true
//   which we're not.
func HSMCreateComponent(payload HSMCompNotification) bool {
	log.Printf("INFO: Creating a component in HSM, %s.", payload.Components[0].ID)
	_, err := json.Marshal(payload)
	if err != nil {
		log.Printf("WARNING: Could not encode JSON for %s: %v (%v)", payload.Components[0].ID,
			err, payload)
	}

	log.Printf("DEBUG: POST to %s with %v", hsm+"/State/Components",
		payload)

	resp, err := rClient.
		R().
		SetBody(payload).
		SetHeader(base.USERAGENT, serviceName).
		Post(hsm + "/State/Components")
	if err != nil {
		log.Printf("WARNING: Unable to send information for %s: %v", payload.Components[0].ID, err)
		log.Printf("WARNING: Errors occured and %s was not added to HSM.", payload.Components[0].ID)
		return false
	}

	if resp.StatusCode() == http.StatusNoContent {
		log.Printf("INFO: Successfully added %s to HSM", payload.Components[0].ID)
		return true
	} else {
		log.Printf("WARNING: An error occurred uploading %s: %s %v", payload.Components[0].ID,
			resp.Status(), resp)
		log.Printf("WARNING: Errors occured and %s was not added to HSM.", payload.Components[0].ID)
		return false
	}
}
