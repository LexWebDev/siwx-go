## What this changes

<!-- And why. If it fixes an issue, link it. -->

## How it was verified

<!--
Which test fails without this change? Paste the go test output if the change touches verification.
-->

## Checks

- [ ] `go test -race ./...` passes locally
- [ ] `internal/testvectors/vectors.json` is untouched, or a new vector was produced with a real key
- [ ] No new network calls outside `evm.RPCChecker`
- [ ] Address casing rules stayed inside the namespace verifier
- [ ] The nonce is still consumed only after the signature check, and atomically
- [ ] Rejections are still machine codes, and nothing remote reaches an error string

<!--
CI runs Go 1.26 and 1.27 with the race detector and golangci-lint. Local green is not CI green,
so check your branch before asking for review.
-->
