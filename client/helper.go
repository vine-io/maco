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

package client

import (
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiErr "github.com/vine-io/maco/api/errors"
)

func parse(err error) error {
	return apiErr.Parse(err)
}

func isUnavailable(err error) bool {
	return err == io.EOF ||
		errors.Is(err, context.Canceled) ||
		status.Code(err) == codes.Unavailable
}

func retryInterval(attempts int) time.Duration {
	if attempts <= 0 {
		return time.Millisecond * 100
	}
	if attempts > 5 {
		return time.Minute
	}
	return time.Duration(attempts*10) * time.Second
}
