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

//go:build !skipCi
// +build !skipCi

package object

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
)

// testOwner is a dedicated owner used by test fixtures, so that the tests
// never touch data of real users (e.g. "admin" or the developer's own account).
const testOwner = "caswaf-test"

func TestMain(m *testing.M) {
	InitConfig()
	casdoor.InitCasdoorConfig()
	proxy.InitHttpClient()

	os.Exit(m.Run())
}

// generateTestCertPem generates a self-signed certificate that is valid for
// 100 years and returns its PEM text together with its NotAfter time. The
// certificate is generated on the fly at test runtime, so no certificate
// material is hardcoded in the repository.
func generateTestCertPem(t *testing.T) (string, time.Time) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test certificate key: %v", err)
	}

	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(36500 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "caswaf-test"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), notAfter
}

// createTestSite inserts a site fixture under the dedicated test owner and
// registers a cleanup that deletes it when the test finishes (no matter
// whether it passed, failed or panicked). If a site with the same id already
// exists, the test is skipped to avoid touching real data.
func createTestSite(t *testing.T, name string) *Site {
	t.Helper()

	existing, err := getSite(testOwner, name)
	if err != nil {
		t.Fatalf("getSite() error: %v", err)
	}
	if existing != nil {
		t.Skipf("site %s/%s already exists in the database, skip to avoid touching existing data", testOwner, name)
	}

	site := &Site{
		Owner:          testOwner,
		Name:           name,
		CreatedTime:    util.GetCurrentTime(),
		OtherDomains:   []string{},
		Rules:          []string{},
		AlertProviders: []string{},
		Challenges:     []string{},
		Hosts:          []string{},
		Nodes:          []*NodeItem{},
	}

	_, err = AddSite(site)
	if err != nil {
		t.Fatalf("AddSite() error: %v", err)
	}

	t.Cleanup(func() {
		if _, err := DeleteSite(site); err != nil {
			t.Errorf("DeleteSite() cleanup error: %v", err)
		}
	})

	return site
}

// createTestCert inserts a cert fixture under the dedicated test owner and
// registers a cleanup that deletes it when the test finishes. If a cert with
// the same id already exists, the test is skipped to avoid touching real data.
func createTestCert(t *testing.T, name string, certificate string) *Cert {
	t.Helper()

	existing, err := getCert(testOwner, name)
	if err != nil {
		t.Fatalf("getCert() error: %v", err)
	}
	if existing != nil {
		t.Skipf("cert %s/%s already exists in the database, skip to avoid touching existing data", testOwner, name)
	}

	cert := &Cert{
		Owner:       testOwner,
		Name:        name,
		CreatedTime: util.GetCurrentTime(),
		Type:        "Manual",
		Certificate: certificate,
	}

	_, err = AddCert(cert)
	if err != nil {
		t.Fatalf("AddCert() error: %v", err)
	}

	t.Cleanup(func() {
		if _, err := DeleteCert(cert); err != nil {
			t.Errorf("DeleteCert() cleanup error: %v", err)
		}
	})

	return cert
}
