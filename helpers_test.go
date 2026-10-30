package siwx_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/evm"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
	"github.com/LexWebDev/siwx-go/solana"
)

func vector(t *testing.T, name string) string {
	t.Helper()

	return testvectors.Get(t, name)
}

func withContracts(checker evm.ContractChecker) []siwx.SignatureVerifier {
	return []siwx.SignatureVerifier{evm.New(checker), solana.New()}
}

type seededStore struct {
	mutex  sync.Mutex
	nonces map[string]bool
}

func newSeededStore(nonces ...string) *seededStore {
	store := &seededStore{nonces: map[string]bool{}}
	for _, nonce := range nonces {
		store.nonces[nonce] = true
	}

	return store
}

func (store *seededStore) Issue(context.Context) (string, error) {
	return "", errors.New("seeded store does not issue")
}

func (store *seededStore) Exists(_ context.Context, nonce string) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.nonces[nonce], nil
}

func (store *seededStore) Consume(_ context.Context, nonce string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if !store.nonces[nonce] {
		return siwx.ErrInvalidNonce
	}

	delete(store.nonces, nonce)

	return nil
}

func fixedClock(moment string) func() time.Time {
	return func() time.Time {
		parsed, err := time.Parse(time.RFC3339, moment)
		if err != nil {
			panic(err)
		}

		return parsed
	}
}

type countingChecker struct {
	verdict bool
	failure error
	calls   int
	chainID string
}

func (checker *countingChecker) IsValid(_ context.Context, _, _, _, chainID string) (bool, error) {
	checker.calls++
	checker.chainID = chainID

	return checker.verdict, checker.failure
}
