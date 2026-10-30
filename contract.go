package siwx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	eip1271MagicValue = "0x1626ba7e"
	rpcTimeout        = 5 * time.Second
	maxRPCBodyBytes   = 1 << 20
	wordHexLength     = 64
	signatureOffset   = 64
)

// RPCContractChecker verifies EIP-1271 signatures through eth_call.
type RPCContractChecker struct {
	url    string
	urls   map[string]string
	client *http.Client
}

// NewRPCContractChecker returns a checker. With urls set, only those CAIP-2 chains are
// served and url is ignored; otherwise url serves every chain.
func NewRPCContractChecker(url string, urls map[string]string) *RPCContractChecker {
	return &RPCContractChecker{
		url:    url,
		urls:   urls,
		client: &http.Client{Timeout: rpcTimeout},
	}
}

// IsValid reports whether the contract at address accepts signature. Its errors never
// contain the endpoint, which often carries an API key.
func (checker *RPCContractChecker) IsValid(ctx context.Context, address, hash, signature, chainID string) (bool, error) {
	if checker == nil {
		return false, nil
	}

	endpoint := checker.endpointFor(chainID)
	if endpoint == "" {
		return false, nil
	}

	calldata, valid := eip1271Calldata(hash, signature)
	if !valid {
		return false, nil
	}

	result, err := checker.ethCall(ctx, endpoint, address, calldata)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(result, eip1271MagicValue), nil
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
		return "", errors.New("siwx: invalid rpc endpoint")
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := checker.client.Do(request)
	if err != nil {
		return "", withoutEndpoint(err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("siwx: rpc answered %d", response.StatusCode)
	}

	return decodeRPCResult(response.Body)
}

func decodeRPCResult(body io.Reader) (string, error) {
	var decoded struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(body, maxRPCBodyBytes)).Decode(&decoded); err != nil {
		return "", err
	}

	if decoded.Error != nil {
		return "", fmt.Errorf("siwx: rpc error: %s", decoded.Error.Message)
	}

	return decoded.Result, nil
}

func withoutEndpoint(err error) error {
	var requestError *url.Error
	if errors.As(err, &requestError) {
		return requestError.Err
	}

	return err
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
