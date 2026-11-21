# Releasing

A release is a version tag. Pushing `vX.Y.Z` makes the
[Release workflow](.github/workflows/release.yml) run the full CI suite on the
tagged commit and then publish:

- `jim-dot-tennis_X.Y.Z_linux_amd64` and `..._linux_arm64` (the single binary),
  a checksums file and generated notes, on GitHub Releases;
- the multi-architecture image `ghcr.io/jameshartt/jim-dot-tennis:X.Y.Z`
  (no leading `v` in the image tag or in what `jim-dot-tennis version` prints);
- build provenance attestations for the image and the binaries.

`latest` moves only for a stable version. Anything before 1.0.0 (`v0.x`) and
anything with a suffix (`v1.2.0-rc.1`) is a pre-release: it is marked as one on
GitHub and never changes `latest`.

## Cut a release

1. Make sure `main` is what you want to ship and its CI is green.
2. In `CHANGELOG.md`, rename the top section to `## [X.Y.Z] - YYYY-MM-DD`, check
   its **Upgrade notes** (see the convention at the top of that file) and
   review the **Changed** list against `git log vPREVIOUS..main`.
3. Update the default in `deploy/docker-compose.yml` and `deploy/.env.example`
   (`JIM_DOT_TENNIS_VERSION`) to `X.Y.Z` if this is the version clubs should
   install by default (not needed for a release candidate).
4. Commit and merge those changes to `main` through the usual pull request.
5. Tag that commit and push the tag (the workflow refuses a tag whose commit is
   not on `main`, a tag that is not `vMAJOR.MINOR.PATCH[-suffix]`, or a version
   with no CHANGELOG section):

   ```bash
   git checkout main && git pull
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

6. Watch the run under the repository's Actions tab. The `CI` job runs first;
   nothing is built or published unless it passes. If a later job fails, fix the
   cause and re-run the failed jobs (re-running publishes the same tags again);
   if the tagged commit itself must change, delete the tag and the failed
   release, and tag again.
7. First release only: make the package public (see below).
8. Check the result: the release page has two binaries, the checksums file and
   notes; `docker pull ghcr.io/jameshartt/jim-dot-tennis:X.Y.Z` works from a
   machine that is not logged in; running it prints the version
   (`docker run --rm ghcr.io/jameshartt/jim-dot-tennis:X.Y.Z ./jim-dot-tennis version`).
9. Edit the GitHub release notes if the generated list needs a sentence of
   context. Upgrade notes stay in `CHANGELOG.md`; the release notes link to it.

## One-time: make the package public

The first push creates the package `jim-dot-tennis` on ghcr.io as **private**.
Clubs cannot pull it until you change that, by hand, once (this is a GitHub
setting the workflow cannot and should not change): on GitHub open your profile
or organisation, Packages, `jim-dot-tennis`, Package settings, Danger Zone,
Change visibility, Public. Also check that the package is linked to this
repository (Package settings, Connect repository) so it appears on the
repository page and inherits its access. Later versions stay public.

## How a club verifies a download

The image and the binaries carry GitHub build provenance attestations, signed
with Sigstore, that tie each artifact to this repository, the tagged commit and
the workflow run that built it. With the GitHub CLI (`gh`) installed:

```bash
# The image (by version tag; the digest is checked, not just the tag)
gh attestation verify oci://ghcr.io/jameshartt/jim-dot-tennis:X.Y.Z --repo jameshartt/jim-dot-tennis

# A downloaded binary
gh attestation verify jim-dot-tennis_X.Y.Z_linux_amd64 --repo jameshartt/jim-dot-tennis

# The checksum file (a plain integrity check, no GitHub account needed)
sha256sum --check --ignore-missing jim-dot-tennis_X.Y.Z_checksums.txt
```

A successful `attestation verify` names the repository, the workflow
(`.github/workflows/release.yml`) and the tag. The image's labels
(`docker inspect`, `org.opencontainers.image.*`) give source, version, revision
and licence.

## Notes

- Versions are semantic. Pre-releases are `v0.x` until 1.0.0 (planned when the
  club-readiness programme closes); use `-rc.N` for release candidates.
- The workflow uses only the built-in `GITHUB_TOKEN`; there are no secrets to
  rotate. Only the jobs that publish have write permissions.
- Third-party actions are pinned to commit SHAs and updated by Dependabot; Go
  and base image versions are read from `go.mod` and the `Dockerfile`.
- To check the release build without publishing, build the same way the workflow
  does, e.g. `docker buildx build --target binary --platform linux/arm64
  --build-arg VERSION=0.0.0-test -o type=local,dest=out .`
