package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdateUser(t *testing.T) {
	userId, err := uuid.NewRandom()
	require.NoError(t, err)
	require.NotEmpty(t, userId)

	updateUserRequest := &pb.UpdateUserRequest{
		Id: userId.String(),
		GamerTag: &wrapperspb.StringValue{
			Value: gofakeit.Gamertag(),
		},
	}

	testCases := []struct {
		name          string
		req           *pb.UpdateUserRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.UpdateUserResponse, err error)
	}{
		{
			name: "success",
			req:  updateUserRequest,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, nil)

				store.EXPECT().
					UpdateUser(gomock.Any(), db.UpdateUserParams{
						UserID: userId,
						GamerTag: pgtype.Text{
							String: updateUserRequest.GetGamerTag().GetValue(),
							Valid:  true,
						},
					}).
					Times(1).
					Return(db.User{
						ID: userId,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)

				require.NotEmpty(t, res.GetUser())
				require.Equal(t, userId.String(), res.GetUser().Id)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.UpdateUserRequest{
				Id:       "",
				GamerTag: &wrapperspb.StringValue{Value: ""},
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"id"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req:  updateUserRequest,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not update user in db",
			req:  updateUserRequest,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, nil)

				store.EXPECT().
					UpdateUser(gomock.Any(), db.UpdateUserParams{
						UserID: userId,
						GamerTag: pgtype.Text{
							String: updateUserRequest.GetGamerTag().GetValue(),
							Valid:  true,
						},
					}).
					Times(1).
					Return(db.User{}, errors.New("could not update user"))
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store, deps.authService)

			h := newTestHandler(deps)

			res, err := h.UpdateUser(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
