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

package logutil

import (
	"testing"

	"go.uber.org/zap"
)

func TestLog(t *testing.T) {
	lc := NewLogConfig()
	lc.Format = "console"
	err := lc.SetupLogging()
	if err != nil {
		t.Fatal(err)
	}

	lc.GetLogger().With(zap.String("a", "b")).Debug("info message")
	lc.GetLogger().With(zap.String("a", "b")).Info("info message")
	lc.GetLogger().With(zap.String("a", "b")).Warn("info message")
	lc.GetLogger().With(zap.String("a", "b")).Error("info message")
}
