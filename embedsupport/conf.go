// Copyright 2026 The casbin Authors. All Rights Reserved.
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

package embedsupport

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
)

// usingEmbeddedConf records that the embedded conf/app.conf was loaded because
// no on-disk one was found, i.e. that this process runs as a single binary.
var usingEmbeddedConf bool

// IsEmbeddedConf reports whether the settings in use came from the copy of
// conf/app.conf baked into the binary rather than from a file on disk.
func IsEmbeddedConf() bool { return usingEmbeddedConf }

// setupConf loads the embedded conf/app.conf, unless an on-disk one exists —
// beego has already loaded that one, and disk always wins.
//
// beego can only read its config from a file, so the embedded copy is written
// to a temporary file just long enough to be parsed. Nothing reads the path
// afterwards: beego keeps the parsed values in memory.
func setupConf(appConf string) {
	if appConf == "" || onDiskConfPath() != "" {
		return
	}

	tmpDir, err := os.MkdirTemp("", "casbin-gateway-conf-*")
	if err != nil {
		warnConf("cannot create a temporary directory for the embedded conf/app.conf: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpConf := filepath.Join(tmpDir, "app.conf")
	if err = os.WriteFile(tmpConf, []byte(appConf), 0o600); err != nil {
		warnConf("cannot write the embedded conf/app.conf to %s: %v", tmpConf, err)
		return
	}

	if err = beego.LoadAppConfig("ini", tmpConf); err != nil {
		warnConf("cannot load the embedded conf/app.conf: %v", err)
		return
	}

	// LoadAppConfig() replaces beego.AppConfig wholesale, which drops the
	// values conf's own init() had already taken from the environment.
	conf.ApplyEnvOverrides()

	usingEmbeddedConf = true
	fmt.Println("Using the conf/app.conf embedded in this binary. To change any setting, put your own conf/app.conf next to the executable.")
}

// onDiskConfPath returns the config file beego found on disk, or "" when there
// is none. It repeats the search beego does in its own init(): BEEGO_CONFIG_PATH
// first, then conf/app.conf under the working directory, then the same under
// the executable's directory. BEEGO_RUNMODE renames the file it looks for.
func onDiskConfPath() string {
	filename := "app.conf"
	if runmode := os.Getenv("BEEGO_RUNMODE"); runmode != "" {
		filename = runmode + ".app.conf"
	}

	var candidates []string
	if configPath := os.Getenv("BEEGO_CONFIG_PATH"); configPath != "" {
		candidates = append(candidates, configPath)
	}
	candidates = append(candidates,
		filepath.Join(beego.WorkPath, "conf", filename),
		filepath.Join(beego.AppPath, "conf", filename),
	)

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// warnConf reports a failure to use the embedded config. Startup continues on
// beego's built-in defaults, which is worth saying out loud: the ports and the
// database it then uses are not the ones app.conf asks for.
func warnConf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "casbin-gateway: %s\n", fmt.Sprintf(format, args...))
	fmt.Fprintln(os.Stderr, "casbin-gateway: falling back to the built-in defaults; put a conf/app.conf next to the executable to control the settings")
}
