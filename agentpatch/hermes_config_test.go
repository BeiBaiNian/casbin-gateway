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

package agentpatch

import "testing"

// TestHermesPluginMembershipRoundTrip checks that enabling the observer reads
// back through the YAML parser and that disabling restores the original bytes.
func TestHermesPluginMembershipRoundTrip(t *testing.T) {
	cases := map[string]string{
		"block list":       "plugins:\n  enabled:\n    - foo\n  disabled:\n    - baz\n\nother: 1\n",
		"dash at key":      "plugins:\n  enabled:\n  - foo\n",
		"flow list":        "plugins:\n  enabled: [foo, bar]\n",
		"empty flow":       "plugins:\n  enabled: []\n",
		"flow comment":     "plugins:\n  enabled: [foo]  # loaded\n",
		"comments":         "plugins:\n  # which plugins load\n  enabled:\n    - foo   # keep\n\nother: 1\n",
		"crlf":             "plugins:\r\n  enabled:\r\n    - foo\r\n",
		"mixed endings":    "top: 1\nplugins:\r\n  enabled:\r\n    - foo\r\n",
		"nested item":      "plugins:\n  enabled:\n    - name: foo\n      opt: 1\n    - bar\n",
		"quoted entries":   "plugins:\n  enabled:\n    - \"foo\"\n    - 'bar'\n",
		"deep indent":      "plugins:\n    enabled:\n        - foo\n",
		"no final newline": "plugins:\n  enabled:\n    - foo",
		"document start":   "---\nplugins:\n  enabled:\n    - foo\n",
		"no plugins key":   "model: gpt\n\n# section\nlogging:\n  level: info\n",
		"empty file":       "",
	}

	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			on, changed, err := setHermesPluginMembership([]byte(original), true)
			if err != nil || !changed {
				t.Fatalf("enable: changed=%v err=%v", changed, err)
			}
			if err := verifyHermesPluginMembership(on, true); err != nil {
				t.Fatalf("enable does not read back: %v\n%s", err, on)
			}
			off, _, err := setHermesPluginMembership(on, false)
			if err != nil {
				t.Fatalf("disable: %v", err)
			}
			if err := verifyHermesPluginMembership(off, false); err != nil {
				t.Fatalf("disable does not read back: %v\n%s", err, off)
			}
			if string(off) != original {
				t.Errorf("not restored\nwant %q\ngot  %q", original, string(off))
			}
		})
	}
}

// TestHermesPluginMembershipRefuses covers documents the line editor must not
// edit blind.
func TestHermesPluginMembershipRefuses(t *testing.T) {
	cases := map[string]string{
		"plugins is a scalar": "plugins: none\n",
		"enabled is an alias": "defaults: &d\n  - foo\nplugins:\n  enabled: *d\n",
		"enabled is a map":    "plugins:\n  enabled:\n    foo: true\n",
		"quoted plugins key":  "\"plugins\":\n  enabled:\n    - foo\n",
	}

	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			updated, _, err := setHermesPluginMembership([]byte(original), true)
			if err != nil {
				return
			}
			if err := verifyHermesPluginMembership(updated, true); err == nil {
				t.Errorf("edit was accepted:\n%s", updated)
			}
		})
	}
}
