// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func normalizeFilename(fn string) string {
	abs, err := filepath.Abs(fn)
	if err != nil {
		return fn
	}
	base, err := os.Getwd()
	if err != nil {
		return abs
	}
	ref, err := filepath.Rel(base, abs)
	if err != nil {
		return abs
	}
	return ref
}

func refName(filename string) string {
	n := strings.Split(filepath.Base(filename), ".")
	return strings.Join(n[:len(n)-1], ".")
}

func addField(fields []*Field, field *Field) []*Field {
	if field.ID == NOTSET {
		if len(fields) > 0 {
			field.ID = fields[len(fields)-1].ID + 1
		} else {
			field.ID = 1
		}
	}
	return append(fields, field)
}

func checkrule(node *node32, rule pegRule) (*node32, error) {
	if node.pegRule != rule {
		return nil, fmt.Errorf("mismatch rule: " + rul3s[node.pegRule])
	}
	return node.up, nil
}

func SplitStrings(s string) []string {
	ss := []string{}
	for _, t := range strings.Split(s, ",") {
		if t != "" {
			ss = append(ss, t)
		}
	}
	return ss
}

func fileNameWithoutExt(path string) string {
	fileName := filepath.Base(path)
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}

func NewStringSet(ss ...string) *StringSet {
	s := &StringSet{s: make(map[string]struct{})}
	s.Add(ss...)
	return s
}

type StringSet struct {
	s map[string]struct{}
}

func (set *StringSet) Add(ss ...string) {
	for _, s := range ss {
		set.s[s] = struct{}{}
	}
}

func (set *StringSet) Contains(s string) bool {
	_, ok := set.s[s]
	// fmt.Printf("StringSet.Contains %s, ok: %v\n", s, ok)
	return ok
}

func (set *StringSet) ToSlice() []string {
	ss := []string{}
	for s := range set.s {
		ss = append(ss, s)
	}

	sort.Slice(ss, func(i, j int) bool {
		return strings.ToLower(ss[i]) < strings.ToLower(ss[j])
	})
	return ss
}

func (set *StringSet) Empty() bool {
	return len(set.s) == 0
}

func (set *StringSet) String() string {
	ss := []string{}
	for s := range set.s {
		ss = append(ss, s)
	}
	return fmt.Sprintf("%#v", ss)
}
