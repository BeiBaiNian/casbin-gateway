// Copyright 2024 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"fmt"

	"github.com/apache/casbin-gateway/embedsupport"
	"github.com/apache/casbin-gateway/util"
)

// ipDbPath is where the IP location database lives in the source tree.
const ipDbPath = "ip/17monipdb.dat"

// InitIpDb loads the IP location database used to tell abroad traffic apart
// from Chinese traffic. The file on disk is used when it is there; a binary
// built with -tags embed falls back to the copy baked into it.
func InitIpDb() {
	if util.FileExist(ipDbPath) {
		err := Init(ipDbPath)
		if err != nil {
			panic(err)
		}

		return
	}

	data := embedsupport.IpDb()
	if len(data) == 0 {
		panic(fmt.Errorf("the IP location database \"%s\" is missing", ipDbPath))
	}

	InitWithData(data)
}

func IsAbroadIp(ip string) bool {
	// If it's an intranet IP, it's not abroad
	if util.IsIntranetIp(ip) {
		return false
	}

	info, err := Find(ip)
	if err != nil {
		fmt.Printf("error: ip = %s, error = %s\n", ip, err.Error())
		return false
	}

	return info.Country != "中国"
}
