package tests

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
)

var (
	vectorsOnce sync.Once
	vectors     map[string]string
)

func vector(t *testing.T, name string) string {
	t.Helper()

	vectorsOnce.Do(func() {
		content, err := os.ReadFile("testdata/vectors.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(content, &vectors); err != nil {
			t.Fatal(err)
		}
	})

	value, found := vectors[name]
	if !found {
		t.Fatalf("unknown vector %s", name)
	}

	return value
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
	calls   int
	chainID string
}

func (checker *countingChecker) IsValid(_ context.Context, _, _, _, chainID string) bool {
	checker.calls++
	checker.chainID = chainID

	return checker.verdict
}
