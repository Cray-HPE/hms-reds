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

package storage

import "testing"

func TestMacState_String(t *testing.T) {

	testData := MacState{
		DiscoveredHTTP: true,
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
		want:  "MacState - HTTP:true, Switch:SomeName[fortyGigabit 3/12] IP:10.11.12.13",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("MacState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
