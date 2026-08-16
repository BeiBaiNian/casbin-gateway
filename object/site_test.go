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

//go:build !skipCi
// +build !skipCi

package object

import (
	"testing"
)

// TestUpdateSiteStatus verifies that UpdateSite persists a status change
// ("Inactive") for a site fixture. Note: the createTestSite fixture has no
// nodes, so the checkNodes() call above only exercises the empty-node path
// and none of its ping/process side effects are executed.
func TestUpdateSiteStatus(t *testing.T) {
	createTestSite(t, "caswaf_my")

	site, err := getSite(testOwner, "caswaf_my")
	if err != nil {
		t.Fatalf("getSite() error: %v", err)
	}
	if site == nil {
		t.Fatalf("site should not be nil")
	}

	site.Status = "Active"
	if err = site.checkNodes(); err != nil {
		t.Fatalf("checkNodes() error: %v", err)
	}

	site, err = getSite(testOwner, "caswaf_my")
	if err != nil {
		t.Fatalf("getSite() error: %v", err)
	}
	if site == nil {
		t.Fatalf("site should not be nil")
	}

	site.Status = "Inactive"
	if _, err = UpdateSite(site.GetId(), site); err != nil {
		t.Fatalf("UpdateSite() error: %v", err)
	}

	updated, err := getSite(testOwner, "caswaf_my")
	if err != nil {
		t.Fatalf("getSite() error: %v", err)
	}
	if updated == nil {
		t.Fatalf("site should not be nil")
	}
	if updated.Status != "Inactive" {
		t.Fatalf("expected site status to be \"Inactive\", got %q", updated.Status)
	}
}
