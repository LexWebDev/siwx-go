package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
)

const magicResult = "0x1626ba7e00000000000000000000000000000000000000000000000000000000"

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
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
	checker := siwx.NewRPCContractChecker(server.URL, nil)

	if !checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("rejected")
	}
}

func TestBuildsTheIsValidSignatureCall(t *testing.T) {
	server, _, received := rpcServer(t, magicResult)
	checker := siwx.NewRPCContractChecker(server.URL, nil)

	checker.IsValid(context.Background(), "0xabc", "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 66), "eip155:1")

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
	checker := siwx.NewRPCContractChecker(server.URL, nil)

	if checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}

func TestTreatsAnRpcFailureAsInvalidRatherThanFailing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"result":"` + magicResult + `"}`))
	}))
	t.Cleanup(server.Close)

	checker := siwx.NewRPCContractChecker(server.URL, nil)

	if checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}

func TestSendsNoCallForASignatureThatIsNotHex(t *testing.T) {
	server, calls, _ := rpcServer(t, magicResult)
	checker := siwx.NewRPCContractChecker(server.URL, nil)

	if checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), "0xnothex", "eip155:1") {
		t.Fatal("accepted")
	}

	if calls.Load() != 0 {
		t.Fatalf("calls: %d", calls.Load())
	}
}

func TestCallsTheRpcOfTheChainTheMessageNames(t *testing.T) {
	mainnet, _, _ := rpcServer(t, "0x"+strings.Repeat("0", 64))
	base, _, _ := rpcServer(t, magicResult)
	checker := siwx.NewRPCContractChecker("", map[string]string{"eip155:1": mainnet.URL, "eip155:8453": base.URL})

	if !checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:8453") {
		t.Fatal("base rejected")
	}
	if checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("mainnet accepted")
	}
}

func TestRefusesAChainOutsideTheMapInsteadOfFallingBack(t *testing.T) {
	legacy, calls, _ := rpcServer(t, magicResult)
	checker := siwx.NewRPCContractChecker(legacy.URL, map[string]string{"eip155:1": legacy.URL})

	if checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:137") {
		t.Fatal("accepted")
	}

	if calls.Load() != 0 {
		t.Fatalf("calls: %d", calls.Load())
	}
}

func TestSendsEveryChainToTheSingleRpcWhenNoMapIsConfigured(t *testing.T) {
	server, _, _ := rpcServer(t, magicResult)
	checker := siwx.NewRPCContractChecker(server.URL, nil)

	if !checker.IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:137") {
		t.Fatal("rejected")
	}
}

func TestRefusesEverythingWithoutAnRpc(t *testing.T) {
	if siwx.NewRPCContractChecker("", nil).IsValid(context.Background(), vector(t, "EVM_SIGNER"), strings.Repeat("a", 64), vector(t, "EVM_SIG"), "eip155:1") {
		t.Fatal("accepted")
	}
}
