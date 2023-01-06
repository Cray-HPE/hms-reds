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
	"github.com/gorilla/mux"
)

func respond_204(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func respond_503(w http.ResponseWriter, msg string) {
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	w.Write([]byte(msg + "\n"))
}

func Versions(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("/v1\n"))
}

/*
 * Validates that reds dependencies are available.
 */
func doReadinessCheck(w http.ResponseWriter, r *http.Request) {
	// check sls
	if !columbia.ColumbiaListRead() {
		msg := "columbia switches not read from SLS for REDS"
		respond_503(w, msg)
		return
	}

	respond_204(w)
	return
}

/*
 * Returns success once the reds server is up
 */
func doLivenessCheck(w http.ResponseWriter, r *http.Request) {
	respond_204(w)
}

func run_HTTPsrv() {
	router := mux.NewRouter()

	router.HandleFunc("/", Versions)

	subrouter := router.PathPrefix("/v1").Subrouter()

	subrouter.HandleFunc("/readiness", doReadinessCheck).Methods("GET")
	subrouter.HandleFunc("/liveness", doLivenessCheck).Methods("GET")

	log.Fatal(http.ListenAndServe(httpListen, router))
}
