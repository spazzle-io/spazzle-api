package handler

import (
	"errors"
	"testing"

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"

	"github.com/stretchr/testify/require"

	"buf.build/go/protovalidate"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	InternalServerError      string = "An unexpected error occurred while processing your request"
	UnauthorizedAccessError  string = "Authorization failed. Please verify your credentials and try again"
	InvalidUserIdError       string = "Invalid user id"
	InvalidServerIdError     string = "Invalid server id"
	InvalidGameIdError       string = "Invalid game id"
	ServerNotFoundError      string = "Server not found"
	InvalidAfterIdError      string = "Invalid after id"
	ServerNameInUseError     string = "Server name already in use"
	ServerAddressInUseError  string = "Server treasury address already in use"
	ServerArchivedError      string = "Server archived"
	InvalidStakePerGameError string = "Invalid stake per game"
	InvalidStreamTypeError   string = "Invalid stream type"
	InvalidGameRoleError     string = "Invalid game role"
	GameNotFoundError        string = "Game not found"
	UserStatsNotFoundError   string = "No game history found for this user"
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

func HandleServerDBError(dbError error) error {
	parsedDBError := db.ParseError(dbError)

	switch {
	// an unarchived server with the same name exists
	case parsedDBError.Code == db.UniqueViolationCode &&
		parsedDBError.ConstraintName == "servers_name_unique_unarchived_idx":
		return status.Error(codes.AlreadyExists, ServerNameInUseError)

	// a server with the same treasury address exists
	case parsedDBError.Code == db.UniqueViolationCode &&
		parsedDBError.ConstraintName == "unique_servers_server_address":
		return status.Error(codes.AlreadyExists, ServerAddressInUseError)

	// no server found
	case errors.Is(dbError, db.RecordNotFoundError):
		return status.Error(codes.NotFound, ServerNotFoundError)

	// unknown/internal error
	default:
		return status.Error(codes.Internal, InternalServerError)
	}
}
