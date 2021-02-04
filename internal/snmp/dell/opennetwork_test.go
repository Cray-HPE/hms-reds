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

package dell

import (
	"reflect"
	"testing"
)

func Test_verifySwitchSWVersion_hit(t *testing.T) {
	var sysDescr = `SNMPv2-MIB::sysDescr.0 = STRING: Dell Networking OS
	Operating System Version: 2.0
	Application Software Version: 9.14(1.1)
	Series: S4048T-ON
	Copyright (c) 1999-2017 by Dell Inc. All Rights Reserved.
	Build Time: Mon Jun  5 09:25:23 2017`

	//blacklist of known bad switch software versions
	var blacklist = []string{"9.14(1.1)"}

	err := verifySwitchSWVersion(sysDescr, blacklist)
	if err == nil {
		t.Errorf("Missed known bad blacklist value \n%s", blacklist)
	}
}

func Test_verifySwitchSWVersion_miss(t *testing.T) {
	var sysDescr = `SNMPv2-MIB::sysDescr.0 = STRING: Dell Networking OS
	Operating System Version: 2.0
	Application Software Version: 9.11(2.1)
	Series: S4048T-ON
	Copyright (c) 1999-2017 by Dell Inc. All Rights Reserved.
	Build Time: Mon Jun  5 09:25:23 2017`

	//blacklist of known bad switch software versions
	var blacklist = []string{"9.14(1.1)"}

	err := verifySwitchSWVersion(sysDescr, blacklist)
	if err != nil {
		t.Errorf("Incorrectly found bad blacklist value %s\n", blacklist)
	}
}

func Test_diffTables_good(t *testing.T) {
	oldTable := make(map[string]string)
	oldTable["001122334455"] = "Gigabit 1/1"
	oldTable["1cedc0ffee00"] = "FortyGigabit 2/30"

	newTable := make(map[string]string)
	newTable["1cedc0ffee00"] = "FortyGigabit 2/30"
	newTable["aabbccddeeff"] = "Ethernet 2/2"

	newRes, oldRes := diffTables(oldTable, newTable)

	oldExpected := map[string]string{
		"001122334455": "Gigabit 1/1",
	}
	newExpected := map[string]string{
		"aabbccddeeff": "Ethernet 2/2",
	}

	if !reflect.DeepEqual(newRes, newExpected) {
		t.Errorf("Bad value for new: expected: \n%s, got \n%s", newExpected, newRes)
	}

	if !reflect.DeepEqual(oldRes, oldExpected) {
		t.Errorf("Bad value for old: expected: \n%s, got \n%s", oldExpected, oldRes)
	}
}

func Test_diffTables_good_oneEmpty(t *testing.T) {
	oldTable := make(map[string]string)

	newTable := make(map[string]string)
	newTable["1cedc0ffee00"] = "FortyGigabit 2/30"
	newTable["aabbccddeeff"] = "Ethernet 2/2"

	newRes, oldRes := diffTables(oldTable, newTable)

	oldExpected := map[string]string{}
	newExpected := map[string]string{
		"1cedc0ffee00": "FortyGigabit 2/30",
		"aabbccddeeff": "Ethernet 2/2",
	}

	if !reflect.DeepEqual(newRes, newExpected) {
		t.Errorf("Bad value for new: expected: \n%s, got \n%s", newExpected, newRes)
	}

	if !reflect.DeepEqual(oldRes, oldExpected) {
		t.Errorf("Bad value for old: expected: \n%s, got \n%s", oldExpected, oldRes)
	}
}

func Test_diffTables_good_differentValues(t *testing.T) {
	oldTable := make(map[string]string)
	oldTable["001122334455"] = "Gigabit 1/1"
	oldTable["1cedcoffee00"] = "FortyGigabit 2/30"

	newTable := make(map[string]string)
	newTable["1cedcoffee00"] = "FortyGigabit 2/30"
	newTable["001122334455"] = "Ethernet 2/2"

	newRes, oldRes := diffTables(oldTable, newTable)

	oldExpected := map[string]string{
		"001122334455": "Gigabit 1/1",
	}
	newExpected := map[string]string{
		"001122334455": "Ethernet 2/2",
	}

	if !reflect.DeepEqual(newRes, newExpected) {
		t.Errorf("Bad value for new: expected: \n%s, got \n%s", newExpected, newRes)
	}

	if !reflect.DeepEqual(oldRes, oldExpected) {
		t.Errorf("Bad value for old: expected: \n%s, got \n%s", oldExpected, oldRes)
	}
}
