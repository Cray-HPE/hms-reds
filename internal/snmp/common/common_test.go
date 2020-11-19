// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

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
