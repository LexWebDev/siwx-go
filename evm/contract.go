package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	eip1271MagicValue = "0x1626ba7e"
	rpcTimeout        = 5 * time.Second
	maxRPCBodyBytes   = 1 << 20
	wordHexLength     = 64
	signatureOffset   = 64
	revertedCode      = 3
)

var errReverted = errors.New("evm: execution reverted")

// Transport failures. They carry no endpoint, because RPC URLs often hold an API key.
var (
	ErrRPCUnreachable = errors.New("evm: rpc unreachable")
	ErrRPCTimeout     = errors.New("evm: rpc timeout")
)

// RPCChecker verifies EIP-1271 signatures through eth_call.
type RPCChecker struct {
	url    string
	urls   map[string]string
	client *http.Client
}

// NewRPCChecker returns a checker. With urls set, only those CAIP-2 chains are served and
// url is ignored; otherwise url serves every chain.
func NewRPCChecker(url string, urls map[string]string) *RPCChecker {
	return &RPCChecker{
		url:    url,
		urls:   urls,
		client: &http.Client{Timeout: rpcTimeout},
	}
}

// WithClient returns a copy of the checker that sends its requests through client.
func (checker *RPCChecker) WithClient(client *http.Client) *RPCChecker {
	configured := *checker
	configured.client = client

	return &configured
}

// IsValid reports whether the contract at address accepts signature. A revert is a plain
// rejection. Errors are fixed strings: nothing from the node or the endpoint reaches them.
func (checker *RPCChecker) IsValid(ctx context.Context, address, hash, signature, chainID string) (bool, error) {
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
	if errors.Is(err, errReverted) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(result, eip1271MagicValue), nil
}

func (checker *RPCChecker) endpointFor(chainID string) string {
	if len(checker.urls) == 0 {
		return checker.url
	}

	return checker.urls[chainID]
}

func (checker *RPCChecker) ethCall(ctx context.Context, endpoint, address, calldata string) (string, error) {
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
		return "", errors.New("evm: invalid rpc endpoint")
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := checker.client.Do(request)
	if err != nil {
		return "", transportFailure(err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("evm: rpc answered %d", response.StatusCode)
	}

	return decodeRPCResult(response.Body)
}

func decodeRPCResult(body io.Reader) (string, error) {
	var decoded struct {
		Result string `json:"result"`
		Error  *struct {
			Code int `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(body, maxRPCBodyBytes)).Decode(&decoded); err != nil {
		return "", errors.New("evm: malformed rpc response")
	}

	if decoded.Error != nil && decoded.Error.Code == revertedCode {
		return "", errReverted
	}

	if decoded.Error != nil {
		return "", fmt.Errorf("evm: rpc error %d", decoded.Error.Code)
	}

	return decoded.Result, nil
}

func transportFailure(err error) error {
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return ErrRPCTimeout
	}

	return ErrRPCUnreachable
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
