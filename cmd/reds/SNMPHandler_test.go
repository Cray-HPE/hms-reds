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
