// Copyright 2015 The appc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

var (
	ErrACStringTooLong       = ACStringError("ACString exceeds maximum number of characters")
	ErrInvalidCharInACString = ACStringError("ACString must contain only printable unicode characters")
)

const (
	// Max number of characters (not bytes)
	ACStringMaxChars = 255
)

// ACString (an App-Container String) is a format used in image labels values of the App Container Standard.
// An ACString MUST be UTF-8 encoded and is restricted to all unicode printable
// characters. Such characters include letters, marks, numbers, punctuation,
// symbols, and the ASCII space character, from unicode categories L, M, N, P,
// S and the ASCII space character.
type ACString string

func (n ACString) String() string {
	return string(n)
}

// Set sets the ACString to the given value, if it is valid; if not,
// an error is returned.
func (n *ACString) Set(s string) error {
	nn, err := NewACString(s)
	if err == nil {
		*n = *nn
	}
	return err
}

// Equals checks whether a given ACString is equal to this one.
func (n ACString) Equals(o ACString) bool {
	return n == o
}

// Empty returns a boolean indicating whether this ACString is empty.
func (n ACString) Empty() bool {
	return n.String() == ""
}

// NewACString generates a new ACString from a string. If the given string is
// not a valid ACString, nil and an error are returned.
func NewACString(s string) (*ACString, error) {
	n := ACString(s)
	if err := n.assertValid(); err != nil {
		return nil, err
	}
	return &n, nil
}

// MustACString generates a new ACString from a string, If the given string is
// not a valid ACString, it panics.
func MustACString(s string) *ACString {
	n, err := NewACString(s)
	if err != nil {
		panic(err)
	}
	return n
}

func (n ACString) assertValid() error {
	s := string(n)
	if utf8.RuneCountInString(s) > ACStringMaxChars {
		return ErrACStringTooLong
	}
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return ErrInvalidCharInACString
		}
	}
	return nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (n *ACString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	nn, err := NewACString(s)
	if err != nil {
		return err
	}
	*n = *nn
	return nil
}

// MarshalJSON implements the json.Marshaler interface
func (n ACString) MarshalJSON() ([]byte, error) {
	if err := n.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(n.String())
}
