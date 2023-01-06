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

package model

import (
	"reflect"
	"testing"
)

func TestRedsCredentials_String(t *testing.T) {
	tests := []struct {
		name     string
		redsCred RedsCredentials
		want     string
	}{{
		name:     "RedactedOutput",
		redsCred: RedsCredentials{Username: "admin", Password: "terminal0"},
		want:     "Username: admin, Password: <REDACTED>",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.redsCred.String(); got != tt.want {
				t.Errorf("RedsCredentials.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedsCredStore_GetDefaultCredentials(t *testing.T) {
	// setup Vault mock
	ss := NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)
	credDefaults := map[string]RedsCredentials{"Cray": {Username: "groot", Password: "terminal6"}, "Cray ACE": {Username: "aceuser", Password: "acepass"}, "Gigabyte": {Username: "gigabyteuser", Password: "gigabytepass"}}
	ss.Store(CredentialsKeyPrefix+"/defaults", credDefaults)

	tests := []struct {
		name    string
		ccs     *RedsCredStore
		want    map[string]RedsCredentials
		wantErr bool
	}{{
		name:    "AllDefaults",
		ccs:     credStorage,
		want:    credDefaults,
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ccs.GetDefaultCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.GetDefaultCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RedsCredStore.GetDefaultCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}
