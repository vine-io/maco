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
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	"github.com/vine-io/maco/pkg/cliutil/flags/types"
)

// UniqueURLs contains unique URLs
// with non-URL exceptions.
type UniqueURLs struct {
	Values  map[string]struct{}
	uss     []url.URL
	Allowed map[string]struct{}
}

func (us *UniqueURLs) Type() string {
	return "UniqueURLs"
}

// Set parses a command line set of URLs formatted like:
// http://127.0.0.1:2380,http://10.1.1.2:80
// Implements "flag.Value" interface.
func (us *UniqueURLs) Set(s string) error {
	if _, ok := us.Values[s]; ok {
		return nil
	}
	if _, ok := us.Allowed[s]; ok {
		us.Values[s] = struct{}{}
		return nil
	}
	ss, err := types.NewURLs(strings.Split(s, ","))
	if err != nil {
		return err
	}
	us.Values = make(map[string]struct{})
	us.uss = make([]url.URL, 0)
	for _, v := range ss {
		us.Values[v.String()] = struct{}{}
		us.uss = append(us.uss, v)
	}
	return nil
}

// String implements "flag.Value" interface.
func (us *UniqueURLs) String() string {
	all := make([]string, 0, len(us.Values))
	for u := range us.Values {
		all = append(all, u)
	}
	sort.Strings(all)
	return strings.Join(all, ",")
}

// NewUniqueURLsWithExceptions implements "url.URL" slice as flag.Value interface.
// Given value is to be separated by comma.
func NewUniqueURLsWithExceptions(s string, exceptions ...string) *UniqueURLs {
	us := &UniqueURLs{Values: make(map[string]struct{}), Allowed: make(map[string]struct{})}
	for _, v := range exceptions {
		us.Allowed[v] = struct{}{}
	}
	if s == "" {
		return us
	}
	if err := us.Set(s); err != nil {
		panic(fmt.Sprintf("new UniqueURLs should never fail: %v", err))
	}
	return us
}

// UniqueURLsFromFlag returns a slice from urls got from the flag.
func UniqueURLsFromFlag(fs *pflag.FlagSet, urlsFlagName string) []url.URL {
	return (*fs.Lookup(urlsFlagName).Value.(*UniqueURLs)).uss
}

// UniqueURLsMapFromFlag returns a map from url strings got from the flag.
func UniqueURLsMapFromFlag(fs *pflag.FlagSet, urlsFlagName string) map[string]struct{} {
	return (*fs.Lookup(urlsFlagName).Value.(*UniqueURLs)).Values
}
