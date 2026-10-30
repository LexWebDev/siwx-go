// Package testvectors exposes the signature vectors shared with the PHP package.
package testvectors

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed vectors.json
var content []byte

//go:embed parser-corpus.json
var corpus []byte

// CorpusCase is a message with the parse result shared with the PHP package.
type CorpusCase struct {
	Raw      string         `json:"raw"`
	Expected map[string]any `json:"expected"`
}

// Corpus returns the shared parser cases.
func Corpus(t testing.TB) []CorpusCase {
	t.Helper()

	var cases []CorpusCase
	if err := json.Unmarshal(corpus, &cases); err != nil {
		t.Fatal(err)
	}

	return cases
}

// Get returns a vector by name and fails the test if it is unknown.
func Get(t testing.TB, name string) string {
	t.Helper()

	var vectors map[string]string
	if err := json.Unmarshal(content, &vectors); err != nil {
		t.Fatal(err)
	}

	value, found := vectors[name]
	if !found {
		t.Fatalf("unknown vector %s", name)
	}

	return value
}
