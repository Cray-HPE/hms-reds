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
	err := ret.Init(url, false)

	return ret, err
}
