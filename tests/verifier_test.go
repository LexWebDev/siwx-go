package tests

import (
	"context"
	"errors"
	"strings"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
)

const (
	signedNonce   = "8Kc2fQ1pXm9Zr4Lt"
	phantomNonce  = "LZAsP6lEAHmRQuQ4"
	afterSigning  = "2026-07-30T12:40:00Z"
	afterPhantom  = "2026-07-30T14:20:00Z"
	beforeSigning = "2026-07-30T12:20:00Z"
)

func newVerifier(t *testing.T, config siwx.Config) *siwx.Verifier {
	t.Helper()

	if config.Domains == nil {
		config.Domains = []string{"dapp.expert"}
	}
	if config.Nonces == nil {
		config.Nonces = newSeededStore(signedNonce)
	}
	if config.Now == nil {
		config.Now = fixedClock(afterSigning)
	}

	verifier, err := siwx.New(config)
	if err != nil {
		t.Fatal(err)
	}

	return verifier
}

func TestVerifiesAnEvmMessage(t *testing.T) {
	session, err := newVerifier(t, siwx.Config{}).Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG"))
	if err != nil {
		t.Fatal(err)
	}

	if session.Address != strings.ToLower(vector(t, "EVM_SIGNER")) {
		t.Fatalf("address: %s", session.Address)
	}
	if session.Namespace != "eip155" || session.ChainID != "eip155:1" || session.Domain != "dapp.expert" {
		t.Fatalf("session: %+v", session)
	}
}

func TestVerifiesAOneClickAuthMessageWithResources(t *testing.T) {
	session, err := newVerifier(t, siwx.Config{}).Verify(context.Background(),
		vector(t, "EVM_RESOURCES_MESSAGE"), vector(t, "EVM_RESOURCES_SIG"))
	if err != nil {
		t.Fatal(err)
	}

	if session.Address != strings.ToLower(vector(t, "EVM_SIGNER")) {
		t.Fatalf("address: %s", session.Address)
	}
}

func TestVerifiesASolanaMessageAndKeepsAddressCasing(t *testing.T) {
	for _, signature := range []string{"SOLANA_SIG_BASE64", "SOLANA_SIG_BASE58"} {
		session, err := newVerifier(t, siwx.Config{}).Verify(context.Background(),
			vector(t, "SOLANA_MESSAGE"), vector(t, signature))
		if err != nil {
			t.Fatalf("%s: %v", signature, err)
		}

		if session.Address != vector(t, "SOLANA_SIGNER") || session.Namespace != "solana" {
			t.Fatalf("session: %+v", session)
		}
	}
}

func TestVerifiesARealCaptureFromThePhantomExtension(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{
		Domains: []string{"localhost:3000"},
		Nonces:  newSeededStore(phantomNonce),
		Now:     fixedClock(afterPhantom),
	})

	session, err := verifier.Verify(context.Background(), vector(t, "PHANTOM_MESSAGE"), vector(t, "PHANTOM_SIG"))
	if err != nil {
		t.Fatal(err)
	}

	if session.Address != vector(t, "PHANTOM_SIGNER") || session.Domain != "localhost:3000" {
		t.Fatalf("session: %+v", session)
	}
}

func TestBurnsTheNonceSoAReplayFails(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{})

	if _, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG")); err != nil {
		t.Fatal(err)
	}

	_, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG"))
	if !errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}
}

func TestKeepsTheNonceWhenTheSignatureIsWrong(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{})

	_, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_FOREIGN_SIG"))
	if !errors.Is(err, siwx.ErrInvalidSignature) {
		t.Fatalf("got %v", err)
	}

	if _, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG")); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsAMessageWhoseNonceWasSwappedAfterSigning(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{Nonces: newSeededStore("swappedNonce1234")})
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), signedNonce, "swappedNonce1234", 1)

	_, err := verifier.Verify(context.Background(), raw, vector(t, "EVM_SIG"))
	if !errors.Is(err, siwx.ErrInvalidSignature) {
		t.Fatalf("got %v", err)
	}
}

