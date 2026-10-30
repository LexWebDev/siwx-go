package siwx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	eip1271MagicValue = "0x1626ba7e"
	rpcTimeout        = 5 * time.Second
	wordHexLength     = 64
	signatureOffset   = 64
)

type RPCContractChecker struct {
	url    string
	urls   map[string]string
	client *http.Client
}

func NewRPCContractChecker(url string, urls map[string]string) *RPCContractChecker {
	return &RPCContractChecker{
		url:    url,
		urls:   urls,
		client: &http.Client{Timeout: rpcTimeout},
	}
}

func (checker *RPCContractChecker) IsValid(ctx context.Context, address, hash, signature, chainID string) bool {
	endpoint := checker.endpointFor(chainID)
	if endpoint == "" {
		return false
	}

	calldata, valid := eip1271Calldata(hash, signature)
	if !valid {
		return false
	}

	result, err := checker.ethCall(ctx, endpoint, address, calldata)
	if err != nil {
		return false
	}

	return strings.HasPrefix(result, eip1271MagicValue)
}

func (checker *RPCContractChecker) endpointFor(chainID string) string {
	if len(checker.urls) == 0 {
		return checker.url
	}

	return checker.urls[chainID]
}

func (checker *RPCContractChecker) ethCall(ctx context.Context, endpoint, address, calldata string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_call",
		"params":  []any{map[string]string{"to": address, "data": calldata}, "latest"},
	})
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := checker.client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("siwx: rpc answered %d", response.StatusCode)
	}

	var body struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", err
	}

	return body.Result, nil
}

func eip1271Calldata(hash, signature string) (string, bool) {
	cleanSignature := strings.TrimPrefix(signature, "0x")

	decoded, err := hex.DecodeString(cleanSignature)
	if err != nil {
		return "", false
	}

	padding := (wordHexLength - len(cleanSignature)%wordHexLength) % wordHexLength

	return eip1271MagicValue +
		fmt.Sprintf("%0*s", wordHexLength, strings.TrimPrefix(hash, "0x")) +
		fmt.Sprintf("%0*x", wordHexLength, signatureOffset) +
		fmt.Sprintf("%0*x", wordHexLength, len(decoded)) +
		cleanSignature + strings.Repeat("0", padding), true
}
