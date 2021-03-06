// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package distro

import (
	"sync/atomic"
)

type introData map[string]string

var data atomic.Value

func Replace(distro introData) {
	data.Store(distro)
}

func Data(k string) string {
	var d introData
	atomd := data.Load()
	// we need a fallback to keep compatibility in scenarios without inject distro info
	// related issue: https://github.com/pingcap/tidb-dashboard/issues/975
	if d == nil {
		d = Resource
	} else {
		d = atomd.(introData)
	}

	if d[k] == "" {
		return k
	}
	return d[k]
}
