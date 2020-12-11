// Copyright 2019 Cray Inc. All Rights Reserved.
// Except as permitted by contract or express written permission of Cray Inc.,
// no part of this work or its content may be modified, used, reproduced or
// disclosed in any form. Modifications made without express permission of
// Cray Inc. may damage the system the software is installed within, may
// disqualify the user from receiving support from Cray Inc. under support or
// maintenance contracts, or require additional support services outside the
// scope of those contracts to repair the software or system.

package factory

import (
	"errors"

	"stash.us.cray.com/HMS/hms-reds/internal/storage"
	"stash.us.cray.com/HMS/hms-reds/internal/storage/etcd"
)

const datastoreTypeEtcd = "etcd"

// MakeStorage creates a storage object for the rest of our code to work with.
func MakeStorage(dstype string, url string, insecure bool) (storage.Storage, error) {
	var ret storage.Storage

	if dstype != datastoreTypeEtcd {
		return nil, errors.New("datastore-type must be \""  + datastoreTypeEtcd + "\"")
	}

	ret = new(etcd.Etcd)
	ret.Init(url, false)

	return ret, nil
}
