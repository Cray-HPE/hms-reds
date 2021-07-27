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

package snmp

import (
	"log"
	"os"

	"github.com/Cray-HPE/hms-reds/internal/snmp/common"
	"github.com/Cray-HPE/hms-reds/internal/snmp/dell"
	"github.com/Cray-HPE/hms-reds/internal/storage"
)

func GetSwitch(name string, storage *storage.Storage) (common.Implementation, error) {
	ret := new(dell.DellONSwitchInfo)

	err := ret.Init(name, storage)
	if err != nil {
		byPassSwitchBlacklistPanic := os.Getenv("REDS_BYPASS_SWITCH_BLACKLIST_PANIC")
		if byPassSwitchBlacklistPanic != "" {
			log.Printf("WARNING: Failed to initialize switch %s, but proceeding after error: %s", name, err)
		} else {
			log.Printf("ERROR: Failed to initialize switch %s, calling panic with error: %s", name, err)
			panic(err)
		}
	}
	return ret, err
}
