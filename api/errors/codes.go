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

package errors

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

func (c Code) ToHttpCode() int {
	switch c {
	case Code_Ok:
		return http.StatusOK
	case Code_Unknown:
		return http.StatusInternalServerError
	case Code_Internal:
		return http.StatusInternalServerError
	case Code_BadRequest:
		return http.StatusBadRequest
	case Code_Unauthorized:
		return http.StatusUnauthorized
	case Code_Forbidden:
		return http.StatusForbidden
	case Code_NotFound:
		return http.StatusNotFound
	case Code_Conflict:
		return http.StatusConflict
	case Code_TooManyRequests:
		return http.StatusTooManyRequests
	case Code_ClientClosed:
		return 499
	case Code_NotImplemented:
		return http.StatusNotImplemented
	case Code_Unavailable:
		return http.StatusServiceUnavailable
	case Code_GatewayTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func (c Code) ToGrpcCode() codes.Code {
	switch c {
	case Code_Ok:
		return codes.OK
	case Code_Unknown:
		return codes.Unknown
	case Code_Internal:
		return codes.Internal
	case Code_BadRequest:
		return codes.InvalidArgument
	case Code_Unauthorized:
		return codes.Unauthenticated
	case Code_Forbidden:
		return codes.PermissionDenied
	case Code_NotFound:
		return codes.NotFound
	case Code_Conflict:
		return codes.AlreadyExists
	case Code_TooManyRequests:
		return codes.ResourceExhausted
	case Code_ClientClosed:
		return codes.Canceled
	case Code_NotImplemented:
		return codes.Unimplemented
	case Code_Unavailable:
		return codes.Unavailable
	case Code_GatewayTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}

func FromHttpCode(c int) Code {
	switch c {
	case http.StatusOK:
		return Code_Ok
	case http.StatusBadRequest:
		return Code_BadRequest
	case http.StatusUnauthorized:
		return Code_Unauthorized
	case http.StatusForbidden:
		return Code_Forbidden
	case http.StatusNotFound:
		return Code_NotFound
	case http.StatusConflict:
		return Code_Conflict
	case http.StatusTooManyRequests:
		return Code_TooManyRequests
	case 499:
		return Code_ClientClosed
	case http.StatusInternalServerError:
		return Code_Internal
	case http.StatusServiceUnavailable:
		return Code_Unavailable
	case http.StatusNotImplemented:
		return Code_NotImplemented
	case http.StatusGatewayTimeout:
		return Code_GatewayTimeout
	default:
		return Code_Unknown
	}
}

func FromGrpcCode(code codes.Code) Code {
	switch code {
	case codes.OK:
		return Code_Ok
	case codes.Canceled:
		return Code_ClientClosed
	case codes.Unknown:
		return Code_Unknown
	case codes.InvalidArgument:
		return Code_BadRequest
	case codes.DeadlineExceeded:
		return Code_Unavailable
	case codes.NotFound:
		return Code_NotFound
	case codes.AlreadyExists:
		return Code_Conflict
	case codes.PermissionDenied:
		return Code_Forbidden
	case codes.ResourceExhausted:
		return Code_TooManyRequests
	case codes.FailedPrecondition:
		return Code_Forbidden
	case codes.Aborted:
		return Code_Conflict
	case codes.OutOfRange:
		return Code_BadRequest
	case codes.Unimplemented:
		return Code_NotImplemented
	case codes.Internal:
		return Code_Internal
	case codes.Unavailable:
		return Code_Unavailable
	case codes.DataLoss:
		return Code_Internal
	case codes.Unauthenticated:
		return Code_Unauthorized
	default:
		return Code_Unknown
	}
}
