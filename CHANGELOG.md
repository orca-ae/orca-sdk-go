# Changelog

## Unreleased

### Maintenance

* Use neutral hosted-extension and OIDC terminology in documentation, examples,
  and source comments, preserving protocol identifiers and runtime behavior.
* Remove the proprietary hosted provider E2E topology and its source, image, and
  OAuth dependencies. Keep direct Managed Agents Helm/Kind E2E, policy/pricing,
  and unavailable hosted-extension assertions. Public SDK APIs, vendored
  contracts, and mocked hosted-extension tests are unchanged.

## [0.4.0](https://github.com/orca-ae/orca-sdk-go/compare/v0.3.1...v0.4.0) (2026-09-28)


### Features

* initial open-source release ([#1](https://github.com/orca-ae/orca-sdk-go/issues/1)) ([05f9512](https://github.com/orca-ae/orca-sdk-go/commit/05f951223f2b25681b64bad7eb9c06d2c3fd4f58))

## [0.3.1](https://github.com/orca-ae/orca-sdk-go/compare/v0.3.0...v0.3.1) (2026-09-28)


### Bug Fixes

* replace LICENSE with the verbatim Apache-2.0 text ([#23](https://github.com/orca-ae/orca-sdk-go/issues/23)) ([73b2a0a](https://github.com/orca-ae/orca-sdk-go/commit/73b2a0abf9badc00d32450d103eaa1a82db1fb71))

## 0.3.0 (2026-09-04)


### Features

* add policy and pricing extension APIs

## 0.2.0 (2026-08-29)


### ⚠ BREAKING CHANGES

* HTTPError is now an alias for APIError rather than a defined type, so a failing status returns a status-specific type that wraps it. Code using errors.As is unaffected and keeps compiling. Code using a direct type assertion, `err.(*HTTPError)`, no longer matches and must switch to errors.As; two of this repo's own tests did, and were migrated here.

### Features

* add paginated list cursors and typed SSE streams
* add the option-based request pipeline and typed errors
* add typed agents resources
* add typed environments, files, skills and triggers
* add typed memory store resources
* add typed sessions resources
* add typed vault and credential resources
* close the remaining core spec gaps
* gate the cloud extension surface behind discovery


### Bug Fixes

* make the release workflow able to open its pull request
* use the default token now that Actions may open pull requests
