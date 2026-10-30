package siwx

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"

	"github.com/mr-tron/base58"
)

type SolanaVerifier struct{}

func NewSolanaVerifier() *SolanaVerifier {
	return &SolanaVerifier{}
}

func (verifier *SolanaVerifier) Namespace() string {
	return "solana"
}

func (verifier *SolanaVerifier) Verify(_ context.Context, message *Message, signature string) bool {
	publicKey, err := base58.Decode(message.Address)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	rawSignature := decodeSolanaSignature(signature)
	if len(rawSignature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(publicKey, []byte(message.Raw), rawSignature)
}

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
