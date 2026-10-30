// Package evm verifies EVM signatures for the eip155 namespace.
package evm

import (
	"context"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"

	siwx "github.com/LexWebDev/siwx-go"
)

const (
	signatureBytes     = 65
	recoveryOffset     = 27
	personalSignPrefix = "\x19Ethereum Signed Message:\n"
)

var evmAddressPattern = regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)

// ContractChecker verifies a smart contract wallet signature on the given chain.
// Returned errors can reach logs, so they must not carry secrets or remote text.
type ContractChecker interface {
	IsValid(ctx context.Context, address, hash, signature, chainID string) (bool, error)
}

// Verifier verifies signatures through EIP-191 recovery and, optionally, EIP-1271.
type Verifier struct {
	contracts ContractChecker
}

// New returns a verifier. A nil contracts disables EIP-1271.
func New(contracts ContractChecker) *Verifier {
	return &Verifier{contracts: contracts}
}

// Namespace returns eip155.
func (verifier *Verifier) Namespace() string {
	return "eip155"
}

// Verify says who signed for the message address, if anyone did.
func (verifier *Verifier) Verify(ctx context.Context, message *siwx.Message, signature string) (siwx.SignerKind, error) {
	if !evmAddressPattern.MatchString(message.Address) {
		return siwx.SignerNone, nil
	}

	recovered, found := recoverAddress(message.Raw, signature)
	if found && recovered == verifier.NormaliseAddress(message.Address) {
		return siwx.SignerAccount, nil
	}

	if verifier.contracts == nil {
		return siwx.SignerNone, nil
	}

	hash := "0x" + hex.EncodeToString(personalMessageHash(message.Raw))

	valid, err := verifier.contracts.IsValid(ctx, message.Address, hash, signature, message.ChainID)
	if err != nil || !valid {
		return siwx.SignerNone, err
	}

	return siwx.SignerContract, nil
}

// NormaliseAddress lowercases the address.
func (verifier *Verifier) NormaliseAddress(address string) string {
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
