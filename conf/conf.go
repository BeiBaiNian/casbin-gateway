// Copyright 2023 The casbin Authors. All Rights Reserved.
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

package conf

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/beego/beego"
)

//go:embed waf.conf
var WafConf string

func init() {
	// this array contains the beego configuration items that may be modified via env
	presetConfigItems := []string{"httpport", "appname"}
	for _, key := range presetConfigItems {
		if value, ok := os.LookupEnv(key); ok {
			err := beego.AppConfig.Set(key, value)
			if err != nil {
				panic(err)
			}
		}
	}
}

func GetConfigString(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	res := beego.AppConfig.String(key)
	if res == "" {
		if key == "staticBaseUrl" {
			res = "https://cdn.casbin.org"
		} else if key == "logConfig" {
			res = fmt.Sprintf("{\"filename\": \"logs/%s.log\", \"maxdays\":99999, \"perm\":\"0770\"}", beego.AppConfig.String("appname"))
		}
	}

	return res
}

func GetConfigBool(key string) bool {
	value := GetConfigString(key)
	if value == "true" {
		return true
	} else {
		return false
	}
}

func GetConfigInt(key string) int {
	value := GetConfigString(key)
	num, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return num
}

// GetConfigIntDefault reads an int setting, falling back to defaultValue when
// the key is missing or unparsable. Unlike GetConfigInt it never panics, so it
// suits settings that have a sensible built-in value.
func GetConfigIntDefault(key string, defaultValue int) int {
	value := strings.Trim(GetConfigString(key), `"' `)
	num, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return num
}

// GetHttpPort is the port serving the management UI and the REST API.
func GetHttpPort() int {
	return GetConfigIntDefault("httpport", 17000)
}

// IsGatewayEnabled reports whether the reverse-proxy gateway should bind the
// gateway ports. It defaults to false: taking over ports 80 and 443 is opt-in,
// because those usually belong to something else on the host.
func IsGatewayEnabled() bool {
	return strings.EqualFold(strings.Trim(GetConfigString("gatewayEnabled"), `"' `), "true")
}

// GetGatewayHttpPort is the plain-HTTP port of the reverse-proxy gateway.
func GetGatewayHttpPort() int {
	return GetConfigIntDefault("gatewayHttpPort", 80)
}

// GetGatewayHttpsPort is the HTTPS port of the reverse-proxy gateway.
func GetGatewayHttpsPort() int {
	return GetConfigIntDefault("gatewayHttpsPort", 443)
}

func GetConfigInt64(key string) (int64, error) {
	value := GetConfigString(key)
	num, err := strconv.ParseInt(value, 10, 64)
	return num, err
}

// DefaultSqliteDataSourceName is used when "dataSourceName" is empty.
const DefaultSqliteDataSourceName = "./data/casbin-gateway.db"

func GetConfigDriverName() string {
	driverName := strings.Trim(GetConfigString("driverName"), `"' `)
	if driverName == "" {
		return "sqlite"
	}

	return driverName
}

// IsSqliteDriver accepts both spellings: modernc.org/sqlite registers itself as
// "sqlite" while XORM calls its dialect "sqlite3".
func IsSqliteDriver(driverName string) bool {
	return driverName == "sqlite" || driverName == "sqlite3"
}

func GetConfigDataSourceName() string {
	dataSourceName := GetConfigString("dataSourceName")

	// A SQLite data source is a file path, so the Docker host rewrite below
	// does not apply to it.
	if IsSqliteDriver(GetConfigDriverName()) {
		dataSourceName = strings.Trim(dataSourceName, `"' `)
		if dataSourceName == "" {
			return DefaultSqliteDataSourceName
		}

		return dataSourceName
	}

	runningInDocker := os.Getenv("RUNNING_IN_DOCKER")
	if runningInDocker == "true" {
		// https://stackoverflow.com/questions/48546124/what-is-linux-equivalent-of-host-docker-internal
		if runtime.GOOS == "linux" {
			dataSourceName = strings.ReplaceAll(dataSourceName, "localhost", "172.17.0.1")
		} else {
			dataSourceName = strings.ReplaceAll(dataSourceName, "localhost", "host.docker.internal")
		}
	}

	return dataSourceName
}

func GetLanguage(language string) string {
	if language == "" || language == "*" {
		return "en"
	}

	if len(language) != 2 || language == "nu" {
		return "en"
	} else {
		return language
	}
}

func IsDemoMode() bool {
	return strings.ToLower(GetConfigString("isDemoMode")) == "true"
}

func GetConfigBatchSize() int {
	res, err := strconv.Atoi(GetConfigString("batchSize"))
	if err != nil {
		res = 100
	}
	return res
}

// GetAgentPatchStateDir is the local directory that holds agent patch
// manifests, backups, and monitor cursors. It is operational state only; agent
// behaviour records remain in memory.
func GetAgentPatchStateDir() string {
	dir := GetConfigString("agentPatchStateDir")
	if dir == "" {
		return "./data/agent-patches"
	}
	return dir
}

// GetAgentRecordCapacity is how many agent monitoring records Gateway keeps in
// memory. Records are never written to disk, so this value alone bounds the
// memory the live window can occupy.
func GetAgentRecordCapacity() int {
	res, err := strconv.Atoi(GetConfigString("agentRecordCapacity"))
	if err != nil || res <= 0 {
		res = 1000
	}
	return res
}

// GetAgentMonitorPollSeconds is how often Gateway rescans the append-only logs
// of agents that are monitored by tailing files. Larger values trade detection
// latency for far less disk traffic on hosts with a long agent history.
func GetAgentMonitorPollSeconds() int {
	res, err := strconv.Atoi(GetConfigString("agentMonitorPollSeconds"))
	if err != nil || res <= 0 {
		res = 5
	}
	return res
}

func GetConfigRealDataSourceName(driverName string) string {
	var dataSourceName string
	if driverName != "mysql" {
		dataSourceName = GetConfigDataSourceName()
	} else {
		dataSourceName = GetConfigDataSourceName() + GetConfigString("dbName")
	}
	return dataSourceName
}
