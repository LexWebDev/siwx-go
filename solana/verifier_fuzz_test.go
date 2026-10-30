package solana_test

import (
	"context"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
	"github.com/LexWebDev/siwx-go/solana"
)

func FuzzVerify(f *testing.F) {
	f.Add(testvectors.Get(f, "SOLANA_SIGNER"), testvectors.Get(f, "SOLANA_SIG_BASE58"))
	f.Add(testvectors.Get(f, "PHANTOM_SIGNER"), testvectors.Get(f, "SOLANA_SIG_BASE64"))
	f.Add("", "")

	f.Fuzz(func(t *testing.T, address, signature string) {
		message := &siwx.Message{Raw: "message", Address: address}

		if signer, err := solana.New().Verify(context.Background(), message, signature); signer != siwx.SignerNone || err != nil {
			t.Fatalf("accepted %q %q: %v", address, signature, err)
		}
	})
}
