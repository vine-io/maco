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
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type ValidationError interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
}

func (e *Error) WithCause(value proto.Message) *Error {
	cause, _ := anypb.New(value)
	if cause != nil {
		e.Causes = append(e.Causes, cause)
	}
	return e
}

func (e *Error) Error() string {
	data, _ := json.Marshal(e)
	return string(data)
}

func (e *Error) Err() error {
	if e.Code == Code_Ok {
		return nil
	}
	return e
}

// ToStatus converts Error to grpc Status
func (e *Error) ToStatus() *status.Status {
	s := status.New(e.Code.ToGrpcCode(), e.Detail)
	for _, cause := range e.Causes {
		rs, err := s.WithDetails(cause)
		if err != nil {
			s = rs
		}
	}
	return s
}

func New(code Code, detail string) *Error {
	return &Error{
		Code:    code,
		Message: code.String(),
		Detail:  detail,
	}
}

func Newf(code Code, format string, args ...any) *Error {
	return New(code, fmt.Sprintf(format, args...))
}

func NewOk() *Error {
	return New(Code_Ok, "")
}

func NewUnknown(detail string) *Error {
	return New(Code_Unknown, detail)
}

func NewUnknownf(format string, args ...any) *Error {
	return Newf(Code_Unknown, format, args...)
}

func NewInternal(detail string) *Error {
	return New(Code_Internal, detail)
}

func NewInternalf(format string, args ...any) *Error {
	return Newf(Code_Internal, format, args...)
}

func NewBadRequest(detail string) *Error {
	return New(Code_BadRequest, detail)
}

func NewBadRequestf(detail string, args ...any) *Error {
	return Newf(Code_BadRequest, detail, args...)
}

func NewUnauthorized(detail string) *Error {
	return New(Code_Unauthorized, detail)
}

func NewUnauthorizedf(detail string, args ...any) *Error {
	return Newf(Code_Unauthorized, detail, args...)
}

func NewForbidden(detail string) *Error {
	return New(Code_Forbidden, detail)
}

func NewForbiddenf(detail string, args ...any) *Error {
	return Newf(Code_Forbidden, detail, args...)
}

func NewNotFound(detail string) *Error {
	return New(Code_NotFound, detail)
}

func NewNotFoundf(detail string, args ...any) *Error {
	return Newf(Code_NotFound, detail, args...)
}

func NewConflict(detail string) *Error {
	return New(Code_Conflict, detail)
}

func NewConflictf(detail string, args ...any) *Error {
	return Newf(Code_Conflict, detail, args...)
}

func NewTooManyRequests(detail string) *Error {
	return New(Code_TooManyRequests, detail)
}

func NewTooManyRequestsf(detail string, args ...any) *Error {
	return Newf(Code_TooManyRequests, detail, args...)
}

func NewClientClosed(detail string) *Error {
	return New(Code_ClientClosed, detail)
}

func NewClientClosedf(detail string, args ...any) *Error {
	return Newf(Code_TooManyRequests, detail, args...)
}

func NewNotImplemented(detail string) *Error {
	return New(Code_NotImplemented, detail)
}

func NewNotImplementedf(detail string, args ...any) *Error {
	return Newf(Code_NotImplemented, detail, args...)
}

func NewUnavailable(detail string) *Error {
	return New(Code_Unavailable, detail)
}

func NewUnavailablef(detail string, args ...any) *Error {
	return Newf(Code_Unavailable, detail, args...)
}

func NewGatewayTimeout(detail string) *Error {
	return New(Code_GatewayTimeout, detail)
}

func NewGatewayTimeoutf(detail string, args ...any) *Error {
	return Newf(Code_GatewayTimeout, detail, args...)
}

func IsOk(err error) bool {
	return Parse(err).Code == Code_Ok
}

func IsUnknown(err error) bool {
	return Parse(err).Code == Code_Unknown
}

func IsInternal(err error) bool {
	return Parse(err).Code == Code_Internal
}

func IsUnauthorized(err error) bool {
	return Parse(err).Code == Code_Unauthorized
}

func IsForbidden(err error) bool {
	return Parse(err).Code == Code_Forbidden
}

func IsNotFound(err error) bool {
	return Parse(err).Code == Code_NotFound
}

func IsConflict(err error) bool {
	return Parse(err).Code == Code_Conflict
}

func IsTooManyRequests(err error) bool {
	return Parse(err).Code == Code_TooManyRequests
}

func IsClientClosed(err error) bool {
	return Parse(err).Code == Code_ClientClosed
}

func IsNotImplemented(err error) bool {
	return Parse(err).Code == Code_NotImplemented
}

func IsUnavailable(err error) bool {
	return Parse(err).Code == Code_Unavailable
}

func IsGatewayTimeout(err error) bool {
	return Parse(err).Code == Code_GatewayTimeout
}

// FromStatus converts grpc Status to Error
func FromStatus(s *status.Status) *Error {
	e := New(FromGrpcCode(s.Code()), s.Message())
	for _, detail := range s.Details() {
		value, ok := detail.(*anypb.Any)
		if ok {
			e = e.WithCause(value)
		}
	}
	return e
}

// Parse converts error to *Error
func Parse(err error) *Error {
	if err == nil {
		return nil
	}
	if v, ok := status.FromError(err); ok {
		return FromStatus(v)
	}
	switch e := err.(type) {
	case *Error:
		return e
	case ValidationError:
		return NewBadRequest(e.Reason())
	default:
		var ee *Error
		if e1 := json.Unmarshal([]byte(err.Error()), &ee); e1 == nil {
			return ee
		}

		return NewUnknown(err.Error())
	}
}
