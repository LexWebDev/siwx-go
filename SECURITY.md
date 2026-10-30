# Security Policy

This module performs authentication. A bug here is an authentication bypass, so please treat
findings accordingly.

## Supported versions

The latest minor release receives security fixes.

## Reporting a vulnerability

**Do not open a public issue.** Use GitHub's private vulnerability reporting on the
[Security tab](https://github.com/LexWebDev/siwx-go/security) of this repository, and describe the
issue, affected versions and a reproduction if you have one.

You will get an initial response within 72 hours.

## Out of scope

- Session and token handling in the consuming application: this module returns a verified address
- An empty `Config.Domains`: it rejects everything by design
- A missing rate limit in front of `NonceHandler`: it is documented as your responsibility
