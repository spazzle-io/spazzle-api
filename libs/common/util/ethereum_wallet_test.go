package util

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func derivePublicKeyFromPrivateKey(privateKeyHex string) (string, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", err
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return "", err
	}

	publicKeyBytes := append(privateKey.X.Bytes(), privateKey.Y.Bytes()...)
	uncompressedPublicKeyHex := hex.EncodeToString(publicKeyBytes)

	return uncompressedPublicKeyHex, nil
}

func TestNewEthereumWallet(t *testing.T) {
	testEthereumWallet, err := NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, testEthereumWallet)

	require.Equal(t, strings.ToLower(testEthereumWallet.PublicKeyHash), strings.ToLower(testEthereumWallet.Address))

	derivedPublicKey, err := derivePublicKeyFromPrivateKey(testEthereumWallet.PrivateKey)
	require.NoError(t, err)
	require.Equal(t, derivedPublicKey, testEthereumWallet.PublicKey)
}

func TestSignMessageEthereum(t *testing.T) {
	ethereumWallet, err := NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, ethereumWallet)

	signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, gofakeit.Phrase())
	require.NoError(t, err)
	require.NotEmpty(t, signature)
	require.Equal(t, signature[:2], "0x")
	require.Len(t, signature, 132)
}

func TestIsEthereumSignatureValid(t *testing.T) {
	ethereumWallet, walletCreationErr := NewEthereumWallet()
	require.NoError(t, walletCreationErr)
	require.NotEmpty(t, ethereumWallet)

	testCases := []struct {
		name                        string
		message                     string
		generateWalletAddressHex    func() string
		generateMessageAndSignature func() (string, string)
		isSignatureValid            bool
	}{
		{
			name: "Success",
			generateWalletAddressHex: func() string {
				return ethereumWallet.Address
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()
				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return message, signature
			},
			isSignatureValid: true,
		},
		{
			name: "Success: wallet address missing 0x prefix",
			generateWalletAddressHex: func() string {
				return strings.TrimPrefix(ethereumWallet.Address, "0x")
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()
				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return message, signature
			},
			isSignatureValid: true,
		},
		{
			name: "Success: signature missing 0x prefix",
			generateWalletAddressHex: func() string {
				return ethereumWallet.Address
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()

				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return message, strings.TrimPrefix(signature, "0x")
			},
			isSignatureValid: true,
		},
		{
			name: "Fail: invalid signature: wrong length",
			generateWalletAddressHex: func() string {
				return ethereumWallet.Address
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()

				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return message, signature[:len(signature)-1]
			},
			isSignatureValid: false,
		},
		{
			name: "Fail: invalid wallet address",
			generateWalletAddressHex: func() string {
				wallet, err := NewEthereumWallet()
				require.NoError(t, err)
				require.NotEmpty(t, wallet)

				return fmt.Sprintf("%sz", wallet.Address[:len(wallet.Address)-1])
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()

				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return message, signature
			},
			isSignatureValid: false,
		},
		{
			name: "Fail: invalid message",
			generateWalletAddressHex: func() string {
				wallet, err := NewEthereumWallet()
				require.NoError(t, err)
				require.NotEmpty(t, wallet)

				return wallet.Address
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()

				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return gofakeit.Phrase(), signature[:len(signature)-1]
			},
			isSignatureValid: false,
		},
		{
			name: "Fail: invalid signature",
			generateWalletAddressHex: func() string {
				wallet, err := NewEthereumWallet()
				require.NoError(t, err)
				require.NotEmpty(t, wallet)

				return wallet.Address
			},
			generateMessageAndSignature: func() (string, string) {
				message := gofakeit.Phrase()

				signature, err := SignMessageEthereum(ethereumWallet.PrivateKey, message)
				require.NoError(t, err)
				require.NotEmpty(t, signature)

				return gofakeit.Phrase(), fmt.Sprintf("%sz", signature[:len(signature)-1])
			},
			isSignatureValid: false,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			message, signature := tc.generateMessageAndSignature()
			isSignatureValid, err := IsEthereumSignatureValid(tc.generateWalletAddressHex(), message, signature)

			if tc.isSignatureValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			require.Equal(t, isSignatureValid, tc.isSignatureValid)
		})
	}
}

func TestParseTestWalletAddress(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		address common.Address
		wantErr bool
	}{
		{
			name:    "valid",
			input:   "0x718031233DE4cBeD7415c5088e0e23CFbbA2F299",
			address: common.HexToAddress("0x718031233DE4cBeD7415c5088e0e23CFbbA2F299"),
			wantErr: false,
		},
		{
			name:    "invalid",
			input:   "0x718031233",
			address: common.Address{},
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			address: common.Address{},
			wantErr: true,
		},
		{
			name:    "missing 0x prefix",
			input:   "718031233DE4cBeD7415c5088e0e23CFbbA2F299",
			address: common.HexToAddress("0x718031233DE4cBeD7415c5088e0e23CFbbA2F299"),
			wantErr: false,
		},
		{
			name:    "zero address",
			input:   "0x0000000000000000000000000000000000000000",
			address: common.Address{},
			wantErr: false,
		},
		{
			name:    "invalid hex characters",
			input:   "0xZZZZ31233DE4cBeD7415c5088e0e23CFbbA2F299",
			address: common.Address{},
			wantErr: true,
		},
		{
			name:    "too long",
			input:   "0x718031233DE4cBeD7415c5088e0e23CFbbA2F299FF",
			address: common.Address{},
			wantErr: true,
		},
		{
			name:    "checksummed address",
			input:   "0x718031233DE4cBeD7415c5088e0e23CFbbA2F299",
			address: common.HexToAddress("0x718031233de4cbed7415c5088e0e23cfbba2f299"),
			wantErr: false,
		},
		{
			name:    "lowercase address",
			input:   "0x718031233de4cbed7415c5088e0e23cfbba2f299",
			address: common.HexToAddress("0x718031233DE4cBeD7415c5088e0e23CFbbA2F299"),
			wantErr: false,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			gotAddress, err := ParseWalletAddress(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.address, gotAddress)
		})
	}
}
