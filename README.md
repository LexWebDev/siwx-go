# siwx-go

**English** | [Українська](README.uk.md)

[![go reference](https://pkg.go.dev/badge/github.com/LexWebDev/siwx-go.svg)](https://pkg.go.dev/github.com/LexWebDev/siwx-go)
[![go](https://img.shields.io/github/go-mod/go-version/LexWebDev/siwx-go?logo=go&logoColor=white&label=go)](go.mod)
[![tests](https://img.shields.io/github/actions/workflow/status/LexWebDev/siwx-go/tests.yml?branch=main&logo=github&label=tests)](https://github.com/LexWebDev/siwx-go/actions/workflows/tests.yml?query=branch%3Amain)
[![license](https://img.shields.io/github/license/LexWebDev/siwx-go?color=blue)](LICENSE)

Server-side Sign-In With X verification for Go. Verifies wallet signatures over
[EIP-4361](https://eips.ethereum.org/EIPS/eip-4361) messages for **EVM** and **Solana** accounts,
and survives WalletConnect One-Click Auth.

The Go counterpart of [lexwebdev/laravel-siwx](https://github.com/LexWebDev/laravel-siwx). Both share
their test vectors and error codes, so one frontend can talk to either backend.

## Why this exists

A naive SIWE verifier works on the desktop happy path and breaks the moment a real wallet
shows up:

- **The header is not fixed.** AppKit builds the first line as
  `${domain} wants you to sign in with your ${networkName} account:`, where `networkName`
  comes from the network config: `Solana`, `Polygon`, anything. A parser that greps for the
  word `Ethereum` rejects every non-EVM login.
- **`Chain ID` is not a number.** AppKit passes a CAIP-2 id (`eip155:1`,
  `solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp`), while the One-Click Auth path builds the
  message through WalletConnect's `formatAuthMessage` and emits a plain integer per EIP-4361.
  Both arrive at the same endpoint. This module accepts both and normalises to CAIP-2.
- **One-Click Auth appends a `Resources:` section** with a `urn:recap:` capability. It is part
  of the signed bytes, so it must be preserved for verification and ignored for field parsing.
- **One-Click Auth rewrites the statement.** Per [ERC-5573](https://eips.ethereum.org/EIPS/eip-5573)
  the wallet appends `I further authorize the stated URI to perform the following actions on my
  behalf: (1) 'request': 'personal_sign' …` to whatever statement the app asked for, and the list
  of methods differs from wallet to wallet. An application that compares the statement with the
  text it requested rejects every mobile login. This module returns the app's own statement in
  `Statement` and keeps the full signed line in `SignedStatement`.
- **Address casing is chain-specific.** Lowercasing a Solana base58 address turns it into a
  different, non-existent account.

Each of those is covered by a test against a real signature vector.

## Supported chains

| CAIP-2 namespace | Package | Chains | Signature scheme |
|---|---|---|---|
| `eip155` | `evm` | every EVM chain | secp256k1 + keccak256, EIP-191, optional EIP-1271 |
| `solana` | `solana` | Solana mainnet/devnet | ed25519, base58 and base64 signatures |

Each chain is a separate package, and only what you pass in `Config.Verifiers` is enabled. A
Solana-only app never compiles the EVM dependencies. Your own `SignatureVerifier` plugs in the same
way as the built-in ones, without touching the core:

```go
verifier, err := siwx.New(siwx.Config{
	Domains:   []string{"app.example"},
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(nil), solana.New(), myBip122Verifier},
})
```

## Installation

```bash
go get github.com/LexWebDev/siwx-go
```

## Configuration

`Domains` is **required** in practice. With an empty list every verification fails with
`siwx_invalid_domain`. That is deliberate: an empty allow list is safer than an open one, and
the domain binding is what stops a signature obtained on another site from working on yours.

| Field | Purpose |
|---|---|
| `Domains` | allow list of domains |
| `Nonces` | **required**, a `NonceStore` |
| `Verifiers` | **required**, the enabled chains |
| `ClockSkew` | tolerance for wallet clocks running ahead, five minutes by default; `siwx.NoClockSkew` turns it off |
| `Now` | replaces `time.Now`, useful in tests |

List each domain **once**, exactly as the wallet will send it in the `domain` line, including a
port if your app runs on one, e.g. `localhost:3000` during development.

The `URI` field is not matched against the allow list separately. Instead its authority (host, plus
port when present) must equal the `domain` line, which is what EIP-4361 requires of a well-formed
message. That is stricter than checking both against the list: with two allowed hosts, a message
could otherwise name one domain and point its URI at the other.

## Usage

`siwx.NonceHandler` serves `GET` requests with `{"nonce": "..."}`. Nonces are single use and expire
after the TTL of the store.

The handler has no rate limit, and every request stores a nonce. **Put a limiter in front of it.**

Authentication routes are deliberately not included: they depend on your user model and token
strategy. A minimal server:

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
	var body struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8192)).Decode(&body); err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}

	session, err := verifier.Verify(request.Context(), body.Message, body.Signature)

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

`Verify` returns a `*siwx.Session` on success:

```go
session.Address   // normalised per chain
session.Namespace // "eip155" or "solana"
session.ChainID   // always CAIP-2, e.g. "eip155:137"
session.Domain
session.IssuedAt  // time.Time
session.Message   // the parsed *siwx.Message
```

`Message` carries every parsed field. The ones worth knowing about when you validate intent:

```go
message.Statement       // what your app asked the wallet to sign, recap sentence removed
message.SignedStatement // the statement line exactly as it appears in the signed message
message.Resources       // entries of the Resources section, e.g. ["urn:recap:…"]
message.Scheme          // optional scheme of the header line, e.g. "https", or empty
message.RequestID       // the Request ID field, or empty
```

Compare `Statement`, not `SignedStatement`, with the text you requested. On the One-Click Auth path
the two differ, and the difference is wallet-specific.

### Nonce stores

| Store | Use it for |
|---|---|
| `memorynonce.New` | a single process |
| `redisnonce.New` | several processes; needs Redis 6.2+ for `GETDEL` |

A custom `NonceStore` must make `Consume` atomic. A read followed by a delete lets two parallel
requests with the same signed message both pass.

### Storing the address

`session.Address` is normalised **per chain**: EVM addresses are lowercased, Solana base58
addresses are returned byte-for-byte because base58 is case-sensitive.

Never store the address alone. The same string can be a valid account on more than one
namespace, so make the unique index cover the pair `(namespace, address)`.

## Error codes

Every rejection wraps a `*siwx.Error` you can match with `errors.Is`. Send `failure.Code` to the
client, never `err.Error()`. When the EIP-1271 call fails, the error wraps both
`ErrInvalidSignature` and the cause, such as `evm.ErrRPCTimeout`, so the login is refused and you
can still log the outage. Any other error, such as a Redis outage, is an infrastructure failure and
comes through unchanged.

| Code | Sentinel | Meaning |
|---|---|---|
| `siwx_invalid_message` | `ErrInvalidMessage` | message breaks the EIP-4361 grammar (duplicate or unknown fields, a multi-line statement, timestamps outside RFC 3339, a `Chain ID` outside CAIP-2), bad version, or a failed timestamp check |
| `siwx_invalid_domain` | `ErrInvalidDomain` | `domain` or `URI` host is not in the allow list, or the list is empty |
| `siwx_invalid_nonce` | `ErrInvalidNonce` | nonce was never issued, already used, or expired |
| `siwx_invalid_signature` | `ErrInvalidSignature` | signature does not belong to the claimed address |
| `siwx_unsupported_namespace` | `ErrUnsupportedNamespace` | no verifier is enabled for the namespace |

The nonce is consumed **after** the signature check passes, so an invalid signature cannot be
used to burn somebody else's nonce.

## Frontend

Use `SIWXConfig` from `@reown/appkit`: there is no separate `@reown/appkit-siwx` package to
install.

```ts
import { createAppKit } from '@reown/appkit'

createAppKit({
  // ...
  siwx: {
    async getNonce() {
      const { nonce } = await fetch('/siwx/nonce').then((r) => r.json())
      return nonce
    },
    async addSession({ message, signature }) {
      await fetch('/api/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, signature }),
      })
    },
    // getSessions, revokeSession, setSessions per your app
  },
})
```

On the One-Click Auth path `createMessage` is called without an address, so the nonce cannot be
derived from the wallet: fetch it from `GET /siwx/nonce` first.

### Phantom refuses the genesis-hash chain id on Solana

AppKit hands `createMessage` the CAIP-2 id `solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp`. The Phantom
extension, as tested on 2026-10-30, will not sign a message carrying it: the wallet fails with
"The app's signature request cannot be shown due to invalid formatting", AppKit shows "Error
signing message", and nothing reaches your backend. Write `solana:mainnet` in the `Chain ID` line
instead:

```js
const signedChainId = chainId === 'solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp' ? 'solana:mainnet' : chainId
```

This module accepts both forms and reports the value as it was signed, so `session.ChainID` can
hold either one. Compare against both if you branch on it.

### One-Click Auth only fires in a single-namespace app

AppKit attempts One-Click Auth (`wc_sessionAuthenticate`, which returns a CACAO and a message
built by WalletConnect rather than by your `createMessage`) only when the networks you register
belong to **exactly one** namespace, and that namespace is `eip155`.

So an app that registers EVM chains **and** Solana in the same AppKit instance never produces a
CACAO: every sign-in falls back to a plain `personal_sign` of the message your own code composed.
That is worth knowing before you go looking for a bug in your `Resources` handling: there simply
will not be a `Resources` section to handle.

This module verifies both shapes either way, so nothing here needs configuring.

Two things follow for testing. A wallet that does not implement `wc_sessionAuthenticate` also falls
back silently, so an absent CACAO does not by itself tell you which side declined. And the request
is sent during connection, not at signing time: if you see a signature prompt arrive after the
session is already established, One-Click Auth did not happen.

## Smart contract wallets (EIP-1271)

Off by default. Pass a checker to `evm.New`, and a signature that fails recovery gets a second
chance through an `eth_call` to `isValidSignature` on the claimed address:

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
`Chain ID` the message names. Use that only if you accept a single chain.

**A contract address belongs to one chain, not to one owner.** The same address can be controlled
by different parties on two networks. If you map more than one chain, key contract-wallet accounts
by `(session.ChainID, session.Address)` rather than by address alone, or map only the single chain
you trust. With a single URL the session reports the chain the message claims, not the one that
was checked, so treat `session.ChainID` as unverified in that mode.

EOA logins never pay for this: the RPC call only happens after recovery fails, and only for a
nonce that is still live, so a request with a made-up nonce costs you no RPC quota. A contract that
reverts is a plain rejection. This is the only place the module touches the network, and its errors
are fixed strings: neither the endpoint, which often carries an API key, nor text from the node
ever reaches them. Swap the implementation by passing your own `evm.ContractChecker`.

## Compatibility

| | |
|---|---|
| Go | 1.26+, CI runs 1.26 and 1.27 with the race detector |
| Frameworks | none required, the handler is a plain `http.Handler` |
| Redis | 6.2+ for `redisnonce` |

## Trying it with real wallets

`examples/stand` is a small server exposing `GET /siwx/nonce` and `POST /api/siwx/verify`. It reads
the variables listed in `.env.example` from the environment.

```bash
go run ./examples/stand
```

## Contributing

Bug reports and pull requests are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) covers how to run
the suite and the rules around verification code, worth reading before you touch a verifier,
since a few of them are not obvious. Participation is under the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Security

See [SECURITY.md](SECURITY.md). Please do not report vulnerabilities in public issues.

## License

MIT. See [LICENSE](LICENSE).
