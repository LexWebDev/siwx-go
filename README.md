# siwx-go

[![tests](https://img.shields.io/github/actions/workflow/status/LexWebDev/siwx-go/tests.yml?branch=main&logo=github&label=tests)](https://github.com/LexWebDev/siwx-go/actions/workflows/tests.yml?query=branch%3Amain)
[![go reference](https://pkg.go.dev/badge/github.com/LexWebDev/siwx-go.svg)](https://pkg.go.dev/github.com/LexWebDev/siwx-go)
[![license](https://img.shields.io/github/license/LexWebDev/siwx-go?color=blue)](LICENSE)

Server-side Sign-In With X verification for Go. Verifies wallet signatures over
[EIP-4361](https://eips.ethereum.org/EIPS/eip-4361) messages for **EVM** and **Solana** accounts,
and survives WalletConnect One-Click Auth.

The Go counterpart of [lexwebdev/laravel-siwx](https://github.com/LexWebDev/laravel-siwx). Both
share their test vectors and error codes, so one frontend can talk to either backend.

## Why this exists

A naive SIWE verifier works on the desktop happy path and breaks the moment a real wallet shows up:

- **The header is not fixed.** AppKit builds the first line from the network name, so a parser
  that greps for the word `Ethereum` rejects every non-EVM login.
- **`Chain ID` is not a number.** AppKit passes a CAIP-2 id (`eip155:1`), One-Click Auth emits a
  plain integer. Both arrive at the same endpoint, and both are normalised to CAIP-2 here.
- **One-Click Auth appends a `Resources:` section and rewrites the statement.** The wallet adds an
  [ERC-5573](https://eips.ethereum.org/EIPS/eip-5573) sentence that differs from wallet to wallet.
  `Statement` returns what your app asked for, `SignedStatement` keeps the signed line.
- **Address casing is chain-specific.** Lowercasing a Solana base58 address produces a different,
  non-existent account.

## Supported chains

| CAIP-2 namespace | Chains | Signature scheme |
|---|---|---|
| `eip155` | every EVM chain | secp256k1 + keccak256, EIP-191, optional EIP-1271 |
| `solana` | Solana mainnet/devnet | ed25519, base58 and base64 signatures |

Each chain is a separate package, and only what you pass in `Config.Verifiers` is enabled. A
Solana-only app never compiles the EVM dependencies, and your own `SignatureVerifier` plugs in the
same way as the built-in ones.

## Installation

```bash
go get github.com/LexWebDev/siwx-go
```

Requires Go 1.26 or newer.

## Usage

```go
store := memorynonce.New(10 * time.Minute)

verifier, err := siwx.New(siwx.Config{
	Domains:   []string{"app.example"},
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(nil), solana.New()},
})
if err != nil {
	log.Fatal(err)
}

http.Handle("GET /siwx/nonce", siwx.NonceHandler(store))

http.HandleFunc("POST /api/auth", func(writer http.ResponseWriter, request *http.Request) {
	session, err := verifier.Verify(request.Context(), message, signature)

	var failure *siwx.Error
	if errors.As(err, &failure) {
		http.Error(writer, failure.Code, http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(writer, "internal error", http.StatusInternalServerError)
		return
	}

	issueYourOwnSession(writer, session.Namespace, session.Address)
})
```

`Domains` is required in practice: with an empty list every verification fails with
`siwx_invalid_domain`. List each domain **once**, exactly as the wallet sends it in the `domain`
line, including a port such as `localhost:3000`. The authority of `URI` must equal that line.

`ClockSkew` tolerates wallet clocks running ahead and defaults to five minutes. Set
`siwx.NoClockSkew` to turn it off.

`NonceHandler` has no rate limit, and every request stores a nonce. **Put a limiter in front of it.**

`Session` carries `Address`, `Namespace`, `ChainID`, `Domain`, `IssuedAt` and the parsed `Message`.
Authentication routes and session storage are deliberately not included: they depend on your user
model and token strategy.

Never store the address alone. The same string can be valid on more than one namespace, so make
the unique index cover `(namespace, address)`.

### Nonce stores

| Store | Use it for |
|---|---|
| `memorynonce.New` | a single process |
| `redisnonce.New` | several processes; needs Redis 6.2+ for `GETDEL` |

A custom `NonceStore` must make `Consume` atomic. A read followed by a delete lets two parallel
requests with the same signed message both pass.

## Error codes

Every rejection wraps a `*siwx.Error` you can match with `errors.Is`. Send `failure.Code` to the
client, never `err.Error()`. When the EIP-1271 call fails, the error wraps both
`ErrInvalidSignature` and the cause, such as `evm.ErrRPCTimeout`, so the login is refused and you
can still log the outage. Any other error, such as a Redis outage, is an infrastructure failure and
comes through unchanged.

| Code | Sentinel | Meaning |
|---|---|---|
| `siwx_invalid_message` | `ErrInvalidMessage` | message breaks the EIP-4361 grammar, bad version, or a failed timestamp check |
| `siwx_invalid_domain` | `ErrInvalidDomain` | `domain` or `URI` authority is not allowed, or the list is empty |
| `siwx_invalid_nonce` | `ErrInvalidNonce` | nonce was never issued, already used, or expired |
| `siwx_invalid_signature` | `ErrInvalidSignature` | signature does not belong to the claimed address |
| `siwx_unsupported_namespace` | `ErrUnsupportedNamespace` | namespace is unknown or disabled |

The nonce is consumed **after** the signature check passes, so an invalid signature cannot burn
somebody else's nonce.

## Frontend notes

**Phantom refuses the genesis-hash chain id.** AppKit passes
`solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp`, and the Phantom extension, as tested on 2026-10-30, will
not sign a message carrying it. Write `solana:mainnet` in the `Chain ID` line instead. Both forms
are accepted, and `Session.ChainID` holds the value as it was signed.

**One-Click Auth only fires in a single-namespace app.** AppKit attempts it only when all your
networks belong to `eip155`. An app registering EVM and Solana together always falls back to
`personal_sign`.

## Smart contract wallets (EIP-1271)

Off by default. Pass a checker to `evm.New`, and a signature that fails recovery gets a second
chance through `eth_call` to `isValidSignature`:

```go
contracts := evm.NewRPCChecker("", map[string]string{
	"eip155:1":    mainnetURL,
	"eip155:8453": baseURL,
})

verifier, err := siwx.New(siwx.Config{
	Domains:   domains,
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(contracts), solana.New()},
})
```

Once the map is set, a chain outside it fails verification. A single URL as the first argument
serves every chain instead, so the signature is checked on that endpoint's network whatever
`Chain ID` the message names. Use that only if you accept one chain.

**A contract address belongs to one chain, not to one owner.** If you map more than one chain, key
contract-wallet accounts by `(ChainID, Address)`, or map only the chain you trust.

EOA logins never pay for this: the call happens only after recovery fails, and only for a live
nonce. A contract that reverts is a plain rejection. This is the only place the module touches the
network, and its errors are fixed strings: neither the endpoint, which often carries an API key,
nor text from the node ever reaches them.

## Trying it with real wallets

`examples/stand` is a small server exposing `GET /siwx/nonce` and `POST /api/siwx/verify`. It reads
the variables listed in `.env.example` from the environment.

```bash
go run ./examples/stand
```

## Security

See [SECURITY.md](SECURITY.md). Please do not report vulnerabilities in public issues.

## License

MIT. See [LICENSE](LICENSE).
