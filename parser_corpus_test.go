package siwx_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
)

const corpusTimeLayout = "2006-01-02T15:04:05"

func corpusTime(moment *time.Time) string {
	if moment == nil {
		return ""
	}

	return moment.UTC().Format(corpusTimeLayout)
}

func TestAgreesWithThePhpPackageOnTheSharedCorpus(t *testing.T) {
	for index, testCase := range testvectors.Corpus(t) {
		message, err := siwx.Parse(testCase.Raw)

		if testCase.Expected["ok"] == false {
			var failure *siwx.Error
			if !errors.As(err, &failure) || failure.Code != testCase.Expected["code"] {
				t.Fatalf("case %d: got %v, want %v", index, err, testCase.Expected["code"])
			}

			continue
		}

		if err != nil {
			t.Fatalf("case %d: %v", index, err)
		}

		resources := make([]any, 0, len(message.Resources))
		for _, resource := range message.Resources {
			resources = append(resources, resource)
		}

		actual := map[string]any{
			"ok": true, "raw": message.Raw, "scheme": message.Scheme, "domain": message.Domain,
			"network": message.Network, "address": message.Address, "statement": message.Statement,
			"signedStatement": message.SignedStatement, "uri": message.URI, "version": message.Version,
			"namespace": message.Namespace, "chainId": message.ChainID, "nonce": message.Nonce,
			"issuedAt": message.IssuedAt.UTC().Format(corpusTimeLayout), "expiration": corpusTime(message.ExpirationTime),
			"notBefore": corpusTime(message.NotBefore), "requestId": message.RequestID, "resources": resources,
		}

		if !reflect.DeepEqual(actual, testCase.Expected) {
			t.Fatalf("case %d:\n got %v\nwant %v", index, actual, testCase.Expected)
		}
	}
}
