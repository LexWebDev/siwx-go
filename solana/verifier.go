// Package solana verifies ed25519 signatures for the solana namespace.
package solana

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"

	"github.com/mr-tron/base58"

	siwx "github.com/LexWebDev/siwx-go"
)

// Verifier verifies signatures encoded in base58 or base64.
type Verifier struct{}

// New returns a verifier.
func New() *Verifier {
	return &Verifier{}
}

// Namespace returns solana.
func (verifier *Verifier) Namespace() string {
	return "solana"
}

// Verify reports whether signature belongs to the message address.
func (verifier *Verifier) Verify(_ context.Context, message *siwx.Message, signature string) (bool, error) {
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
func (verifier *Verifier) NormaliseAddress(address string) string {
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
