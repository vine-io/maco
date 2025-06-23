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

func TestNewUniqueURLsWithExceptions(t *testing.T) {
	tests := []struct {
		s         string
		exp       map[string]struct{}
		rs        string
		exception string
	}{
		{ // non-URL but allowed by exception
			s:         "*",
			exp:       map[string]struct{}{"*": {}},
			rs:        "*",
			exception: "*",
		},
		{
			s:         "",
			exp:       map[string]struct{}{},
			rs:        "",
			exception: "*",
		},
		{
			s:         "https://1.2.3.4:8080",
			exp:       map[string]struct{}{"https://1.2.3.4:8080": {}},
			rs:        "https://1.2.3.4:8080",
			exception: "*",
		},
		{
			s:         "https://1.2.3.4:8080,https://1.2.3.4:8080",
			exp:       map[string]struct{}{"https://1.2.3.4:8080": {}},
			rs:        "https://1.2.3.4:8080",
			exception: "*",
		},
		{
			s:         "http://10.1.1.1:80",
			exp:       map[string]struct{}{"http://10.1.1.1:80": {}},
			rs:        "http://10.1.1.1:80",
			exception: "*",
		},
		{
			s:         "http://localhost:80",
			exp:       map[string]struct{}{"http://localhost:80": {}},
			rs:        "http://localhost:80",
			exception: "*",
		},
		{
			s:         "http://:80",
			exp:       map[string]struct{}{"http://:80": {}},
			rs:        "http://:80",
			exception: "*",
		},
		{
			s:         "https://localhost:5,https://localhost:3",
			exp:       map[string]struct{}{"https://localhost:3": {}, "https://localhost:5": {}},
			rs:        "https://localhost:3,https://localhost:5",
			exception: "*",
		},
		{
			s:         "http://localhost:5,https://localhost:3",
			exp:       map[string]struct{}{"https://localhost:3": {}, "http://localhost:5": {}},
			rs:        "http://localhost:5,https://localhost:3",
			exception: "*",
		},
	}
	for i := range tests {
		uv := NewUniqueURLsWithExceptions(tests[i].s, tests[i].exception)
		if !reflect.DeepEqual(tests[i].exp, uv.Values) {
			t.Fatalf("#%d: expected %+v, got %+v", i, tests[i].exp, uv.Values)
		}
		if uv.String() != tests[i].rs {
			t.Fatalf("#%d: expected %q, got %q", i, tests[i].rs, uv.String())
		}
	}
}
