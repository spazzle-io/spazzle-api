package util

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

type EthereumWallet struct {
	PublicKey     string `json:"public_key"`
	PrivateKey    string `json:"private_key"`
	Address       string `json:"address"`
	PublicKeyHash string `json:"public_key_hash"`
}

func NewEthereumWallet() (*EthereumWallet, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	ethereumWallet := &EthereumWallet{}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	ethereumWallet.PrivateKey = hexutil.Encode(privateKeyBytes)[2:]

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	ethereumWallet.PublicKey = hexutil.Encode(publicKeyBytes)[4:]

	ethereumWallet.Address = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	hash := sha3.NewLegacyKeccak256()
	hash.Write(publicKeyBytes[1:])
	ethereumWallet.PublicKeyHash = hexutil.Encode(hash.Sum(nil)[12:])

	return ethereumWallet, nil
}

func SignMessageEthereum(privateKeyHex string, message string) (string, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("could not parse secp256k1 private key: %w", err)
	}

	messageWithPrefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%s%s", fmt.Sprint(len(message)), message)
	hash := crypto.Keccak256Hash([]byte(messageWithPrefix))

	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("could not sign message: %w", err)
	}

	signatureHex := "0x" + hex.EncodeToString(signature)

	return signatureHex, nil
}

func IsEthereumSignatureValid(walletAddressHex string, message string, signature string) (bool, error) {
	if !strings.HasPrefix(walletAddressHex, "0x") {
		walletAddressHex = fmt.Sprintf("0x%s", walletAddressHex)
	}

	if !strings.HasPrefix(signature, "0x") {
		signature = fmt.Sprintf("0x%s", signature)
	}

	if len(signature) != 132 {
		return false, fmt.Errorf("invalid signature length: %d", len(signature))
	}

	signatureBytes, err := hexutil.Decode(signature)
	if err != nil {
		return false, fmt.Errorf("could not decode signature: %w", err)
	}

	messageHash := accounts.TextHash([]byte(message))
	if signatureBytes[crypto.RecoveryIDOffset] == 27 || signatureBytes[crypto.RecoveryIDOffset] == 28 {
		signatureBytes[crypto.RecoveryIDOffset] -= 27
	}

	recoveredPublicKey, err := crypto.SigToPub(messageHash, signatureBytes)
	if err != nil {
		return false, fmt.Errorf("could not recover public key: %w", err)
	}

	recoveredWalletAddress := crypto.PubkeyToAddress(*recoveredPublicKey)

	if !strings.EqualFold(walletAddressHex, recoveredWalletAddress.Hex()) {
		return false, fmt.Errorf("signature verification failed: address mismatch: "+
			"initial: %s recovered: %s", walletAddressHex, recoveredWalletAddress)
	}

	return true, nil
}
