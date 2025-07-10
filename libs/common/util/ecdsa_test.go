package util

import (
	"crypto/ecdsa"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testServicePublicKeyPEM  = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
	testServicePrivateKeyPEM = "MHcCAQEEINIZr7eRHNKIo+kqyLU5j8Y3mRmfn+5k2OY685DzM1MOoAoGCCqGSM49AwEHoUQDQgAEkcpsUaeko+BLe9sutR3FRCIQPBwlRU9UN2/69Q4RLb8upVzVcK+22dEJtvVzhu3bl1hgPk3HLIYPrtuLqKOQbw=="
)

func TestParsePrivateKeyFromPEM(t *testing.T) {
	testCases := []struct {
		name            string
		privateKeyPEM   string
		expectedToError bool
	}{
		{
			name:            "successfully parses private key",
			privateKeyPEM:   testServicePrivateKeyPEM,
			expectedToError: false,
		},
		{
			name:            "invalid private key PEM",
			privateKeyPEM:   "invalid PEM",
			expectedToError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, err := ParsePrivateKeyFromPEM(tc.privateKeyPEM)

			if tc.expectedToError {
				require.Error(t, err)
				require.Nil(t, privateKey)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, privateKey)
			require.IsType(t, &ecdsa.PrivateKey{}, privateKey)
		})
	}
}

func TestParsePublicKeyFromPEM(t *testing.T) {
	testCases := []struct {
		name            string
		publicKeyPEM    string
		expectedToError bool
	}{
		{
			name:            "successfully parses public key",
			publicKeyPEM:    testServicePublicKeyPEM,
			expectedToError: false,
		},
		{
			name:            "invalid public key PEM",
			publicKeyPEM:    "invalid PEM",
			expectedToError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			publicKey, err := ParsePublicKeyFromPEM(tc.publicKeyPEM)

			if tc.expectedToError {
				require.Error(t, err)
				require.Nil(t, publicKey)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, publicKey)
			require.IsType(t, &ecdsa.PublicKey{}, publicKey)
		})
	}
}

func TestECDSASignAndVerify(t *testing.T) {
	testCases := []struct {
		name             string
		messageToSign    string
		messageToVerify  string
		publicKeyPEM     string
		privateKeyPEM    string
		isSignatureValid bool
	}{
		{
			name:             "successfully signs message and verifies signature",
			messageToSign:    "test message",
			messageToVerify:  "test message",
			publicKeyPEM:     testServicePublicKeyPEM,
			privateKeyPEM:    testServicePrivateKeyPEM,
			isSignatureValid: true,
		},
		{
			name:             "invalid verification message",
			messageToSign:    "test message",
			messageToVerify:  "invalid message",
			publicKeyPEM:     testServicePublicKeyPEM,
			privateKeyPEM:    testServicePrivateKeyPEM,
			isSignatureValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			publicKey, err := ParsePublicKeyFromPEM(tc.publicKeyPEM)
			require.NoError(t, err)
			require.NotEmpty(t, publicKey)

			privateKey, err := ParsePrivateKeyFromPEM(tc.privateKeyPEM)
			require.NoError(t, err)
			require.NotEmpty(t, privateKey)

			signature, err := ECDSASign([]byte(tc.messageToSign), privateKey)
			require.NoError(t, err)
			require.NotEmpty(t, signature)

			isSignatureValid, err := ECDSAVerify([]byte(tc.messageToVerify), publicKey, signature)
			require.NoError(t, err)
			require.Equal(t, tc.isSignatureValid, isSignatureValid)
		})
	}
}
