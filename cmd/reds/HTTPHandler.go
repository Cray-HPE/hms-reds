// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	base "stash.us.cray.com/HMS/hms-base"
	"stash.us.cray.com/HMS/hms-reds/internal/columbia"
	"stash.us.cray.com/HMS/hms-reds/internal/model"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
	sstorage "stash.us.cray.com/HMS/hms-securestorage"
)

// Channel for sending notifications
var imchan chan HTTPReport
var globalStorage storage.Storage

var credStorage *model.RedsCredStore

/*
 * Search the knownCredentials list for a BMC based on MAC address. This looks for
 * any one of the specified MAC addresses to match any one of the MAC addresses
 * in a known BMC credential entry.
 */
func findBMC(adders *storage.SystemAddresses) (storage.BMCCredItem, int) {
	for i, addr1 := range adders.Addresses {
		addr, err := credStorage.FindMacCredentials(addr1.MACAddress)
		if err != nil {
			log.Printf("WARNING: unable to fetch credentials for %s: %s", addr1, err.Error())
		}
		if len(addr.Credentials.Username) != 0 {
			return addr, i
		}
	}
	empty := storage.BMCCredItem{}
	return empty, -1
}

/*
 * Adds a BMC and its newly created credentials to the knownCredentials list.
 * If the BMC already exists, just update the entry. This is likely due to a
 * node failing before issuing a "configuration complete" (which results in
 * the entry being removed) and rebooting.
 */
func addBMC(addrs *storage.SystemAddresses, resp storage.BMCCredentials) {
	creds := new(storage.BMCCredItem)
	creds.BMCAddrs = addrs
	creds.Credentials.Username = resp.Username
	creds.Credentials.Password = resp.Password

	for _, addr := range addrs.Addresses {
		credStorage.AddMacCredentials(addr.MACAddress, *creds)
	}
}

/*
 * This removes the BMC from the knownCredentials list and returns the removed
 * BMC. This is generally done when a "configuration complete" and the
 * credentials are sent to HSM.
 */
func removeBMC(addrs *storage.SystemAddresses) storage.BMCCredItem {
	ret, _ := findBMC(addrs)

	for _, addr := range addrs.Addresses {
		credStorage.ClearMacCredentials(addr.MACAddress)
	}

	return ret
}

/*
* Logs a call to an HTTP endpoint.  args are any notable argumetns we want to log here
 */
func log_call(r *http.Request, args ...string) {
	log.Printf("HTTP: %s called, args %s", r.URL.RequestURI(), args)
}

func simpleMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		if r.URL.Path != "/v1/liveness" &&
			r.URL.Path != "/v1/readiness" {
			log.Printf("HTTP: %s called", r.RequestURI)
		}
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func respond_200(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
}

func respond_204(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func respond_400(w http.ResponseWriter, url, msg string) {
	pdet := base.NewProblemDetails("about:blank",
		http.StatusText(http.StatusBadRequest),
		msg,
		url,
		http.StatusBadRequest)
	base.SendProblemDetails(w, pdet, 0)
}

func respond_401(w http.ResponseWriter, url, msg string) {
	pdet := base.NewProblemDetails("about:blank",
		http.StatusText(http.StatusUnauthorized),
		msg,
		url,
		http.StatusUnauthorized)
	base.SendProblemDetails(w, pdet, 0)
}

func respond_404(w http.ResponseWriter, url, msg string) {
	pdet := base.NewProblemDetails("about:blank",
		http.StatusText(http.StatusNotFound),
		msg,
		url,
		http.StatusNotFound)
	base.SendProblemDetails(w, pdet, 0)
}

func respond_405(w http.ResponseWriter, url, allowStr string) {
	astr := "Only " + allowStr + " operation permitted"
	pdet := base.NewProblemDetails("about:blank",
		http.StatusText(http.StatusMethodNotAllowed),
		astr,
		url,
		http.StatusMethodNotAllowed)
	w.Header().Add("Allow", allowStr)
	base.SendProblemDetails(w, pdet, 0)
}

func respond_500(w http.ResponseWriter, msg string) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	w.Write([]byte(msg + "\n"))
}

func respond_503(w http.ResponseWriter, msg string) {
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	w.Write([]byte(msg + "\n"))
}

// func respond_501(w http.ResponseWriter) {
// http.Error(w, "Not Implemented", http.StatusNotImplemented)
// w.Write([]byte("This endpoint is not yet implemented.\n"))
// }

/*
 * Responds the credential requests.
 */
func respondCredentials(w http.ResponseWriter, cred storage.BMCCredentials) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(cred)
	if err != nil {
		log.Printf("Couldn't encode a JSON command response: %s\n", err)
	}
}

func authenticate(r *http.Request) bool {
	//TODO: Authorization is not currently supported upstream. Ignore for now.
	// authHeader := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	// if len(authHeader) != 2 || authHeader[0] != "Basic" {
	// return false
	// }

	// authDecoded, _ := base64.StdEncoding.DecodeString(authHeader[1])
	// pair := strings.SplitN(string(authDecoded), ":", 2)

	// if len(pair) != 2 || pair[0] != "someUser" || pair[1] != "somePass"  {
	// return false
	// }

	return true
}

func Versions(w http.ResponseWriter, r *http.Request) {
	log_call(r)
	w.Write([]byte("/v1\n"))
}

