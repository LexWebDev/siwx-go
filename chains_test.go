package siwx_test

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
)

func evmVerifies(message *siwx.Message, signature string) bool {
	valid, err := siwx.NewEIP155Verifier(nil).Verify(context.Background(), message, signature)

	return valid && err == nil
}

func solanaVerifies(message *siwx.Message, signature string) bool {
	valid, err := siwx.NewSolanaVerifier().Verify(context.Background(), message, signature)

	return valid && err == nil
}

func TestEip155RecoversTheSigner(t *testing.T) {
	recovered, found := siwx.RecoverAddress(vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG"))

	if !found || recovered != strings.ToLower(vector(t, "EVM_SIGNER")) {
		t.Fatalf("recovered %q, found %v", recovered, found)
	}
}

func TestEip155HashesPerEip191(t *testing.T) {
	hash := hex.EncodeToString(siwx.PersonalMessageHash("hello"))

	if hash != "50b2c43fd39106bafbba0da34fc430e1f91e3c96ea2acee2bc34119f92b37750" {
		t.Fatalf("hash: %s", hash)
	}
}

func TestEip155RejectsMalformedSignaturesWithoutPanicking(t *testing.T) {
	message := parse(t, vector(t, "EVM_MESSAGE"))
	signature := vector(t, "EVM_SIG")

	cases := map[string]string{
		"empty":               "",
		"too short":           "0x1234",
		"not hex":             "0x" + strings.Repeat("z", 130),
		"zero r and s":        "0x" + strings.Repeat("0", 128) + "1b",
		"recovery id too big": signature[:len(signature)-2] + "ff",
	}

	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			if evmVerifies(message, malformed) {
				t.Fatal("accepted")
			}
		})
	}
}

func TestEip155AcceptsASignatureWithoutThePrefixAndWithARawRecoveryID(t *testing.T) {
	message := parse(t, vector(t, "EVM_MESSAGE"))
	signature := strings.TrimPrefix(vector(t, "EVM_SIG"), "0x")
	rawRecoveryID := signature[:128] + "01"

	if !evmVerifies(message, rawRecoveryID) {
		t.Fatal("rejected")
	}
}

func TestEip155RejectsAnAddressOfTheWrongShape(t *testing.T) {
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIGNER"), "0x1234", 1)

	if evmVerifies(parse(t, raw), vector(t, "EVM_SIG")) {
		t.Fatal("accepted")
	}
}

func TestEip155LowercasesAddresses(t *testing.T) {
	if address := siwx.NewEIP155Verifier(nil).NormaliseAddress("0xAbC"); address != "0xabc" {
		t.Fatalf("address: %s", address)
	}
}

func TestSolanaLeavesAddressCasingAlone(t *testing.T) {
	address := vector(t, "SOLANA_SIGNER")

	if normalised := siwx.NewSolanaVerifier().NormaliseAddress(address); normalised != address {
		t.Fatalf("address: %s", normalised)
	}
}

func TestSolanaRejectsMalformedInputWithoutPanicking(t *testing.T) {
	message := parse(t, vector(t, "SOLANA_MESSAGE"))
	shortKey := parse(t, strings.Replace(vector(t, "SOLANA_MESSAGE"), vector(t, "SOLANA_SIGNER"), "abc", 1))
	invalidKey := parse(t, strings.Replace(vector(t, "SOLANA_MESSAGE"), vector(t, "SOLANA_SIGNER"), "0OIl", 1))

	cases := map[string]struct {
		message   *siwx.Message
		signature string
	}{
		"empty signature":    {message, ""},
		"short signature":    {message, "abc"},
		"not an encoding":    {message, "@@@"},
		"short public key":   {shortKey, vector(t, "SOLANA_SIG_BASE58")},
		"invalid public key": {invalidKey, vector(t, "SOLANA_SIG_BASE58")},
		"foreign signature":  {parse(t, vector(t, "PHANTOM_MESSAGE")), vector(t, "SOLANA_SIG_BASE58")},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if solanaVerifies(testCase.message, testCase.signature) {
				t.Fatal("accepted")
			}
		})
	}
}

func TestSolanaPrefersBase58OverBase64(t *testing.T) {
	signature := vector(t, "PHANTOM_SIG")

	if len(signature) != 88 {
		t.Fatalf("the regression needs an 88 character signature, got %d", len(signature))
	}

	if !solanaVerifies(parse(t, vector(t, "PHANTOM_MESSAGE")), signature) {
		t.Fatal("rejected")
	}
}
