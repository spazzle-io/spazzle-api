package util

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// ECDSASign signs the given message using the provided ECDSA private key and returns a base64-encoded signature.
func ECDSASign(message []byte, privateKey *ecdsa.PrivateKey) (string, error) {
	hash := sha256.Sum256(message)

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("error signing message: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// ECDSAVerify verifies a base64-encoded ECDSA signature against the given message and public key.
// The message is hashed with SHA-256 before verification.
func ECDSAVerify(message []byte, publicKey *ecdsa.PublicKey, signatureBase64 string) (bool, error) {
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, fmt.Errorf("error decoding signature: %w", err)
	}

	hash := sha256.Sum256(message)

	valid := ecdsa.VerifyASN1(publicKey, hash[:], signature)
	return valid, nil
}

// ParsePublicKeyFromPEM parses a PEM-encoded ECDSA public key string and returns the *ecdsa.PublicKey.
// If the input is missing the standard PEM headers, they are automatically added.
// Returns an error if the input is not a valid PEM block or does not contain an ECDSA public key.
func ParsePublicKeyFromPEM(publicKeyPEM string) (*ecdsa.PublicKey, error) {
	const pemPrefix = "-----BEGIN PUBLIC KEY-----"
	const pemSuffix = "-----END PUBLIC KEY-----"

	if !strings.HasPrefix(publicKeyPEM, pemPrefix) {
		publicKeyPEM = fmt.Sprintf("%s\n%s", pemPrefix, publicKeyPEM)
	}

	if !strings.HasSuffix(publicKeyPEM, pemSuffix) {
		publicKeyPEM = fmt.Sprintf("%s\n%s", publicKeyPEM, pemSuffix)
	}

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("error decoding PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing public key: %w", err)
	}

	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error converting to ECDSA public key")
	}

	return ecdsaPubKey, nil
}

// ParsePrivateKeyFromPEM parses a PEM-encoded ECDSA private key string and returns the *ecdsa.PrivateKey.
// If the input is missing the standard PEM headers, they are automatically added.
// Returns an error if the input is not a valid PEM block or does not contain a valid EC private key.
func ParsePrivateKeyFromPEM(privateKeyPEM string) (*ecdsa.PrivateKey, error) {
	const pemPrefix = "-----BEGIN EC PRIVATE KEY-----" // #nosec G101
	const pemSuffix = "-----END EC PRIVATE KEY-----"

	if !strings.HasPrefix(privateKeyPEM, pemPrefix) {
		privateKeyPEM = fmt.Sprintf("%s\n%s", pemPrefix, privateKeyPEM)
	}

	if !strings.HasSuffix(privateKeyPEM, pemSuffix) {
		privateKeyPEM = fmt.Sprintf("%s\n%s", privateKeyPEM, pemSuffix)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("error decoding PEM block")
	}

	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key: %w", err)
	}

	return privateKey, nil
}
