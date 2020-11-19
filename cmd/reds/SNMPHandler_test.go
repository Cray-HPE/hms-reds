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
	"testing"
)

func TestSplitToValues(t *testing.T) {
	testString := ".1.3.6.1.2.1.1.3.0 = 42,.1.3.6.1.6.3.1.1.4.1.0 = .1.3.6.1.6.3.1.1.4.1.0,.1.3.6.1.4.1.9.1.663 = \"00 11 22 33 44 56 \""
	res := SplitToValues(testString)

	value, ok := res[".1.3.6.1.2.1.1.3.0"]
	if !ok {
		t.Errorf("Lookup of key .1.3.6.1.2.1.1.3.0 failed, map was: %s", res)
	} else if value != "42" {
		t.Errorf("Value of key .1.3.6.1.2.1.1.3.0 was wrong; expected 42, got %s", value)
	}

	value, ok = res[".1.3.6.1.6.3.1.1.4.1.0"]
	if !ok {
		t.Errorf("Lookup of key .1.3.6.1.6.3.1.1.4.1.0 failed, map was: %s", res)
	} else if value != ".1.3.6.1.6.3.1.1.4.1.0" {
		t.Errorf("Value of key .1.3.6.1.6.3.1.1.4.1.0 was wrong; expected .1.3.6.1.6.3.1.1.4.1.0, got %s", value)
	}

	value, ok = res[".1.3.6.1.4.1.9.1.663"]
	if !ok {
		t.Errorf("Lookup of key .1.3.6.1.4.1.9.1.663 failed, map was: %s", res)
	} else if value != "\"00 11 22 33 44 56 \"" {
		t.Errorf("Value of key .1.3.6.1.4.1.9.1.663 was wrong; expected \"00 11 22 33 44 56 \", got %s", value)
	}
}

func TestSplitSNMPLine(t *testing.T) {
	testString1 := "172.17.0.1 , .1.3.6.1.2.1.1.3.0 = 42"
	testString2 := "some.dummy.host , .1.3.6.1.2.1.1.3.0 = 42"

	IPAddr, Line := SplitSNMPLine(testString1)
	if IPAddr != "172.17.0.1" {
		t.Errorf("IP address extraction in testString1 failed, should have been 172.17.0.1, got %s", IPAddr)
	}

	if Line != ".1.3.6.1.2.1.1.3.0 = 42" {
		t.Errorf("Line extraction failed on testString1, should have been \".1.3.6.1.2.1.1.3.0 = 42\", got %s", Line)
	}

	IPAddr, Line = SplitSNMPLine(testString2)
	if IPAddr != "some.dummy.host" {
		t.Errorf("IP address extraction failed on testString2, should have been 172.17.0.1, got %s", IPAddr)
	}

	if Line != ".1.3.6.1.2.1.1.3.0 = 42" {
		t.Errorf("Line extraction failed on testString2, should have been \".1.3.6.1.2.1.1.3.0 = 42\", got %s", Line)
	}
}