/*
 * Validates that reds dependencies are available.
 */
func doReadinessCheck(w http.ResponseWriter, r *http.Request) {
	// error response string
	var msg = ""
	var hasProblem = false

	// check etcd
	if !globalStorage.CheckLiveness() {
		msg = "etcd for REDS is not ready"
		hasProblem = true
	}

	// check sls
	if !columbia.ColumbiaListRead() {
		cMsg := "columbia switches not read from SLS for REDS"
		if hasProblem {
			msg = msg + " : " + cMsg
		} else {
			msg = cMsg
			hasProblem = true
		}
	}

	// parse the results
	if hasProblem {
		respond_503(w, msg)
	} else {
		respond_204(w)
	}
}

/*
 * Returns success once the reds server is up
 */
func doLivenessCheck(w http.ResponseWriter, r *http.Request) {
	respond_204(w)
}

/*
 * Bad method function for /credentials
 */
func doCredentialsBadMethod(w http.ResponseWriter, r *http.Request) {
	respond_405(w, r.URL.Path, "POST")
}

/*
 * Processes POST requests to /credentials to generate BMC credentials.
 * Parses the request for MAC addresses associated with a BMC, generates
 * the credentials, and responds to the request with the new credentials.
 */
func doPostCredentials(w http.ResponseWriter, r *http.Request) {
	var resp storage.BMCCredentials
	log_call(r)

	if r.Body == nil {
		respond_400(w, r.URL.Path, "Missing request body")
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	bmcAddrs := new(storage.SystemAddresses)
	err = json.Unmarshal(body, bmcAddrs)
	if err != nil {
		respond_400(w, r.URL.Path, "Error while parsing json request")
		return
	}
	if len(bmcAddrs.Addresses) < 1 {
		respond_400(w, r.URL.Path, "Missing MAC addresses")
		return
	}

	globalCreds, err := credStorage.GetGlobalCredentials()
	defaultCreds, err2 := credStorage.GetDefaultCredentials()

	if err == nil && len(globalCreds.Username) != 0 {
		// If set, use the global credentials for all BMCs
		resp.Username = globalCreds.Username
		resp.Password = globalCreds.Password
	} else if err2 == nil && len(defaultCreds["Cray"].Username) != 0 {
		//TODO: Randomize credentials
		resp.Username = defaultCreds["Cray"].Username
		resp.Password = defaultCreds["Cray"].Password
	} else {
		respond_500(w, "No credentials available")
	}
	respondCredentials(w, resp)

	addBMC(bmcAddrs, resp)
}

/*
 * Bad method function for /discovery
 */
func doDiscoveryBadMethod(w http.ResponseWriter, r *http.Request) {
	respond_405(w, r.URL.Path, "PUT")
}

func doPutDiscovery(w http.ResponseWriter, r *http.Request) {
	var report HTTPReport
	log_call(r)

	if r.Body == nil {
		respond_400(w, r.URL.Path, "Missing request body")
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	bmcAddrs := new(storage.SystemAddresses)
	err = json.Unmarshal(body, bmcAddrs)
	if err != nil {
		respond_400(w, r.URL.Path, "Error while parsing json request")
		return
	}
	if len(bmcAddrs.Addresses) < 1 {
		respond_400(w, r.URL.Path, "Missing MAC addresses")
		return
	}

	bmc := removeBMC(bmcAddrs)
	if len(bmc.Credentials.Username) != 0 {
		report.reportType = HTTPREPORT_CONFIG_COMPLETE
		report.bmcAddrs = bmc.BMCAddrs.Addresses
		report.username = bmc.Credentials.Username
		report.password = bmc.Credentials.Password
	} else {
		respond_400(w, r.URL.Path, "BMC credentials not found")
		return
	}

	imchan <- report
	respond_200(w)
}

func GetRouterV1(r *mux.Router) {
	// Define soubrouter for /v1/

	s := r.PathPrefix("/v1").Subrouter()

	s.HandleFunc("/readiness", doReadinessCheck).Methods("GET")
	s.HandleFunc("/liveness", doLivenessCheck).Methods("GET")

	s.HandleFunc("/credentials", doPostCredentials).Methods("POST")
	s.HandleFunc("/credentials", doCredentialsBadMethod).Methods("GET", "PUT", "DELETE")
	s.HandleFunc("/discovery", doPutDiscovery).Methods("PUT")
	s.HandleFunc("/discovery", doDiscoveryBadMethod).Methods("GET", "POST", "DELETE")

	// TODO: this setup gives blank responses for things that are the wrong method
	// (eg: GET to /v1/credentials).  Fix that so some response is given
}

func GetRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(simpleMw)

	router.HandleFunc("/", Versions)
	GetRouterV1(router)

	return (router)
}

func run_HTTPsrv(ichan chan HTTPReport, istorage storage.Storage) {
	router := GetRouter()

	imchan = ichan
	globalStorage = istorage

	log.Printf("Connecting to secure store (Vault)...")
	// Start a connection to Vault
	if ss, err := sstorage.NewVaultAdapter("secret"); err != nil {
		log.Printf("Error: Secure Store connection failed - %s", err)
		panic(err)
	} else {
		log.Printf("Connection to secure store (Vault) succeeded")
		credStorage = model.NewRedsCredStore("reds-creds", ss)
	}

	log.Fatal(http.ListenAndServe(httpListen, router))
}