func TestRejectsWithTheExpectedCode(t *testing.T) {
	evm := vector(t, "EVM_MESSAGE")
	expired := evm + "\nExpiration Time: 2026-07-30T12:00:00.000Z"
	premature := evm + "\nNot Before: 2026-07-30T13:00:00.000Z"
	wrongVersion := strings.Replace(evm, "Version: 1", "Version: 2", 1)
	foreignURI := strings.Replace(evm, "URI: https://dapp.expert", "URI: https://evil.example", 1)
	foreignPort := strings.Replace(vector(t, "PHANTOM_MESSAGE"), "URI: http://localhost:3000", "URI: http://localhost:3001", 1)

	cases := map[string]struct {
		config siwx.Config
		raw    string
		want   error
	}{
		"domain outside the list":    {siwx.Config{Domains: []string{"other.example"}}, evm, siwx.ErrInvalidDomain},
		"empty allow list":           {siwx.Config{Domains: []string{}}, evm, siwx.ErrInvalidDomain},
		"never issued nonce":         {siwx.Config{Nonces: newSeededStore()}, evm, siwx.ErrInvalidNonce},
		"expired message":            {siwx.Config{}, expired, siwx.ErrInvalidMessage},
		"not yet valid message":      {siwx.Config{}, premature, siwx.ErrInvalidMessage},
		"issued in the future":       {siwx.Config{Now: fixedClock(beforeSigning)}, evm, siwx.ErrInvalidMessage},
		"unsupported version":        {siwx.Config{}, wrongVersion, siwx.ErrInvalidMessage},
		"uri host differs":           {siwx.Config{Domains: []string{"dapp.expert", "evil.example"}}, foreignURI, siwx.ErrInvalidDomain},
		"uri port differs":           {siwx.Config{Domains: []string{"localhost:3000", "localhost"}, Nonces: newSeededStore(phantomNonce), Now: fixedClock(afterPhantom)}, foreignPort, siwx.ErrInvalidDomain},
		"disabled namespace":         {siwx.Config{Namespaces: []string{"solana"}}, evm, siwx.ErrUnsupportedNamespace},
		"namespace without verifier": {siwx.Config{Namespaces: []string{"eip155", "bip122"}}, strings.Replace(evm, "Chain ID: 1", "Chain ID: bip122:000000000019d6689c085ae165831e93", 1), siwx.ErrUnsupportedNamespace},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := newVerifier(t, testCase.config).Verify(context.Background(), testCase.raw, vector(t, "EVM_SIG"))

			if !errors.Is(err, testCase.want) {
				t.Fatalf("got %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestToleratesClockSkewOnIssuedAt(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{Now: fixedClock("2026-07-30T12:30:00Z")})

	if _, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG")); err != nil {
		t.Fatal(err)
	}
}

func TestMatchesTheAllowListOnTheDomainWhenTheHeaderCarriesAScheme(t *testing.T) {
	_, err := newVerifier(t, siwx.Config{}).Verify(context.Background(), "https://"+vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG"))

	if !errors.Is(err, siwx.ErrInvalidSignature) {
		t.Fatalf("got %v", err)
	}
}

func TestToleratesSpacesAroundEntriesOfTheAllowList(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{Domains: []string{"other.example", " dapp.expert "}})

	if _, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG")); err != nil {
		t.Fatal(err)
	}
}

func TestRequiresANonceStore(t *testing.T) {
	if _, err := siwx.New(siwx.Config{Domains: []string{"dapp.expert"}}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestErrorsCarryTheMachineCode(t *testing.T) {
	cases := map[error]string{
		siwx.ErrInvalidMessage:       "siwx_invalid_message",
		siwx.ErrInvalidDomain:        "siwx_invalid_domain",
		siwx.ErrInvalidNonce:         "siwx_invalid_nonce",
		siwx.ErrInvalidSignature:     "siwx_invalid_signature",
		siwx.ErrUnsupportedNamespace: "siwx_unsupported_namespace",
	}

	for err, code := range cases {
		if err.Error() != code {
			t.Fatalf("got %s, want %s", err, code)
		}
	}
}

func TestFallsBackToTheContractCheckerWhenRecoveryDoesNotMatch(t *testing.T) {
	checker := &countingChecker{verdict: true}
	verifier := newVerifier(t, siwx.Config{Contracts: checker})
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), "Chain ID: 1", "Chain ID: 8453", 1)

	session, err := verifier.Verify(context.Background(), raw, vector(t, "EVM_FOREIGN_SIG"))
	if err != nil {
		t.Fatal(err)
	}

	if session.ChainID != "eip155:8453" || checker.chainID != "eip155:8453" {
		t.Fatalf("session %s, checker %s", session.ChainID, checker.chainID)
	}
}

func TestRejectsWhenTheContractCheckerDeclines(t *testing.T) {
	verifier := newVerifier(t, siwx.Config{Contracts: &countingChecker{}})

	_, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_FOREIGN_SIG"))
	if !errors.Is(err, siwx.ErrInvalidSignature) {
		t.Fatalf("got %v", err)
	}
}

func TestDoesNotCallTheCheckerWhenRecoveryAlreadyMatches(t *testing.T) {
	checker := &countingChecker{verdict: true}

	if _, err := newVerifier(t, siwx.Config{Contracts: checker}).Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG")); err != nil {
		t.Fatal(err)
	}

	if checker.calls != 0 {
		t.Fatalf("calls: %d", checker.calls)
	}
}

