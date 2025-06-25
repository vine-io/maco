/*
Copyright 2025 The maco Authors

This program is offered under a commercial and under the AGPL license.
For AGPL licensing, see below.

AGPL licensing:
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package flags

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

// UniqueStringsValue wraps a list of unique strings.
// The values are set in order.
type UniqueStringsValue struct {
	Values map[string]struct{}
}

func (us *UniqueStringsValue) Type() string {
	return "uniqueString"
}

// Set parses a command line set of strings, separated by comma.
// Implements "flag.Value" interface.
// The values are set in order.
func (us *UniqueStringsValue) Set(s string) error {
	us.Values = make(map[string]struct{})
	for _, v := range strings.Split(s, ",") {
		us.Values[v] = struct{}{}
	}
	return nil
}

// String implements "flag.Value" interface.
func (us *UniqueStringsValue) String() string {
	return strings.Join(us.stringSlice(), ",")
}

func (us *UniqueStringsValue) stringSlice() []string {
	ss := make([]string, 0, len(us.Values))
	for v := range us.Values {
		ss = append(ss, v)
	}
	sort.Strings(ss)
	return ss
}

// NewUniqueStringsValue implements string slice as "flag.Value" interface.
// Given value is to be separated by comma.
// The values are set in order.
func NewUniqueStringsValue(s string) (us *UniqueStringsValue) {
	us = &UniqueStringsValue{Values: make(map[string]struct{})}
	if s == "" {
		return us
	}
	if err := us.Set(s); err != nil {
		panic(fmt.Sprintf("new UniqueStringsValue should never fail: %v", err))
	}
	return us
}

// UniqueStringsFromFlag returns a string slice from the flag.
func UniqueStringsFromFlag(fs *pflag.FlagSet, flagName string) []string {
	return (*fs.Lookup(flagName).Value.(*UniqueStringsValue)).stringSlice()
}

// UniqueStringsMapFromFlag returns a map of strings from the flag.
func UniqueStringsMapFromFlag(fs *pflag.FlagSet, flagName string) map[string]struct{} {
	return (*fs.Lookup(flagName).Value.(*UniqueStringsValue)).Values
}
