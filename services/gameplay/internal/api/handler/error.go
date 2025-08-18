package handler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"buf.build/go/protovalidate"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	InternalServerError     string = "An unexpected error occurred while processing your request"
	UnauthorizedAccessError string = "Authorization failed. Please verify your credentials and try again"
	InvalidUserIdError      string = "Invalid user id"
	InvalidServerIdError    string = "Invalid server id"
	ServerNotFoundError     string = "Server not found"
	InvalidAfterIdError     string = "invalid after id"
)

func ProtovalidateViolation(protovalidateErr error) []*errdetails.BadRequest_FieldViolation {
	var violations []*errdetails.BadRequest_FieldViolation

	var validationErr *protovalidate.ValidationError
	ok := errors.As(protovalidateErr, &validationErr)
	if !ok {
		violations = append(violations, FieldViolation("", protovalidateErr))
		return violations
	}

	for _, v := range validationErr.Violations {
		violations = append(violations, FieldViolation(
			v.FieldDescriptor.JSONName(), errors.New(v.Proto.GetMessage()),
		))
	}
	return violations
}

func FieldViolation(field string, err error) *errdetails.BadRequest_FieldViolation {
	return &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: err.Error(),
	}
}

func InvalidArgumentError(violations []*errdetails.BadRequest_FieldViolation) error {
	badRequest := &errdetails.BadRequest{FieldViolations: violations}
	statusInvalid := status.New(codes.InvalidArgument, "invalid parameters")

	statusDetails, err := statusInvalid.WithDetails(badRequest)
	if err != nil {
		return statusInvalid.Err()
	}

	return statusDetails.Err()
}

func CheckInvalidRequestParams(t *testing.T, err error, expectedFieldViolations []string) {
	var violations []string

	st, ok := status.FromError(err)
	require.True(t, ok)

	details := st.Details()

	for _, detail := range details {
		br, ok := detail.(*errdetails.BadRequest)
		require.True(t, ok)

		fieldViolations := br.FieldViolations
		for _, violation := range fieldViolations {
			violations = append(violations, violation.Field)
		}
	}

	require.ElementsMatch(t, expectedFieldViolations, violations)
}