func TestSpendsNoRpcCallOnANonceThatWasNeverIssued(t *testing.T) {
	checker := &countingChecker{verdict: true}
	verifier := newVerifier(t, siwx.Config{Contracts: checker, Nonces: newSeededStore()})

	_, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_FOREIGN_SIG"))
	if !errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}

	if checker.calls != 0 {
		t.Fatalf("calls: %d", checker.calls)
	}
}

type failingStore struct {
	*seededStore
	failure error
}

func (store failingStore) Exists(context.Context, string) (bool, error) {
	return false, store.failure
}

func TestPassesAStoreFailureThroughUnchanged(t *testing.T) {
	failure := errors.New("connection refused")
	verifier := newVerifier(t, siwx.Config{Nonces: failingStore{newSeededStore(), failure}})

	_, err := verifier.Verify(context.Background(), vector(t, "EVM_MESSAGE"), vector(t, "EVM_SIG"))
	if !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
}

type recordingVerifier struct{ seen string }

func (verifier *recordingVerifier) Namespace() string { return "bip122" }

func (verifier *recordingVerifier) Verify(_ context.Context, message *siwx.Message, _ string) bool {
	verifier.seen = message.Address

	return true
}

func (verifier *recordingVerifier) NormaliseAddress(address string) string {
	return "normalised:" + address
}

func TestAcceptsACustomVerifierWithoutTouchingTheCore(t *testing.T) {
	custom := &recordingVerifier{}
	verifier := newVerifier(t, siwx.Config{
		Namespaces: []string{"eip155", "bip122"},
		Verifiers:  []siwx.SignatureVerifier{custom},
	})
	raw := strings.Replace(vector(t, "EVM_MESSAGE"), "Chain ID: 1", "Chain ID: bip122:000000000019d6689c085ae165831e93", 1)

	session, err := verifier.Verify(context.Background(), raw, "any")
	if err != nil {
		t.Fatal(err)
	}

	if session.Address != "normalised:"+vector(t, "EVM_SIGNER") || custom.seen != vector(t, "EVM_SIGNER") {
		t.Fatalf("session: %+v", session)
	}
}
