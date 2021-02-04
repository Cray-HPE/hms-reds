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

package common

import (
	"testing"
)

func Test_MacAddressFromOID_good(t *testing.T) {
	var mac string
	ret, err := MacAddressFromOID("1.17.4.3.1.1.164.191.0.43.110.255")
	mac = *ret
	expected := "a4bf002b6eff"
	if err != nil {
		t.Errorf("Got error: %s", err)
	}
	if mac != expected {
		t.Errorf("Conversion failed; expected %s, got %s", expected, mac)
	}
}

func Test_MacAddressFromOID_tooBigValue(t *testing.T) {
	ret, err := MacAddressFromOID("1.17.4.3.1.1.164.191.1.43.110.256")
	if ret != nil {
		t.Errorf("Did not return nil value (and should have)!")
	}
	if err == nil {
		t.Errorf("Returned nil error!")
	}
}

func Test_MacAddressFromOID_tooShort(t *testing.T) {
	ret, err := MacAddressFromOID("1.17.4.3.1")
	if ret != nil {
		t.Errorf("Did not return nil value (and should have)!")
	}
	if err == nil {
		t.Errorf("Returned nil error!")
	}
}
