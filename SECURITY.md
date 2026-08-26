# Security Policy

## Reporting a vulnerability

Please do not open a public GitHub issue for security problems.

Report vulnerabilities privately to **security@streamnative.io**. Include the
affected version, a description of the issue, and — if you have one — a minimal
reproduction. We will acknowledge your report and keep you updated on the fix.

## Scope

This repository is the Go client library. Issues in the Orca Agent Engine
service itself, or in a StreamNative Cloud deployment, should also go to the
address above; mention which component you believe is affected.

Credential handling is the most security-sensitive part of this SDK. Of
particular interest:

- API keys or access tokens leaking into logs, error messages, or URLs.
- Requests carrying a credential to an unintended host, for example through
  base-URL handling or redirect following.
- Header or path injection through user-supplied resource names.

## Supported versions

Fixes land on the latest minor release. Older versions are not patched.
