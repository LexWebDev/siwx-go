# Security Policy

This module performs authentication. A bug here is an authentication bypass, so please treat
findings accordingly.

## Supported versions

The latest minor release receives security fixes. Older minors are not patched: upgrade first,
then report if the issue persists.

| Version | Supported |
|---|---|
| 0.1.x | yes |

## Reporting a vulnerability

**Do not open a public issue.** Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/LexWebDev/siwx-go/security) of this repository.
2. Click **Report a vulnerability**.
3. Describe the issue, affected versions, and a reproduction if you have one.

The report stays private between you and the maintainer, and a fix can be prepared and
disclosed together with a CVE if warranted.

You will get an initial response within 72 hours. If you receive nothing in that window,
please assume the notification was lost and follow up in the same private thread.

## What to include

- Module version and Go version
- The chain namespace involved (`eip155`, `solana`, or a custom one)
- The message and signature that reproduce the problem, if they contain no private data
- What you expected to happen versus what happened

## Out of scope

- Vulnerabilities in the consuming application's session or token handling: this module
  returns a verified address and does not manage sessions
- An empty `Config.Domains`: it rejects everything by design
- A missing rate limit in front of `NonceHandler`: it is documented as your responsibility
- Findings that require a compromised RPC endpoint that the application itself configured

## Disclosure

Once a fix is released, the advisory is published with credit to the reporter unless anonymity
is requested.
