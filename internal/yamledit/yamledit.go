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

// Package yamledit edits a YAML file through its node tree, so a rewrite keeps
// the comments, key order and formatting of everything it does not touch. An
// agent's YAML configuration is a file its owner also edits by hand.
package yamledit

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// indent is what the agents whose files this edits write themselves.
const indent = 2

// Document is one parsed YAML file.
type Document struct {
	root *yaml.Node
}

// Parse reads a document. Empty content is an empty document rather than an
// error: a file the agent has not written yet is not a malformed one.
func Parse(data []byte) (*Document, error) {
	document := &Document{root: &yaml.Node{Kind: yaml.DocumentNode}}
	if len(bytes.TrimSpace(data)) == 0 {
		return document, nil
	}
	if err := yaml.Unmarshal(data, document.root); err != nil {
		return nil, err
	}
	return document, nil
}

// Mapping is the document's root mapping, created when the document is empty.
func (d *Document) Mapping() (*yaml.Node, error) {
	return d.rootNode(yaml.MappingNode, "!!map", "a mapping")
}

// Sequence is the document's root sequence, created when the document is empty.
func (d *Document) Sequence() (*yaml.Node, error) {
	return d.rootNode(yaml.SequenceNode, "!!seq", "a list")
}

// Bytes renders the document. An untouched empty document renders to nothing,
// which is what it was.
func (d *Document) Bytes() ([]byte, error) {
	if len(d.root.Content) == 0 {
		return nil, nil
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(indent)
	if err := encoder.Encode(d.root); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// rootNode returns the root of the wanted kind. A root holding something else
// is an error rather than a value to replace: that is the owner's own file.
func (d *Document) rootNode(kind yaml.Kind, tag string, want string) (*yaml.Node, error) {
	if len(d.root.Content) == 0 {
		node := &yaml.Node{Kind: kind, Tag: tag}
		d.root.Content = []*yaml.Node{node}
		return node, nil
	}

	node := d.root.Content[0]
	if node.Kind == kind {
		return node, nil
	}
	// A file holding nothing but comments parses to a null root, which is as
	// empty as an absent file and is filled in the same way.
	if isNull(node) {
		node.Kind, node.Tag, node.Value, node.Content = kind, tag, "", nil
		return node, nil
	}
	return nil, fmt.Errorf("the document is not %s", want)
}

// Get walks keys through nested mappings, nil when any of them is missing.
func Get(mapping *yaml.Node, keys ...string) *yaml.Node {
	current := mapping
	for _, key := range keys {
		if current == nil || current.Kind != yaml.MappingNode {
			return nil
		}
		current = entry(current, key)
	}
	return current
}

// String is the scalar at keys, empty when it is missing or is not a scalar.
func String(mapping *yaml.Node, keys ...string) string {
	node := Get(mapping, keys...)
	if node == nil || node.Kind != yaml.ScalarNode || isNull(node) {
		return ""
	}
	return node.Value
}

// Set writes value at keys, creating the mappings on the way. The key node of
// an entry that already exists is kept, so its comments survive the change.
func Set(mapping *yaml.Node, value any, keys ...string) error {
	if len(keys) == 0 {
		return errors.New("no key to set")
	}
	node, err := Node(value)
	if err != nil {
		return err
	}

	parent := mapping
	for _, key := range keys[:len(keys)-1] {
		parent, err = ensureMapping(parent, key)
		if err != nil {
			return err
		}
	}
	put(parent, keys[len(keys)-1], node)
	return nil
}

// Delete removes the entry at keys and reports whether there was one.
func Delete(mapping *yaml.Node, keys ...string) bool {
	if len(keys) == 0 {
		return false
	}
	parent := Get(mapping, keys[:len(keys)-1]...)
	if parent == nil || parent.Kind != yaml.MappingNode {
		return false
	}

	key := keys[len(keys)-1]
	for index := 0; index+1 < len(parent.Content); index += 2 {
		if parent.Content[index].Value == key {
			parent.Content = append(parent.Content[:index], parent.Content[index+2:]...)
			return true
		}
	}
	return false
}

// Node renders a Go value as the YAML nodes that spell it.
func Node(value any) (*yaml.Node, error) {
	if node, ok := value.(*yaml.Node); ok {
		return node, nil
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}, nil
	}
	return document.Content[0], nil
}

// Render is one value as a whole YAML document, which is what a preview shows.
func Render(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(indent)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// IsEmpty reports a mapping or sequence holding nothing.
func IsEmpty(node *yaml.Node) bool {
	return node == nil || len(node.Content) == 0
}

func entry(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func put(mapping *yaml.Node, key string, node *yaml.Node) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content[index+1] = node
			return
		}
	}
	mapping.Content = append(mapping.Content, keyNode(key), node)
}

func ensureMapping(mapping *yaml.Node, key string) (*yaml.Node, error) {
	existing := entry(mapping, key)
	if existing == nil {
		created := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapping.Content = append(mapping.Content, keyNode(key), created)
		return created, nil
	}
	if existing.Kind == yaml.MappingNode {
		return existing, nil
	}
	if isNull(existing) {
		existing.Kind, existing.Tag, existing.Value = yaml.MappingNode, "!!map", ""
		return existing, nil
	}
	return nil, fmt.Errorf("%q is not a mapping", key)
}

func keyNode(key string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && (node.Tag == "!!null" || node.Tag == "")
}
