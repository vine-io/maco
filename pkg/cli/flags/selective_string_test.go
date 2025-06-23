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
	"testing"
)

func TestSelectiveStringValue(t *testing.T) {
	tests := []struct {
		vals []string

		val  string
		pass bool
	}{
		// known values
		{[]string{"abc", "def"}, "abc", true},
		{[]string{"on", "off", "false"}, "on", true},

		// unrecognized values
		{[]string{"abc", "def"}, "ghi", false},
		{[]string{"on", "off"}, "", false},
	}
	for i, tt := range tests {
		sf := NewSelectiveStringValue(tt.vals...)
		if sf.v != tt.vals[0] {
			t.Errorf("#%d: want default val=%v,but got %v", i, tt.vals[0], sf.v)
		}
		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}

func TestSelectiveStringsValue(t *testing.T) {
	tests := []struct {
		vals []string

		val  string
		pass bool
	}{
		{[]string{"abc", "def"}, "abc", true},
		{[]string{"abc", "def"}, "abc,def", true},
		{[]string{"abc", "def"}, "abc, def", false},
		{[]string{"on", "off", "false"}, "on,false", true},
		{[]string{"abc", "def"}, "ghi", false},
		{[]string{"on", "off"}, "", false},
		{[]string{"a", "b", "c", "d", "e"}, "a,c,e", true},
	}
	for i, tt := range tests {
		sf := NewSelectiveStringsValue(tt.vals...)
		err := sf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}
