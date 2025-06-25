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
	"net/url"
	"reflect"
	"testing"
)

func TestValidateURLsValueBad(t *testing.T) {
	tests := []string{
		// bad IP specification
		":2379",
		"127.0:8080",
		"123:456",
		// bad port specification
		"127.0.0.1:foo",
		"127.0.0.1:",
		// unix sockets not supported
		"unix://",
		"unix://tmp/olive.sock",
		// bad strings
		"somewhere",
		"234#$",
		"file://foo/bar",
		"http://hello/asdf",
		"http://10.1.1.1",
	}
	for i, in := range tests {
		u := URLsValue{}
		if err := u.Set(in); err == nil {
			t.Errorf(`#%d: unexpected nil error for in=%q`, i, in)
		}
	}
}

func TestNewURLsValue(t *testing.T) {
	tests := []struct {
		s   string
		exp []url.URL
	}{
		{s: "https://1.2.3.4:8080", exp: []url.URL{{Scheme: "https", Host: "1.2.3.4:8080"}}},
		{s: "http://10.1.1.1:80", exp: []url.URL{{Scheme: "http", Host: "10.1.1.1:80"}}},
		{s: "http://localhost:80", exp: []url.URL{{Scheme: "http", Host: "localhost:80"}}},
		{s: "http://:80", exp: []url.URL{{Scheme: "http", Host: ":80"}}},
		{
			s: "http://localhost:1,https://localhost:2",
			exp: []url.URL{
				{Scheme: "http", Host: "localhost:1"},
				{Scheme: "https", Host: "localhost:2"},
			},
		},
	}
	for i := range tests {
		uu := []url.URL(*NewURLsValue(tests[i].s))
		if !reflect.DeepEqual(tests[i].exp, uu) {
			t.Fatalf("#%d: expected %+v, got %+v", i, tests[i].exp, uu)
		}
	}
}
