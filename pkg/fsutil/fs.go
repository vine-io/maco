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

package fsutil

import (
	"os"
)

// LoadDir checks specified paths and creates all paths if it does not exist
func LoadDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

// FileExists checks specified pathname does exist
func FileExists(path string) bool {
	stat, _ := os.Stat(path)
	return stat != nil
}
