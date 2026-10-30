package evm_test

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/evm"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
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
	signer, err := evm.New(nil).Verify(context.Background(), message, signature)

	return signer != siwx.SignerNone && err == nil
}

func TestRecoversTheSigner(t *testing.T) {
	recovered, found := evm.RecoverAddress(testvectors.Get(t, "EVM_MESSAGE"), testvectors.Get(t, "EVM_SIG"))

	if !found || recovered != strings.ToLower(testvectors.Get(t, "EVM_SIGNER")) {
		t.Fatalf("recovered %q, found %v", recovered, found)
	}
}

func TestHashesPerEip191(t *testing.T) {
	hash := hex.EncodeToString(evm.PersonalMessageHash("hello"))

	if hash != "50b2c43fd39106bafbba0da34fc430e1f91e3c96ea2acee2bc34119f92b37750" {
		t.Fatalf("hash: %s", hash)
	}
}

func TestRejectsMalformedSignaturesWithoutPanicking(t *testing.T) {
	message := parse(t, testvectors.Get(t, "EVM_MESSAGE"))
	signature := testvectors.Get(t, "EVM_SIG")

	cases := map[string]string{
		"empty":               "",
		"too short":           "0x1234",
		"not hex":             "0x" + strings.Repeat("z", 130),
		"zero r and s":        "0x" + strings.Repeat("0", 128) + "1b",
		"recovery id too big": signature[:len(signature)-2] + "ff",
	}

	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			if verifies(message, malformed) {
				t.Fatal("accepted")
			}
		})
	}
}

func TestAcceptsASignatureWithoutThePrefixAndWithARawRecoveryID(t *testing.T) {
	message := parse(t, testvectors.Get(t, "EVM_MESSAGE"))
	signature := strings.TrimPrefix(testvectors.Get(t, "EVM_SIG"), "0x")
	rawRecoveryID := signature[:128] + "01"

	if !verifies(message, rawRecoveryID) {
		t.Fatal("rejected")
	}
}

func TestRejectsAnAddressOfTheWrongShape(t *testing.T) {
	raw := strings.Replace(testvectors.Get(t, "EVM_MESSAGE"), testvectors.Get(t, "EVM_SIGNER"), "0x1234", 1)

	if verifies(parse(t, raw), testvectors.Get(t, "EVM_SIG")) {
		t.Fatal("accepted")
	}
}

func TestLowercasesAddresses(t *testing.T) {
	if address := evm.New(nil).NormaliseAddress("0xAbC"); address != "0xabc" {
		t.Fatalf("address: %s", address)
	}
}

type fixedChecker struct {
	valid bool
	err   error
}

func (checker fixedChecker) IsValid(context.Context, string, string, string, string) (bool, error) {
	return checker.valid, checker.err
}

func TestSaysWhichKindOfSignerVerified(t *testing.T) {
	message := parse(t, testvectors.Get(t, "EVM_MESSAGE"))
	outage := errors.New("rpc down")

	cases := map[string]struct {
		checker   evm.ContractChecker
		signature string
		want      siwx.SignerKind
		failure   error
	}{
		"recovered key":       {nil, testvectors.Get(t, "EVM_SIG"), siwx.SignerAccount, nil},
		"no checker":          {nil, testvectors.Get(t, "EVM_FOREIGN_SIG"), siwx.SignerNone, nil},
		"contract accepts":    {fixedChecker{valid: true}, testvectors.Get(t, "EVM_FOREIGN_SIG"), siwx.SignerContract, nil},
		"contract declines":   {fixedChecker{}, testvectors.Get(t, "EVM_FOREIGN_SIG"), siwx.SignerNone, nil},
		"contract check down": {fixedChecker{err: outage}, testvectors.Get(t, "EVM_FOREIGN_SIG"), siwx.SignerNone, outage},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			signer, err := evm.New(testCase.checker).Verify(context.Background(), message, testCase.signature)

			if signer != testCase.want || !errors.Is(err, testCase.failure) {
				t.Fatalf("signer %v, err %v", signer, err)
			}
		})
	}
}
