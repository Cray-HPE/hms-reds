// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package snmp

import (
	"log"
	"os"

	"stash.us.cray.com/HMS/hms-reds/internal/snmp/common"
	"stash.us.cray.com/HMS/hms-reds/internal/snmp/dell"
	"stash.us.cray.com/HMS/hms-reds/internal/storage"
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
