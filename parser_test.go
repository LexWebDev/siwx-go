package siwx_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
)

const (
	statementText = "Sign in with Ethereum to the application."
	recapSentence = "I further authorize the stated URI to perform the following actions on my behalf: " +
		"(1) 'request': 'personal_sign' for 'eip155'."
	recapResource = "urn:recap:eyJhdHQiOnsiZWlwMTU1Ijp7InJlcXVlc3QvcGVyc29uYWxfc2lnbiI6W3t9XX19fQ"
	genesisChain  = "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp"
)

func parse(t *testing.T, raw string) *siwx.Message {
	t.Helper()

	message, err := siwx.Parse(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	return message
}

func TestParsesAnEvmMessageWithANumericChainID(t *testing.T) {
	message := parse(t, vector(t, "EVM_MESSAGE"))

	if message.Domain != "dapp.expert" || message.Network != "Ethereum" {
		t.Fatalf("header: %+v", message)
	}
	if message.Address != vector(t, "EVM_SIGNER") || message.URI != "https://dapp.expert" {
		t.Fatalf("address or uri: %+v", message)
	}
	if message.Statement != statementText || message.Version != "1" {
		t.Fatalf("statement or version: %+v", message)
	}
	if message.Namespace != "eip155" || message.ChainID != "eip155:1" {
		t.Fatalf("chain: %+v", message)
	}
	if message.Nonce != "8Kc2fQ1pXm9Zr4Lt" {
		t.Fatalf("nonce: %s", message.Nonce)
	}
	if !message.IssuedAt.Equal(time.Date(2026, 7, 30, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("issued at: %s", message.IssuedAt)
	}
}

func TestParsesASolanaMessageWithACaip2ChainID(t *testing.T) {
	message := parse(t, vector(t, "SOLANA_MESSAGE"))

	if message.Network != "Solana" || message.Address != vector(t, "SOLANA_SIGNER") {
		t.Fatalf("header: %+v", message)
	}
	if message.Namespace != "solana" || message.ChainID != genesisChain {
		t.Fatalf("chain: %+v", message)
	}
}

func TestAcceptsAnEip155ChainIDAlreadyInCaip2Form(t *testing.T) {
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), "Chain ID: 1", "Chain ID: eip155:137", 1)

	message := parse(t, raw)

	if message.Namespace != "eip155" || message.ChainID != "eip155:137" {
		t.Fatalf("chain: %+v", message)
	}
}

func TestParsesOptionalExpirationAndNotBefore(t *testing.T) {
	raw := vector(t, "EVM_MESSAGE") +
		"\nExpiration Time: 2026-07-30T13:34:56.000Z\nNot Before: 2026-07-30T12:00:00.000Z"

	message := parse(t, raw)

	if message.ExpirationTime == nil || !message.ExpirationTime.Equal(time.Date(2026, 7, 30, 13, 34, 56, 0, time.UTC)) {
		t.Fatalf("expiration: %v", message.ExpirationTime)
	}
	if message.NotBefore == nil || !message.NotBefore.Equal(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("not before: %v", message.NotBefore)
	}
}

func TestParsesAMessageWithASingleBlankLineAndNoStatement(t *testing.T) {
	raw := "dapp.expert wants you to sign in with your Ethereum account:\n" +
		vector(t, "EVM_SIGNER") + "\n\n" +
		"URI: https://dapp.expert\nVersion: 1\nChain ID: 1\n" +
		"Nonce: 8Kc2fQ1pXm9Zr4Lt\nIssued At: 2026-07-30T12:34:56.000Z"

	message := parse(t, raw)

	if message.Statement != "" || message.URI != "https://dapp.expert" {
		t.Fatalf("message: %+v", message)
	}
}

func TestSeparatesTheRecapSentenceFromTheStatement(t *testing.T) {
	signed := statementText + " " + recapSentence
	raw := strings.Replace(vector(t, "EVM_RESOURCES_MESSAGE"), statementText, signed, 1)

	message := parse(t, raw)

	if message.Statement != statementText || message.SignedStatement != signed {
		t.Fatalf("statement: %+v", message)
	}
	if !reflect.DeepEqual(message.Resources, []string{recapResource}) {
		t.Fatalf("resources: %v", message.Resources)
	}
}

func TestYieldsAnEmptyStatementWhenOnlyTheRecapSentenceWasSigned(t *testing.T) {
	raw := strings.Replace(vector(t, "EVM_RESOURCES_MESSAGE"), statementText, recapSentence, 1)

	message := parse(t, raw)

	if message.Statement != "" || message.SignedStatement != recapSentence {
		t.Fatalf("statement: %+v", message)
	}
}

func TestLeavesTheStatementUntouchedWithoutARecapResource(t *testing.T) {
	signed := statementText + " " + recapSentence
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), statementText, signed, 1)

	message := parse(t, raw)

	if message.Statement != signed || len(message.Resources) != 0 {
		t.Fatalf("statement: %+v", message)
	}
}

