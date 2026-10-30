package evm_test

import (
	"testing"

	"github.com/LexWebDev/siwx-go/evm"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
)

func FuzzRecoverAddress(f *testing.F) {
	f.Add(testvectors.Get(f, "EVM_MESSAGE"), testvectors.Get(f, "EVM_SIG"))
	f.Add("", "")
	f.Add("message", "0x")

	f.Fuzz(func(t *testing.T, message, signature string) {
		address, found := evm.RecoverAddress(message, signature)

		if found && len(address) != 42 {
			t.Fatalf("address %q", address)
		}
	})
}
