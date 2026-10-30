package solana_test

import (
	"context"
	"strings"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
	"github.com/LexWebDev/siwx-go/solana"
)

func parse(t *testing.T, raw string) *siwx.Message {
	t.Helper()

	message, err := siwx.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	return message
}

func verifies(message *siwx.Message, signature string) bool {
	signer, err := solana.New().Verify(context.Background(), message, signature)

	return signer != siwx.SignerNone && err == nil
}

func TestLeavesAddressCasingAlone(t *testing.T) {
	address := testvectors.Get(t, "SOLANA_SIGNER")

	if normalised := solana.New().NormaliseAddress(address); normalised != address {
		t.Fatalf("address: %s", normalised)
	}
}

func TestRejectsMalformedInputWithoutPanicking(t *testing.T) {
	message := parse(t, testvectors.Get(t, "SOLANA_MESSAGE"))
	shortKey := parse(t, strings.Replace(testvectors.Get(t, "SOLANA_MESSAGE"), testvectors.Get(t, "SOLANA_SIGNER"), "abc", 1))
	invalidKey := parse(t, strings.Replace(testvectors.Get(t, "SOLANA_MESSAGE"), testvectors.Get(t, "SOLANA_SIGNER"), "0OIl", 1))

	cases := map[string]struct {
		message   *siwx.Message
		signature string
	}{
		"empty signature":    {message, ""},
		"short signature":    {message, "abc"},
		"not an encoding":    {message, "@@@"},
		"short public key":   {shortKey, testvectors.Get(t, "SOLANA_SIG_BASE58")},
		"invalid public key": {invalidKey, testvectors.Get(t, "SOLANA_SIG_BASE58")},
		"foreign signature":  {parse(t, testvectors.Get(t, "PHANTOM_MESSAGE")), testvectors.Get(t, "SOLANA_SIG_BASE58")},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if verifies(testCase.message, testCase.signature) {
				t.Fatal("accepted")
			}
		})
	}
}

func TestPrefersBase58OverBase64(t *testing.T) {
	signature := testvectors.Get(t, "PHANTOM_SIG")

	if len(signature) != 88 {
		t.Fatalf("the regression needs an 88 character signature, got %d", len(signature))
	}

	if !verifies(parse(t, testvectors.Get(t, "PHANTOM_MESSAGE")), signature) {
		t.Fatal("rejected")
	}
}
