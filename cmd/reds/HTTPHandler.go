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
	"log"
	"net/http"

	"github.com/Cray-HPE/hms-reds/internal/columbia"
	"github.com/Cray-HPE/hms-reds/internal/model"
	"github.com/Cray-HPE/hms-reds/internal/storage"
	sstorage "github.com/Cray-HPE/hms-securestorage"
	"github.com/gorilla/mux"
)

// Channel for sending notifications
var imchan chan HTTPReport
var globalStorage storage.Storage

var credStorage *model.RedsCredStore

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

func respond_204(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func respond_503(w http.ResponseWriter, msg string) {
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	w.Write([]byte(msg + "\n"))
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

func GetRouterV1(r *mux.Router) {
	// Define soubrouter for /v1/

	s := r.PathPrefix("/v1").Subrouter()

	s.HandleFunc("/readiness", doReadinessCheck).Methods("GET")
	s.HandleFunc("/liveness", doLivenessCheck).Methods("GET")

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
