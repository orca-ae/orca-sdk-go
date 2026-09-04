# Changelog

## [0.3.0](https://github.com/orca-ae/orca-sdk-go/compare/v0.2.0...v0.3.0) (2026-09-04)


### Features

* add policy and pricing extension APIs ([e790403](https://github.com/orca-ae/orca-sdk-go/commit/e7904038fe987eccc975b22386b29de350f0f491))

## [0.2.0](https://github.com/orca-ae/orca-sdk-go/compare/v0.1.0...v0.2.0) (2026-08-29)


### ⚠ BREAKING CHANGES

* HTTPError is now an alias for APIError rather than a defined type, so a failing status returns a status-specific type that wraps it. Code using errors.As - which is what orca-cli does - is unaffected and keeps compiling. Code using a direct type assertion, `err.(*HTTPError)`, no longer matches and must switch to errors.As; two of this repo's own tests did, and were migrated here.

### Features

* add paginated list cursors and typed SSE streams ([#3](https://github.com/orca-ae/orca-sdk-go/issues/3)) ([b847ae3](https://github.com/orca-ae/orca-sdk-go/commit/b847ae31cadc113b8c4d5abd5943cd53f963e811))
* add the option-based request pipeline and typed errors ([#2](https://github.com/orca-ae/orca-sdk-go/issues/2)) ([2c3c073](https://github.com/orca-ae/orca-sdk-go/commit/2c3c073ccc3071768fef8a810639146e52d992d4))
* add typed agents resources ([#5](https://github.com/orca-ae/orca-sdk-go/issues/5)) ([81eaa63](https://github.com/orca-ae/orca-sdk-go/commit/81eaa635cb4424cc889144b197a8738bd5c2192c))
* add typed environments, files, skills and triggers ([#9](https://github.com/orca-ae/orca-sdk-go/issues/9)) ([b1efc8b](https://github.com/orca-ae/orca-sdk-go/commit/b1efc8b4f3c51c69cdfb4ef5cca11d54fe580611))
* add typed memory store resources ([#7](https://github.com/orca-ae/orca-sdk-go/issues/7)) ([a941476](https://github.com/orca-ae/orca-sdk-go/commit/a9414764aecc180fa56db7fa4cd7fd3cee826a42))
* add typed sessions resources ([#6](https://github.com/orca-ae/orca-sdk-go/issues/6)) ([b9dcdd4](https://github.com/orca-ae/orca-sdk-go/commit/b9dcdd48791348ab8cadc3d45034e92afd262eb7))
* add typed vault and credential resources ([#8](https://github.com/orca-ae/orca-sdk-go/issues/8)) ([2dae0ec](https://github.com/orca-ae/orca-sdk-go/commit/2dae0eceab9871354caa6da4b25c4de1a57099d1))
* close the remaining core spec gaps ([#13](https://github.com/orca-ae/orca-sdk-go/issues/13)) ([df1870c](https://github.com/orca-ae/orca-sdk-go/commit/df1870c704e707a463db93dcd5b28d082d774b0e))
* gate the cloud extension surface behind discovery ([#4](https://github.com/orca-ae/orca-sdk-go/issues/4)) ([57d67ea](https://github.com/orca-ae/orca-sdk-go/commit/57d67ea8d312837d8d02fcc33fa22cb17b76f6ec))


### Bug Fixes

* make the release workflow able to open its pull request ([#14](https://github.com/orca-ae/orca-sdk-go/issues/14)) ([bfc8681](https://github.com/orca-ae/orca-sdk-go/commit/bfc868144ffa0e11a6eee4e1e5059b96719ef6c7))
* use the default token now that Actions may open pull requests ([#15](https://github.com/orca-ae/orca-sdk-go/issues/15)) ([2e09b48](https://github.com/orca-ae/orca-sdk-go/commit/2e09b4858280c212195f701936c23ca4a6c1c4bc))
