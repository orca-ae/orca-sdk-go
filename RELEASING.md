# Releasing

Releases are driven by [Release Please](https://github.com/googleapis/release-please) through
`.github/workflows/release.yml`. Conventional commit messages decide the version and write the
changelog; a human decides when to ship by merging one pull request.

## There is no publish step

A Go module is served from its git tag. Creating the tag *is* the release — there is no artifact
to build, upload, or register, and nothing to undo if you change your mind before tagging.

This is why the process here is shorter than an npm package's, which has to gate an irreversible
`npm publish` behind a release-candidate cycle.

## The loop

1. Land work on `main` with [Conventional Commits](https://www.conventionalcommits.org/). The
   prefix is what picks the version: `fix:` bumps the patch, `feat:` the minor, and `feat!:` or a
   `BREAKING CHANGE:` footer marks a breaking change.
2. Release Please opens a pull request titled `release: <version>`. It bumps
   `.release-please-manifest.json`, writes `CHANGELOG.md`, and rewrites the version in
   `internal/version.go` and the README install line.
3. Review it — the changelog is the release notes, so read it as one — and merge.
4. The next run creates the tag and the GitHub Release.

While this module is pre-1.0, a breaking change bumps the **minor** (`bump-minor-pre-major`), so
`0.1.0` with a breaking change becomes `0.2.0`, not `1.0.0`.

## Never edit the version by hand

`internal/version.go` and the README install line are listed in `extra-files` in
`release-please-config.json` and are rewritten by the release pull request. Editing either by
hand puts them out of step with the tag, and the constant is not decorative: it is sent as
`X-Orca-Client` on every request, so a wrong value misreports the client version in the
deployment's logs.

Both carry markers that make them findable:

```go
const Version = "0.2.0" // x-release-please-version
```

```
<!-- x-release-please-start-version -->
...
<!-- x-release-please-end -->
```

## Release candidates

There is no RC automation, because RCs here are occasional rather than a cadence. Cut one by hand
when a consumer needs to validate against unreleased work:

```sh
git tag -a v0.3.0-rc.1 <sha> -m 'v0.3.0-rc.1'
git push origin v0.3.0-rc.1
```

A pre-release sorts below the stable version, so `go get -u` will not select it by accident, and
Release Please ignores it when computing the next release.

**Do not delete RC tags after promoting.** A consumer that has already resolved `v0.3.0-rc.1` has
it in their `go.sum`. `proxy.golang.org` keeps serving a version it has cached, but anyone who
resolves the module straight from git — through `GOPRIVATE` or `GOPROXY=direct` — needs the tag,
and deleting it breaks their build.

## Consuming a release

The module is public, so `go get` resolves it through `proxy.golang.org` and verifies it against
`sum.golang.org` like any other module:

```sh
go get github.com/orca-ae/orca-sdk-go@latest
```

No token or `GOPRIVATE` entry is needed. If a `GOPRIVATE` pattern you set for other modules
happens to match this one, narrow it: a module matched by `GOPRIVATE` skips the checksum database,
so nothing verifies what you download.

## Maintainer checks

Before merging a change to the exported API, and before merging a release pull request, a
maintainer builds the known downstream consumers against it. `scripts/detect-breaking-changes`
catches a signature change, but not a behaviour change a consumer relies on. For the Orca CLI:

```sh
cd ../orca-cli
go mod edit -replace github.com/orca-ae/orca-sdk-go=../orca-sdk-go
go build ./... && go test ./...
go mod edit -dropreplace github.com/orca-ae/orca-sdk-go
```

This needs the CLI's source; contributors don't.

## Required permissions

The workflow uses the default `secrets.GITHUB_TOKEN` and declares the write access Release Please
needs — `contents: write` to push its branch and the tag, `pull-requests: write` to open the
release pull request — so the repository's default workflow permissions can stay read-only. One
repository setting is still required:

- **Allow GitHub Actions to create and approve pull requests.** Release Please's whole model is a
  pull request; without this it pushes the branch and then fails with
  `GitHub Actions is not permitted to create or approve pull requests`. This is governed by an
  organization policy as well as a repository setting, and the organization one wins.

One consequence worth knowing before adding a tag-triggered workflow here: a tag pushed with
`GITHUB_TOKEN` does not trigger other workflows. Such a workflow would silently never fire, and
would need the tag pushed by a token belonging to a real account instead.

## When a release goes wrong

- **No release pull request appeared.** Check the `Release` workflow run. If it failed with
  `GitHub Actions is not permitted to create or approve pull requests`, the token is wrong — see
  above.
- **Nothing to release.** Release Please only counts commits whose type is release-worthy. A run
  of `chore:` and `test:` commits produces no pull request; that is correct, not a failure.
- **Wrong version.** Fix the commit history going forward rather than editing the manifest: the
  version is derived from the commits, and hand-editing makes the next computation wrong too.
- **Tag pushed by mistake.** Do not delete it if anyone may have resolved it. Release the fix as
  the next patch instead.
