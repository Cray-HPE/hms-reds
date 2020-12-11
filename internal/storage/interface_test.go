// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package storage

import "testing"

func TestMacState_String(t *testing.T) {

	testData := MacState{
		DiscoveredHTTP: true,
		DiscoveredSNMP: false,
		SwitchName:     "SomeName",
		SwitchPort:     "fortyGigabit 3/12",
		Username:       "groot",
		Password:       "Terminal6",
		IPAddress:      "10.11.12.13",
	}
	tests := []struct {
		name  string
		state MacState
		want  string
	}{{
		name:  "BasicToString",
		state: testData,
		want:  "MacState - HTTP:true, SNMP:false. Switch:SomeName[fortyGigabit 3/12] IP:10.11.12.13",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("MacState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
