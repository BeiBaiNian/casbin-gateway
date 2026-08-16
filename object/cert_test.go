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

//go:build !skipCi
// +build !skipCi

package object

import (
	"testing"
	"time"
)

func TestGetCertExpireTime(t *testing.T) {
	certPem, notAfter := generateTestCertPem(t)
	createTestCert(t, "casbin.com", certPem)

	cert, err := getCert(testOwner, "casbin.com")
	if err != nil {
		t.Fatalf("getCert() error: %v", err)
	}
	if cert == nil {
		t.Fatalf("cert should not be nil")
	}

	expireTime, err := getCertExpireTime(cert.Certificate)
	if err != nil {
		t.Fatalf("getCertExpireTime() error: %v", err)
	}

	expected := notAfter.Truncate(time.Second)
	actual, err := time.Parse(time.RFC3339, expireTime)
	if err != nil {
		t.Fatalf("failed to parse expire time %q: %v", expireTime, err)
	}
	if !actual.Equal(expected) {
		t.Fatalf("expire time = %s, want %s", actual.Format(time.RFC3339), expected.Format(time.RFC3339))
	}
}

// TestRenewCert renews a real certificate via an external provider, which is a
// manual operation and is skipped in automated test runs.
func TestRenewCert(t *testing.T) {
	t.Skip("renews a real certificate via an external provider, run manually with real data")
}

// TestRenewAllCerts renews real certificates via external providers, which is
// a manual operation and is skipped in automated test runs.
func TestRenewAllCerts(t *testing.T) {
	t.Skip("renews real certificates via external providers, run manually with real data")
}

// TestCheckCertsNoOpForEmptyNodes covers the no-op path of checkCerts() for
// a site fixture with no nodes and no tag: getNodeNameFromTag("") returns ""
// which never equals the real hostname, so checkCerts() returns nil before
// the domain loop runs. The test therefore really verifies getCertMap() and
// getSite() plus that early return, not any certificate processing.
func TestCheckCertsNoOpForEmptyNodes(t *testing.T) {
	createTestSite(t, "test-site")

	var err error
	certMap, err = getCertMap()
	if err != nil {
		t.Fatalf("getCertMap() error: %v", err)
	}

	site, err := getSite(testOwner, "test-site")
	if err != nil {
		t.Fatalf("getSite() error: %v", err)
	}
	if site == nil {
		t.Fatalf("site should not be nil")
	}

	if err = site.checkCerts(); err != nil {
		t.Fatalf("checkCerts() error: %v", err)
	}
}
