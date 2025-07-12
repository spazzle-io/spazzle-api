package handler

import (
	"errors"

	"buf.build/go/protovalidate"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	InternalServerError        string = "An unexpected error occurred while processing your request"
	SignatureVerificationError string = "Signature verification failed. Please try again"
	UnauthorizedAccessError    string = "Authorization failed. Please verify your credentials and try again"
)

func protovalidateViolation(protovalidateErr error) []*errdetails.BadRequest_FieldViolation {
	var violations []*errdetails.BadRequest_FieldViolation

	var validationErr *protovalidate.ValidationError
	ok := errors.As(protovalidateErr, &validationErr)
	if !ok {
		violations = append(violations, fieldViolation("", protovalidateErr))
		return violations
	}

	for _, v := range validationErr.Violations {
		violations = append(violations, fieldViolation(
			v.FieldDescriptor.JSONName(), errors.New(v.Proto.GetMessage()),
		))
	}
	return violations
}

func fieldViolation(field string, err error) *errdetails.BadRequest_FieldViolation {
	return &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: err.Error(),
	}
}

func invalidArgumentError(violations []*errdetails.BadRequest_FieldViolation) error {
	badRequest := &errdetails.BadRequest{FieldViolations: violations}
	statusInvalid := status.New(codes.InvalidArgument, "invalid parameters")

	statusDetails, err := statusInvalid.WithDetails(badRequest)
	if err != nil {
		return statusInvalid.Err()
	}

	return statusDetails.Err()
}
