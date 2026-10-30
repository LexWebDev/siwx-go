package evm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LexWebDev/siwx-go/evm"
	"github.com/LexWebDev/siwx-go/internal/testvectors"
)

const magicResult = "0x1626ba7e00000000000000000000000000000000000000000000000000000000"

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

func accepted(checker *evm.RPCChecker, address, hash, signature, chainID string) bool {
	valid, _ := checker.IsValid(context.Background(), address, hash, signature, chainID)

	return valid
}

func rpcServer(t *testing.T, result string) (*httptest.Server, *atomic.Int32, *rpcRequest) {
	t.Helper()

	calls := &atomic.Int32{}
	received := &rpcRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		_ = json.NewDecoder(request.Body).Decode(received)
		_ = json.NewEncoder(writer).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(server.Close)

	return server, calls, received
}

func TestReadsTheMagicValueFromAnEthCallResponse(t *testing.T) {
	server, _, _ := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker(server.URL, nil)

	if !accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("rejected")
	}
}

func TestBuildsTheIsValidSignatureCall(t *testing.T) {
	server, _, received := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker(server.URL, nil)

	accepted(checker, "0xabc", "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 66), "eip155:1")

	want := "0x1626ba7e" +
		strings.Repeat("a", 64) +
		strings.Repeat("0", 62) + "40" +
		strings.Repeat("0", 62) + "21" +
		strings.Repeat("b", 66) + strings.Repeat("0", 62)

	call, valid := received.Params[0].(map[string]any)
	if !valid {
		t.Fatalf("params: %v", received.Params)
	}

	if received.JSONRPC != "2.0" || received.Method != "eth_call" || received.Params[1] != "latest" {
		t.Fatalf("request: %+v", received)
	}
	if call["to"] != "0xabc" || call["data"] != want {
		t.Fatalf("call: %v", call)
	}
}

func TestTreatsAnyOtherMagicValueAsInvalid(t *testing.T) {
	server, _, _ := rpcServer(t, "0x"+strings.Repeat("0", 64))
	checker := evm.NewRPCChecker(server.URL, nil)

	if accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}

func TestTreatsAnRpcFailureAsInvalidRatherThanFailing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"result":"` + magicResult + `"}`))
	}))
	t.Cleanup(server.Close)

	checker := evm.NewRPCChecker(server.URL, nil)

	if accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}

func TestSendsNoCallForASignatureThatIsNotHex(t *testing.T) {
	server, calls, _ := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker(server.URL, nil)

	if accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), "0xnothex", "eip155:1") {
		t.Fatal("accepted")
	}

	if calls.Load() != 0 {
		t.Fatalf("calls: %d", calls.Load())
	}
}

func TestCallsTheRpcOfTheChainTheMessageNames(t *testing.T) {
	mainnet, _, _ := rpcServer(t, "0x"+strings.Repeat("0", 64))
	base, _, _ := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker("", map[string]string{"eip155:1": mainnet.URL, "eip155:8453": base.URL})

	if !accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:8453") {
		t.Fatal("base rejected")
	}
	if accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("mainnet accepted")
	}
}

func TestRefusesAChainOutsideTheMapInsteadOfFallingBack(t *testing.T) {
	legacy, calls, _ := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker(legacy.URL, map[string]string{"eip155:1": legacy.URL})

	if accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:137") {
		t.Fatal("accepted")
	}

	if calls.Load() != 0 {
		t.Fatalf("calls: %d", calls.Load())
	}
}

func TestSendsEveryChainToTheSingleRpcWhenNoMapIsConfigured(t *testing.T) {
	server, _, _ := rpcServer(t, magicResult)
	checker := evm.NewRPCChecker(server.URL, nil)

	if !accepted(checker, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:137") {
		t.Fatal("rejected")
	}
}

func TestRefusesEverythingWithoutAnRpc(t *testing.T) {
	if accepted(evm.NewRPCChecker("", nil), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}

func TestReportsAnRpcFailureWithoutRevealingTheEndpoint(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	endpoint := server.URL + "/v1/secret-project-key"
	checker := evm.NewRPCChecker(endpoint, nil)
	server.Close()

	valid, err := checker.IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err == nil {
		t.Fatalf("valid %v, err %v", valid, err)
	}
	if strings.Contains(err.Error(), "secret-project-key") {
		t.Fatalf("leaked: %v", err)
	}
}

func TestReportsAnErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	valid, err := evm.NewRPCChecker(server.URL, nil).IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err == nil {
		t.Fatalf("valid %v, err %v", valid, err)
	}
}

func TestIsNotAnErrorWhenAChainIsSimplyNotConfigured(t *testing.T) {
	valid, err := evm.NewRPCChecker("", nil).IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err != nil {
		t.Fatalf("valid %v, err %v", valid, err)
	}
}

func jsonRPCServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

func TestReportsAJsonRpcErrorWithoutRepeatingItsText(t *testing.T) {
	server := jsonRPCServer(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"forged\nlog line"}}`)

	valid, err := evm.NewRPCChecker(server.URL, nil).IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err == nil {
		t.Fatalf("valid %v, err %v", valid, err)
	}
	if strings.Contains(err.Error(), "forged") || !strings.Contains(err.Error(), "-32005") {
		t.Fatalf("got %v", err)
	}
}

func TestTreatsAContractRevertAsARejectionRatherThanAnOutage(t *testing.T) {
	server := jsonRPCServer(t, `{"jsonrpc":"2.0","id":1,"error":{"code":3,"message":"execution reverted: forged"}}`)

	valid, err := evm.NewRPCChecker(server.URL, nil).IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err != nil {
		t.Fatalf("valid %v, err %v", valid, err)
	}
}

func TestDoesNotRevealASecretInTheHostname(t *testing.T) {
	checker := evm.NewRPCChecker("http://secret-project-key.invalid/v1", nil)

	valid, err := checker.IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || !errors.Is(err, evm.ErrRPCUnreachable) {
		t.Fatalf("valid %v, err %v", valid, err)
	}
	if strings.Contains(err.Error(), "secret-project-key") {
		t.Fatalf("leaked: %v", err)
	}
}

func TestReportsATimeoutSeparately(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := evm.NewRPCChecker(server.URL, nil).IsValid(ctx, testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if !errors.Is(err, evm.ErrRPCTimeout) {
		t.Fatalf("got %v", err)
	}
}

func TestDoesNotRepeatAMalformedResponse(t *testing.T) {
	server := jsonRPCServer(t, `forged response`)

	valid, err := evm.NewRPCChecker(server.URL, nil).IsValid(context.Background(), testvectors.Get(t, "EVM_SIGNER"), strings.Repeat("a", 64), testvectors.Get(t, "EVM_SIG"), "eip155:1")

	if valid || err == nil || strings.Contains(err.Error(), "forged") {
		t.Fatalf("valid %v, err %v", valid, err)
	}
}
