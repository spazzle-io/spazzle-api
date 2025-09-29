package word

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetRandomWords(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetRandomWordsRequest
		buildStubs    func(wordStore *mockwordstore.MockStore)
		checkResponse func(t *testing.T, res *pb.GetRandomWordsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetRandomWordsRequest{
				ServerId: uuid.New().String(),
				Limit:    &wrappers.Int32Value{Value: 5},
			},
			buildStubs: func(wordStore *mockwordstore.MockStore) {
				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{
						{
							Word:    "word-1",
							AddedAt: time.Now().UTC(),
						},
						{
							Word:    "word-2",
							AddedAt: time.Now().UTC(),
						},
						{
							Word:    "word-3",
							AddedAt: time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetRandomWordsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.Words)
				require.Len(t, res.Words, 3)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetRandomWordsRequest{
				ServerId: "fake-server-id",
			},
			buildStubs: func(wordStore *mockwordstore.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetRandomWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "server not found",
			req: &pb.GetRandomWordsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(wordStore *mockwordstore.MockStore) {
				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{}, wordstore.ErrServerNotfound)
			},
			checkResponse: func(t *testing.T, res *pb.GetRandomWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch server random words",
			req: &pb.GetRandomWordsRequest{
				ServerId: uuid.New().String(),
				Limit:    &wrappers.Int32Value{Value: 5},
			},
			buildStubs: func(wordStore *mockwordstore.MockStore) {
				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{}, errors.New("could not fetch words"))
			},
			checkResponse: func(t *testing.T, res *pb.GetRandomWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)
			authService := mockservices.NewMockAuthGrpcService(ctrl)
			mockWordStore := mockwordstore.NewMockStore(ctrl)

			tc.buildStubs(mockWordStore)

			serverHandler := newTestHandler(store, cache, authService)
			serverHandler.wordStore = mockWordStore

			res, err := serverHandler.GetRandomWords(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
