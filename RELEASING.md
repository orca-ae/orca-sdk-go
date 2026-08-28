# Releasing

Releases are driven by [Release Please](https://github.com/googleapis/release-please) through
`.github/workflows/release.yml`. Conventional commit messages decide the version and write the
changelog; a human decides when to ship by merging one pull request.

## There is no publish step

A Go module is served from its git tag. Creating the tag *is* the release — there is no artifact
to build, upload, or register, and nothing to undo if you change your mind before tagging.

This is why the process here is shorter than the one in `orca-sdk-typescript`, which has to gate
an irreversible `npm publish` behind a release-candidate cycle.

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

**Do not delete RC tags after promoting.** The TypeScript process does, because an npm version can
be republished in a way a git tag cannot. Here, a consumer that has already resolved
`v0.3.0-rc.1` has it in their `go.sum`; deleting the tag breaks their build, and this module
resolves direct-to-git through `GOPRIVATE` rather than through a proxy that would keep serving it.

## Consuming a release

The repository is internal, so module resolution goes straight to git rather than through
`proxy.golang.org`:

```sh
export GOPRIVATE='github.com/orca-ae/*'
go get github.com/orca-ae/orca-sdk-go@v0.2.0
```

In CI, authenticate the fetch with a token that can read the repository:

```sh
git config --global \
  url."https://x-access-token:${MODULE_TOKEN}@github.com/orca-ae/".insteadOf \
  https://github.com/orca-ae/
```

## Required secrets

The workflow uses `secrets.SNBOT_GITHUB_TOKEN` rather than the default `GITHUB_TOKEN`.

This repository disallows GitHub Actions from creating pull requests, which is precisely what
Release Please must do — with the default token it pushes its branch and then fails at the pull
request. Scoping an elevated token to this one workflow is narrower than lifting that restriction
for every workflow in the repository.

It also means the tag is created by a real account. A tag created with `GITHUB_TOKEN` does not
trigger other workflows, so a tag-triggered workflow added later would silently never run.

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
