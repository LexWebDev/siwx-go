package siwx

import (
	"context"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

const (
	signatureBytes     = 65
	recoveryOffset     = 27
	personalSignPrefix = "\x19Ethereum Signed Message:\n"
)

var evmAddressPattern = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)

// EIP155Verifier verifies EVM signatures through EIP-191 recovery and, optionally, EIP-1271.
type EIP155Verifier struct {
	contracts ContractSignatureChecker
}

// NewEIP155Verifier returns a verifier. A nil contracts disables EIP-1271.
func NewEIP155Verifier(contracts ContractSignatureChecker) *EIP155Verifier {
	return &EIP155Verifier{contracts: contracts}
}

// Namespace returns eip155.
func (verifier *EIP155Verifier) Namespace() string {
	return "eip155"
}

// Verify reports whether signature belongs to the message address.
func (verifier *EIP155Verifier) Verify(ctx context.Context, message *Message, signature string) (bool, error) {
	if !evmAddressPattern.MatchString(message.Address) {
		return false, nil
	}

	recovered, found := recoverAddress(message.Raw, signature)
	if found && recovered == verifier.NormaliseAddress(message.Address) {
		return true, nil
	}

	if verifier.contracts == nil {
		return false, nil
	}

	hash := "0x" + hex.EncodeToString(personalMessageHash(message.Raw))

	return verifier.contracts.IsValid(ctx, message.Address, hash, signature, message.ChainID)
}

// NormaliseAddress lowercases the address.
func (verifier *EIP155Verifier) NormaliseAddress(address string) string {
	return strings.ToLower(address)
}

func personalMessageHash(message string) []byte {
	return keccak256([]byte(personalSignPrefix + strconv.Itoa(len(message)) + message))
}

func recoverAddress(message, signature string) (string, bool) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil || len(decoded) != signatureBytes {
		return "", false
	}

	recoveryID := decoded[signatureBytes-1]
	if recoveryID >= recoveryOffset {
		recoveryID -= recoveryOffset
	}
	if recoveryID > 1 {
		return "", false
	}

	compact := make([]byte, 0, signatureBytes)
	compact = append(compact, recoveryOffset+recoveryID)
	compact = append(compact, decoded[:signatureBytes-1]...)

	publicKey, _, err := ecdsa.RecoverCompact(compact, personalMessageHash(message))
	if err != nil {
		return "", false
	}

	digest := keccak256(publicKey.SerializeUncompressed()[1:])

	return "0x" + hex.EncodeToString(digest[len(digest)-20:]), true
}

func keccak256(data []byte) []byte {
	digest := sha3.NewLegacyKeccak256()
	digest.Write(data)

	return digest.Sum(nil)
}
