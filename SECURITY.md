# Security Policy

## Reporting a vulnerability

Please do not report security problems in a public issue, pull request, or
discussion. Report them privately, in either of these ways:

1. **GitHub private vulnerability reporting** — use *Report a vulnerability* on
   the [Security tab](https://github.com/orca-ae/orca-sdk-go/security/advisories/new).
   We prefer this channel.
2. **Email security@runorca.ai**, with the repository name in the subject line.

Include the affected version, a description of the issue, and — if you have
one — a minimal reproduction. We will acknowledge your report and keep you
updated on the fix.

## Scope

This repository is the Go client library. Issues in the Orca Agent Engine
itself, or in a hosted deployment, can also go to the address above; mention
which component you believe is affected.

Credential handling is the most security-sensitive part of this SDK. Of
particular interest:

- API keys or access tokens leaking into logs, error messages, or URLs.
- Requests carrying a credential to an unintended host, for example through
  base-URL handling or redirect following.
- Header or path injection through user-supplied resource names.

## Supported versions

Fixes land on the latest minor release. Older versions are not patched.
