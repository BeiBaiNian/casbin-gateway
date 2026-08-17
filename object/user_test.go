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

package object

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/conf"
)

// setEnv points the config at a throwaway sqlite database, and restores both
// the environment and the package ormer afterwards so that the other tests in
// this package still see the app.conf database.
func setEnv(t *testing.T, values map[string]string) {
	t.Helper()

	for key, value := range values {
		old, existed := os.LookupEnv(key)
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
		if err := os.Setenv(key, value); err != nil {
			t.Fatal(err)
		}
	}
}

func initSqliteOrmer(t *testing.T) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "gateway_test.db")
	setEnv(t, map[string]string{
		"driverName":     "sqlite",
		"dataSourceName": dbPath,
		"dbName":         "",
	})

	oldOrmer := ormer
	InitAdapter()
	ormer.createTable()

	testOrmer := ormer
	t.Cleanup(func() {
		// Release the file handle before t.TempDir() removes the directory.
		runtime.SetFinalizer(testOrmer, nil)
		_ = testOrmer.Engine.Close()
		ormer = oldOrmer
	})
}

func TestInitUsersSeedsAdminWithoutCasdoor(t *testing.T) {
	setEnv(t, map[string]string{"casdoorEndpoint": ""})
	t.Cleanup(func() { casdoor.InitCasdoorConfig() })

	casdoor.InitCasdoorConfig()
	if conf.IsCasdoorAvailable() {
		t.Fatal("casdoor should not be available with an empty endpoint")
	}
	if !IsSigninEnabled() {
		t.Fatal("local signin should be enabled when casdoor is not configured")
	}

	initSqliteOrmer(t)
	InitUsers()

	user, err := GetUser("admin")
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("InitUsers() should seed the admin user")
	}
	if !IsAdminUsingDefaultPassword() {
		t.Fatal("the seeded admin should be using the default password")
	}

	casdoorUser := user.ToCasdoorUser()
	if casdoorUser.Owner != UserOwner || casdoorUser.Name != "admin" || !casdoorUser.IsAdmin {
		t.Fatalf("ToCasdoorUser() = %+v", casdoorUser)
	}

	if err = UpdateUserPassword(user, "newpass"); err != nil {
		t.Fatal(err)
	}
	if IsAdminUsingDefaultPassword() {
		t.Fatal("admin should no longer be using the default password")
	}

	// Seeding again must not reset an admin that already exists.
	InitUsers()
	if _, ok, _ := VerifyUser("admin", "newpass"); !ok {
		t.Fatal("InitUsers() should leave an existing admin alone")
	}
}

func TestVerifyUser(t *testing.T) {
	setEnv(t, map[string]string{"casdoorEndpoint": ""})
	t.Cleanup(func() { casdoor.InitCasdoorConfig() })
	casdoor.InitCasdoorConfig()

	initSqliteOrmer(t)
	InitUsers()

	if _, ok, _ := VerifyUser("admin", "123"); !ok {
		t.Fatal("the seeded admin should sign in with the default password")
	}
	if _, ok, _ := VerifyUser("admin", "wrong"); ok {
		t.Fatal("a wrong password should not sign in")
	}
	if _, ok, _ := VerifyUser("nobody", "123"); ok {
		t.Fatal("an unknown user should not sign in")
	}
	if _, ok, err := VerifyUser("", "123"); ok || err == nil {
		t.Fatal("an empty username should be rejected")
	}
}

func TestSigninDisabledWhenCasdoorConfigured(t *testing.T) {
	setEnv(t, map[string]string{"casdoorEndpoint": "https://door.casdoor.com"})
	t.Cleanup(func() { casdoor.InitCasdoorConfig() })

	casdoor.InitCasdoorConfig()
	if !conf.IsCasdoorAvailable() {
		t.Fatal("casdoor should be available when an endpoint is configured")
	}
	if IsSigninEnabled() {
		t.Fatal("local signin should be disabled when casdoor is configured")
	}
}
