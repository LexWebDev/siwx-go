package siwx_test

import (
	"regexp"
	"strings"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
)

var (
	fuzzedNonce = regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)
	fuzzedChain = regexp.MustCompile(`^[-a-z0-9]{3,8}:[-_a-zA-Z0-9]{1,32}$`)
)

func FuzzParse(f *testing.F) {
	for _, name := range []string{"EVM_MESSAGE", "EVM_RESOURCES_MESSAGE", "SOLANA_MESSAGE", "PHANTOM_MESSAGE"} {
		f.Add(testvectors.Get(f, name))
	}

	f.Fuzz(func(t *testing.T, raw string) {
		message, err := siwx.Parse(raw)
		if err != nil {
			return
		}

		if !fuzzedNonce.MatchString(message.Nonce) {
			t.Fatalf("nonce %q", message.Nonce)
		}
		if !fuzzedChain.MatchString(message.ChainID) || !strings.HasPrefix(message.ChainID, message.Namespace+":") {
			t.Fatalf("chain %q in namespace %q", message.ChainID, message.Namespace)
		}
		if message.Address == "" || message.URI == "" || message.Domain == "" {
			t.Fatalf("empty required field: %+v", message)
		}

		again, err := siwx.Parse(message.Raw)
		if err != nil {
			t.Fatalf("raw does not parse again: %v", err)
		}
		if again.Raw != message.Raw || again.Nonce != message.Nonce || again.ChainID != message.ChainID {
			t.Fatalf("parsing raw changed the message: %+v", again)
		}
	})
}
