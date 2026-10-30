# Changelog

All notable changes to this module are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the module adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Breaking:** `SignatureVerifier.Verify` returns a `SignerKind` instead of a `bool`.
- **Breaking:** `Parse` rejects empty required fields and a carriage return inside the message.
  `Verify` already refused such messages, so only direct callers of `Parse` are affected.

### Added

- `Session.ContractWallet` reports that a smart contract, not a key, accepted the signature.
- `Config.Chains` and `ErrUnsupportedChain` limit the accepted chains before any other check.
- `evm.RPCChecker.WithClient` sends the calls through your own `*http.Client`.
- Fuzz targets for the parser and both verifiers, runnable examples, and a parser corpus shared
  with the PHP package.

## [0.1.0] - 2026-10-30

First release, the Go counterpart of `lexwebdev/laravel-siwx` 0.4.0.

### Added

- EIP-4361 parsing and a nonce HTTP handler in the root package.
- `evm` and `solana` verifiers, enabled explicitly through `Config.Verifiers`.
- EIP-1271 with one RPC endpoint per chain.
- `memorynonce` and `redisnonce` stores with atomic consumption.

### Verified

- Live sign-ins through Phantom, MetaMask on both Ethereum and Solana, and a phone over
  WalletConnect. All of them used a plain message signature; a CACAO from a real wallet is not
  covered yet.
- The same test vectors as the PHP package, and a comparison of both parsers on 117 messages.

[Unreleased]: https://github.com/LexWebDev/siwx-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/LexWebDev/siwx-go/releases/tag/v0.1.0