func TestRejectsMessagesThatBreakTheGrammar(t *testing.T) {
	evm := vector(t, "EVM_MESSAGE")
	resources := vector(t, "EVM_RESOURCES_MESSAGE")

	cases := map[string]string{
		"malformed header":           "garbage\n" + vector(t, "EVM_SIGNER"),
		"empty address":              strings.Replace(evm, vector(t, "EVM_SIGNER"), "", 1),
		"empty uri":                  strings.Replace(evm, "URI: https://dapp.expert", "URI: ", 1),
		"empty version":              strings.Replace(evm, "Version: 1", "Version: ", 1),
		"stray carriage return":      strings.Replace(evm, "URI: https://dapp.expert", "URI: https://dapp.expert\r\r", 1),
		"missing nonce":              strings.Replace(evm, "Nonce: 8Kc2fQ1pXm9Zr4Lt\n", "", 1),
		"short nonce":                strings.Replace(evm, "8Kc2fQ1pXm9Zr4Lt", "abc", 1),
		"duplicate uri":              evm + "\nURI: https://dapp.expert",
		"duplicate nonce":            evm + "\nNonce: zzzz9999zzzz",
		"field after resources":      resources + "\nNonce: zzzz9999zzzz",
		"second resources section":   resources + "\nResources:\n- https://dapp.expert",
		"unknown field":              evm + "\nInjected: value",
		"resource without a section": evm + "\n- https://dapp.expert",
		"second statement line":      strings.Replace(evm, statementText, statementText+"\nSecond line.", 1),
		"relative issued at":         strings.Replace(evm, "2026-07-30T12:34:56.000Z", "now", 1),
		"relative expiration":        evm + "\nExpiration Time: +100 years",
		"date without time":          evm + "\nNot Before: 2026-07-30",
		"date without offset":        evm + "\nNot Before: 2026-07-30T12:34:56",
		"single digit hour":          evm + "\nNot Before: 2026-07-30T1:34:56Z",
		"comma fraction":             evm + "\nNot Before: 2026-07-30T12:34:56,5Z",
		"impossible date":            evm + "\nNot Before: 2026-02-31T12:34:56Z",
		"chain outside caip-2":       strings.Replace(evm, "Chain ID: 1", "Chain ID: eip155:!!!", 1),
		"chain without a reference":  strings.Replace(evm, "Chain ID: 1", "Chain ID: eip155:", 1),
		"bare chain word":            strings.Replace(evm, "Chain ID: 1", "Chain ID: mainnet", 1),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := siwx.Parse(raw)

			if !errors.Is(err, siwx.ErrInvalidMessage) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestAcceptsEveryShapeRealBuildersProduce(t *testing.T) {
	evm := vector(t, "EVM_MESSAGE")

	cases := map[string]string{
		"two blank lines without a statement": "dapp.expert wants you to sign in with your Ethereum account:\n" +
			vector(t, "EVM_SIGNER") + "\n\n\n" +
			"URI: https://dapp.expert\nVersion: 1\nChain ID: 1\n" +
			"Nonce: 8Kc2fQ1pXm9Zr4Lt\nIssued At: 2026-07-30T12:34:56.000Z",
		"empty resources section": evm + "\nResources:",
		"all optional fields in order": evm +
			"\nExpiration Time: 2026-07-30T13:34:56.000Z\nNot Before: 2026-07-30T12:00:00.000Z" +
			"\nRequest ID: req-42\nResources:\n- https://dapp.expert/a\n- urn:recap:eyJhdHQiOnt9fQ",
		"statement containing a colon": strings.Replace(evm, statementText, "Note: sign in to the application.", 1),
		"timestamp without fraction":   strings.Replace(evm, "12:34:56.000Z", "12:34:56Z", 1),
		"timestamp with offset":        strings.Replace(evm, "12:34:56.000Z", "15:34:56+03:00", 1),
		"uri with a port and a query": strings.Replace(evm, "URI: https://dapp.expert",
			"URI: https://dapp.expert:8443/login?next=/home", 1),
		"carriage returns": strings.ReplaceAll(evm, "\n", "\r\n"),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if nonce := parse(t, raw).Nonce; nonce != "8Kc2fQ1pXm9Zr4Lt" {
				t.Fatalf("nonce: %s", nonce)
			}
		})
	}
}

func TestReadsTheOptionalSchemeFromTheHeader(t *testing.T) {
	message := parse(t, "https://"+vector(t, "EVM_MESSAGE"))

	if message.Scheme != "https" || message.Domain != "dapp.expert" {
		t.Fatalf("header: %+v", message)
	}
}

func TestHasNoSchemeWhenTheHeaderCarriesNone(t *testing.T) {
	if scheme := parse(t, vector(t, "EVM_MESSAGE")).Scheme; scheme != "" {
		t.Fatalf("scheme: %s", scheme)
	}
}

func TestKeepsThePortWhenTheHeaderCarriesAScheme(t *testing.T) {
	message := parse(t, "http://"+vector(t, "PHANTOM_MESSAGE"))

	if message.Scheme != "http" || message.Domain != "localhost:3000" {
		t.Fatalf("header: %+v", message)
	}
}

func TestExposesTheRequestID(t *testing.T) {
	if requestID := parse(t, vector(t, "EVM_MESSAGE")+"\nRequest ID: req-42").RequestID; requestID != "req-42" {
		t.Fatalf("request id: %s", requestID)
	}
}

func TestReportsTheSolanaChainAsItWasSigned(t *testing.T) {
	for _, signed := range []string{genesisChain, "solana:mainnet"} {
		raw := strings.Replace(vector(t, "SOLANA_MESSAGE"), genesisChain, signed, 1)

		message := parse(t, raw)

		if message.Namespace != "solana" || message.ChainID != signed {
			t.Fatalf("chain: %+v", message)
		}
	}
}
