package siwx

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"

	"github.com/mr-tron/base58"
)

// SolanaVerifier verifies ed25519 signatures encoded in base58 or base64.
type SolanaVerifier struct{}

// NewSolanaVerifier returns a verifier.
func NewSolanaVerifier() *SolanaVerifier {
	return &SolanaVerifier{}
}

// Namespace returns solana.
func (verifier *SolanaVerifier) Namespace() string {
	return "solana"
}

// Verify reports whether signature belongs to the message address.
func (verifier *SolanaVerifier) Verify(_ context.Context, message *Message, signature string) (bool, error) {
	publicKey, err := base58.Decode(message.Address)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false, nil
	}

	rawSignature := decodeSolanaSignature(signature)
	if len(rawSignature) != ed25519.SignatureSize {
		return false, nil
	}

	return ed25519.Verify(publicKey, []byte(message.Raw), rawSignature), nil
}

// NormaliseAddress returns the address unchanged, because base58 is case-sensitive.
func (verifier *SolanaVerifier) NormaliseAddress(address string) string {
	return address
}

func decodeSolanaSignature(signature string) []byte {
	decoded, err := base58.Decode(signature)
	if err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded
	}

	decoded, err = base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil
	}

	return decoded
}
