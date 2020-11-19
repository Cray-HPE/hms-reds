// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package model

import (
	"reflect"
	"testing"

	"stash.us.cray.com/HMS/hms-reds/internal/storage"
	"stash.us.cray.com/HMS/hms-reds/internal/storage/mock"
)

func TestRedsCredStore_GetGlobalCredentials(t *testing.T) {
	ss := mock.NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)

	testData := storage.BMCCredentials{
		Username: "foo",
		Password: "bar",
	}

	tests := []struct {
		name         string
		ccs          *RedsCredStore
		pushRedsKey  string
		pushRedsCred storage.BMCCredentials
		wantRedsCred storage.BMCCredentials
		wantErr      bool
	}{{
		name:         "EmptyGet",
		ccs:          credStorage,
		pushRedsKey:  "",
		pushRedsCred: storage.BMCCredentials{},
		wantRedsCred: storage.BMCCredentials{},
		wantErr:      false,
	}, {
		name:         "BasicGet",
		ccs:          credStorage,
		pushRedsKey:  CredentialsKeyPrefix + "/global/ipmi",
		pushRedsCred: testData,
		wantRedsCred: testData,
		wantErr:      false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.pushRedsKey) > 0 {
				ss.Store(tt.pushRedsKey, tt.pushRedsCred)
			}
			gotRedsCred, err := tt.ccs.GetGlobalCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.GetGlobalCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRedsCred, tt.wantRedsCred) {
				t.Errorf("RedsCredStore.GetGlobalCredentials() = %v, want %v", gotRedsCred, tt.wantRedsCred)
			}
		})
	}
}

func TestRedsCredStore_ClearGlobalCredentials(t *testing.T) {
	ss := mock.NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)

	tests := []struct {
		name    string
		ccs     *RedsCredStore
		wantErr bool
	}{{
		name:    "CheckEmpty",
		ccs:     credStorage,
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ccs.ClearGlobalCredentials(); (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.ClearGlobalCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			globalCreds := &storage.BMCCredentials{}
			emptyCreds := &storage.BMCCredentials{}
			ss.Lookup(CredentialsKeyPrefix+"/global/ipmi", globalCreds)
			if !reflect.DeepEqual(globalCreds, emptyCreds) {
				t.Errorf("RedsCredStore.GetGlobalCredentials() = %v, want %v", globalCreds, emptyCreds)
			}
		})
	}
}

func TestRedsCredStore_AddMacCredentials(t *testing.T) {
	ss := mock.NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)

	testAddrs1 := storage.SystemAddresses{
		Addresses: []storage.BMCAddress{
			storage.BMCAddress{
				MACAddress: "00beef151337",
			},
			storage.BMCAddress{
				MACAddress: "00d00d15af00",
			},
		},
	}
	testData1 := storage.BMCCredItem{
		Credentials: storage.BMCCredentials{
			Username: "foo",
			Password: "bar",
		},
		BMCAddrs: &testAddrs1,
	}

	type args struct {
		mac   string
		creds storage.BMCCredItem
	}
	tests := []struct {
		name         string
		ccs          *RedsCredStore
		args         args
		wantErr      bool
		wantMacCreds storage.BMCCredItem
	}{{
		name:         "SingleMAC",
		ccs:          credStorage,
		args:         args{"00beef151337", testData1},
		wantErr:      false,
		wantMacCreds: testData1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ccs.AddMacCredentials(tt.args.mac, tt.args.creds); (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.AddMacCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			macCreds := &storage.BMCCredItem{}
			ss.Lookup(CredentialsKeyPrefix+"/"+tt.args.mac, macCreds)
			if !reflect.DeepEqual(*macCreds, tt.args.creds) {
				t.Errorf("RedsCredStore.AddMacCredentials() = %v, want %v", *macCreds, tt.args.creds)
			}
		})
	}
}

func TestRedsCredStore_FindMacCredentials(t *testing.T) {
	ss := mock.NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)

	testAddrs1 := storage.SystemAddresses{
		Addresses: []storage.BMCAddress{
			storage.BMCAddress{
				MACAddress: "00beef151337",
			},
			storage.BMCAddress{
				MACAddress: "00d00d15af00",
			},
		},
	}
	testData1 := storage.BMCCredItem{
		Credentials: storage.BMCCredentials{
			Username: "foo",
			Password: "bar",
		},
		BMCAddrs: &testAddrs1,
	}

	type args struct {
		mac   string
		creds storage.BMCCredItem
	}
	tests := []struct {
		name         string
		ccs          *RedsCredStore
		args         args
		wantRedsCred storage.BMCCredItem
		wantErr      bool
	}{{
		name:         "SingleMAC",
		ccs:          credStorage,
		args:         args{"00beef151337", testData1},
		wantErr:      false,
		wantRedsCred: testData1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ccs.AddMacCredentials(tt.args.mac, tt.args.creds)
			gotRedsCred, err := tt.ccs.FindMacCredentials(tt.args.mac)
			if (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.FindMacCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRedsCred, tt.wantRedsCred) {
				t.Errorf("RedsCredStore.FindMacCredentials() = %v, want %v", gotRedsCred, tt.wantRedsCred)
			}
		})
	}
}

func TestRedsCredStore_ClearMacCredentials(t *testing.T) {
	ss := mock.NewKvMock()
	credStorage := NewRedsCredStore(CredentialsKeyPrefix, ss)

	type args struct {
		mac string
	}
	tests := []struct {
		name    string
		ccs     *RedsCredStore
		args    args
		wantErr bool
	}{{
		name:    "SingleMAC",
		ccs:     credStorage,
		args:    args{"00beef151337"},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ccs.ClearMacCredentials(tt.args.mac); (err != nil) != tt.wantErr {
				t.Errorf("RedsCredStore.ClearMacCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			gotRedsCred, _ := tt.ccs.FindMacCredentials(tt.args.mac)
			emptyCreds := storage.BMCCredItem{}
			if !reflect.DeepEqual(gotRedsCred, emptyCreds) {
				t.Errorf("RedsCredStore.GetGlobalCredentials() = %v, want %v", gotRedsCred, emptyCreds)
			}

		})
	}
}

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
	ss := mock.NewKvMock()
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
