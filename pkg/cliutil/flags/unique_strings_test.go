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
	"reflect"
	"testing"
)

func TestNewUniqueStrings(t *testing.T) {
	tests := []struct {
		s   string
		exp map[string]struct{}
		rs  string
	}{
		{ // non-URL but allowed by exception
			s:   "*",
			exp: map[string]struct{}{"*": {}},
			rs:  "*",
		},
		{
			s:   "",
			exp: map[string]struct{}{},
			rs:  "",
		},
		{
			s:   "example.com",
			exp: map[string]struct{}{"example.com": {}},
			rs:  "example.com",
		},
		{
			s:   "localhost,localhost",
			exp: map[string]struct{}{"localhost": {}},
			rs:  "localhost",
		},
		{
			s:   "b.com,a.com",
			exp: map[string]struct{}{"a.com": {}, "b.com": {}},
			rs:  "a.com,b.com",
		},
		{
			s:   "c.com,b.com",
			exp: map[string]struct{}{"b.com": {}, "c.com": {}},
			rs:  "b.com,c.com",
		},
	}
	for i := range tests {
		uv := NewUniqueStringsValue(tests[i].s)
		if !reflect.DeepEqual(tests[i].exp, uv.Values) {
			t.Fatalf("#%d: expected %+v, got %+v", i, tests[i].exp, uv.Values)
		}
		if uv.String() != tests[i].rs {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].rs, uv.String())
		}
	}
}
