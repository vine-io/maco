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
	"strings"

	"github.com/spf13/pflag"

	"github.com/vine-io/maco/pkg/cli/flags/types"
)

// URLsValue wraps "types.URLs".
type URLsValue types.URLs

func (us *URLsValue) Type() string {
	return "urlsValue"
}

// Set parses a command line set of URLs formatted like:
// http://127.0.0.1:2380,http://10.1.1.2:80
// Implements "flag.Value" interface.
func (us *URLsValue) Set(s string) error {
	ss, err := types.NewURLs(strings.Split(s, ","))
	if err != nil {
		return err
	}
	*us = URLsValue(ss)
	return nil
}

// String implements "flag.Value" interface.
func (us *URLsValue) String() string {
	all := make([]string, len(*us))
	for i, u := range *us {
		all[i] = u.String()
	}
	return strings.Join(all, ",")
}

// NewURLsValue implements "url.URL" slice as flag.Value interface.
// Given value is to be separated by comma.
func NewURLsValue(s string) *URLsValue {
	if s == "" {
		return &URLsValue{}
	}
	v := &URLsValue{}
	if err := v.Set(s); err != nil {
		panic(fmt.Sprintf("new URLsValue should never fail: %v", err))
	}
	return v
}

// URLsFromFlag returns a slices from url got from the flag.
func URLsFromFlag(fs *pflag.FlagSet, urlsFlagName string) []url.URL {
	return []url.URL(*fs.Lookup(urlsFlagName).Value.(*URLsValue))
}
