package treasury_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/spazzle-io/safekit/pkg/safe"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury"
	mocktreasury "github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPredictAddress(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x1111111111111111111111111111111111111111")
		predictedAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(predictedAddress, nil)

		c := treasury.NewTestClient(safeClient)

		address, err := c.PredictAddress(serverID, owner)
		require.NoError(t, err)
		require.Equal(t, address, predictedAddress)
	})

	t.Run("error predicting treasury address", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x1111111111111111111111111111111111111111")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(common.Address{}, errors.New("error predicting address"))

		c := treasury.NewTestClient(safeClient)

		address, err := c.PredictAddress(serverID, owner)
		require.Error(t, err)
		require.Empty(t, address)
	})
}

func TestDeploy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")
		predictedAddress := common.HexToAddress("0x222222222222222222222222222222222222")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			Deploy(gomock.Any(), gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(&safe.DeployResult{
				SafeAddress: predictedAddress,
			}, nil)

		c := treasury.NewTestClient(safeClient)

		result, err := c.Deploy(context.Background(), serverID, owner)
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, result.Address, predictedAddress)
	})

	t.Run("error deploying treasury", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			Deploy(gomock.Any(), gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(nil, errors.New("error deploying treasury"))

		c := treasury.NewTestClient(safeClient)

		result, err := c.Deploy(context.Background(), serverID, owner)
		require.Error(t, err)
		require.Empty(t, result)
	})
}

func TestIsDeployed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")
		predictedAddress := common.HexToAddress("0x222222222222222222222222222222222")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(predictedAddress, nil)

		safeClient.EXPECT().
			IsDeployed(gomock.Any(), gomock.Eq(predictedAddress)).
			Times(1).
			Return(true, nil)

		c := treasury.NewTestClient(safeClient)

		deployed, err := c.IsDeployed(context.Background(), serverID, owner)
		require.NoError(t, err)
		require.True(t, deployed)
	})

	t.Run("not deployed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")
		predictedAddress := common.HexToAddress("0x222222222222222222222222222222222")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(predictedAddress, nil)

		safeClient.EXPECT().
			IsDeployed(gomock.Any(), gomock.Eq(predictedAddress)).
			Times(1).
			Return(false, nil)

		c := treasury.NewTestClient(safeClient)

		deployed, err := c.IsDeployed(context.Background(), serverID, owner)
		require.NoError(t, err)
		require.False(t, deployed)
	})

	t.Run("error predicting treasury", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(common.Address{}, errors.New("error predicting address"))

		c := treasury.NewTestClient(safeClient)

		deployed, err := c.IsDeployed(context.Background(), serverID, owner)
		require.Error(t, err)
		require.False(t, deployed)
	})

	t.Run("error checking if treasury is deployed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		serverID := uuid.New()
		owner := common.HexToAddress("0x111111111111111111111111111111111111")
		predictedAddress := common.HexToAddress("0x222222222222222222222222222222222")

		safeClient := mocktreasury.NewSafeClient(ctrl)

		safeClient.EXPECT().
			PredictAddress(gomock.Eq([]common.Address{owner}), gomock.Eq(uint8(1)), gomock.Eq(serverID[:])).
			Times(1).
			Return(predictedAddress, nil)

		safeClient.EXPECT().
			IsDeployed(gomock.Any(), gomock.Eq(predictedAddress)).
			Times(1).
			Return(false, errors.New("error checking treasury"))

		c := treasury.NewTestClient(safeClient)

		deployed, err := c.IsDeployed(context.Background(), serverID, owner)
		require.Error(t, err)
		require.False(t, deployed)
	})
}
