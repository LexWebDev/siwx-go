# Contributing

Bug reports and pull requests are welcome. This is an authentication module, so the bar for changes
to verification logic is high. Read the rules below before you write code, they will save you a
rewrite.

## Running the tests

```bash
go test -race ./...
```

One package, or one test by name:

```bash
go test ./evm
go test -run TestVerifiesAnEvmMessage .
```

CI also runs `gofmt`, `go vet` and golangci-lint on Go 1.26 and 1.27. Passing locally is not the
same as passing in CI, so check the workflow result on your branch before asking for review.

## Rules that will get a pull request sent back

**Do not edit `internal/testvectors/vectors.json`.** The vectors are real signatures, exported from
the PHP package. Change one character and they stop verifying, and the suite starts testing nothing.
If you need a new case, produce a new vector with a real key and add it alongside the existing ones.

**Keep the root package free of network calls and chain code.** The only component allowed to make a
request is `evm.RPCChecker`, and its tests run against a local server. The root package imports no
chain, which is what lets a Solana-only app skip the EVM dependencies.

**Address normalisation belongs to the namespace verifier.** EVM addresses are lowercased, Solana
base58 addresses are returned untouched. Nothing about address casing goes into shared code:
lowercasing a base58 address silently produces a different account.

**The nonce is consumed after the signature check, never before.** Reversing that order turns an
invalid signature into a way to burn somebody else's nonce. There is a test pinning this; if you
find yourself changing it, stop and open an issue instead.

**`Consume` must be atomic.** A read followed by a delete lets two parallel requests with one signed
message both pass. Every store runs the shared checks in `internal/noncetest`.

**Errors carry machine codes, not diagnostics.** Rejections use the documented sentinels. Never put
recovered addresses, hashes, endpoints or text from a remote node into an error: it ends up in
somebody's API response or logs.

## Adding a chain

The architecture expects this. Implement `siwx.SignatureVerifier` in a package of its own and pass it
through `Config.Verifiers`. No changes to the parser or the orchestrator should be necessary; if they
are, say so in the pull request, because that is a design problem worth discussing.

One condition: bring vectors produced by real wallets, not by a script that reimplements what you
think the wallet does. Wallets disagree about what exactly they sign, and that disagreement is the
whole difficulty.

## Tests come first

A pull request that changes behaviour without a test that fails before the change is not reviewable:
there is no way to tell whether the new code does what you say it does.

## Commits

Short, imperative, lowercase, prefixed by type. `git log` shows the shape:

```
feat: verify solana ed25519 signatures
fix: consume a nonce atomically so parallel requests cannot both pass
```

## Out of scope

These are deliberate omissions, not gaps to fill:

- adapters for Gin, Echo and other frameworks: the module speaks `net/http` and plain functions
- ready-made auth routes: everybody's user model and token strategy differ
- session storage: the application owns sessions

If you disagree with one of these, open an issue first. A pull request adding them will most likely
be declined, and neither of us wants that after you have written it.

## Security

Do not open a public issue for a vulnerability. [SECURITY.md](SECURITY.md) explains the private
channel.
