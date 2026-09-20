# Releases and installation

## Publish a stable release

1. Commit and push the tested changes to `main`.
2. Run `scripts/release vX.Y.Z --dry-run`.
3. Run `scripts/release vX.Y.Z`.
4. Check the GitHub Actions `release` workflow.

The helper rejects a dirty checkout, an existing tag, or a checkout that differs from remote `main`.
It pushes only the requested annotated tag. It never force-pushes tags.

The workflow runs tests and vet, then uses GoReleaser 2.17.0.
It publishes four binary archives, Debian/RPM packages, and SHA-256 checksums.
Stable releases also update `muratmirgun/homebrew-tap` from those exact checksums.
Prereleases do not replace the Homebrew formula.

## Homebrew automation

The `HOMEBREW_TAP_SSH_KEY` repository secret contains a deploy key.
Its public key has write access only to `muratmirgun/homebrew-tap`.
The release workflow uses the key to update `Formula/owncode.rb`.
No personal access token is required.

To rotate the key, replace the deploy key on the tap and update the OwnCode secret.
Never store the private key in the repository.

If tap publication fails after asset publication, regenerate the formula from that release's `checksums.txt`:

```bash
python3 scripts/homebrew-formula.py vX.Y.Z checksums.txt /tmp/owncode.rb
```

Commit that file as `Formula/owncode.rb` in the tap.
Do not replace a published release tag to repair a packaging issue.

## Local verification

```bash
bash -n install scripts/release
python3 scripts/test-install.py
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Snapshots stay under `dist/` and never publish artifacts.
The installer tests use local fixtures and do not access the network or change shell startup files.

OwnCode does not currently sign or notarize macOS binaries.
The download installer verifies checksums over HTTPS; these are integrity checks, not publisher signatures.
