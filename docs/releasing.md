# Releasing

Releases are built from tags by GitHub Actions and GoReleaser. Do not upload locally built archives to an existing release and never replace published artifacts in place.

## Preflight

```bash
git status --short
go mod verify
make check
govulncheck ./...
goreleaser check
goreleaser release --snapshot --clean
```

Inspect the complete diff for customer data and secrets. Confirm fixtures contain only synthetic values and that the current canonical schema version is documented in `docs/output.md`.

Verify snapshot checksums and inspect every archive family for:

- The correct `gbdata` executable
- `LICENSE`, `NOTICE`, and `README.md`
- `third_party_licenses/golang_go.LICENSE`

Run a snapshot binary against both committed XML fixtures and a ZIP fixture. Confirm `gbdata version` contains the expected version.

## Publish

1. Ensure `main` is green and the working tree is clean.
2. Choose a semantic version. Before 1.0, incompatible CLI, Go API, or canonical-schema changes require a minor release.
3. Create a signed annotated tag, for example:

   ```bash
   git tag -s v0.1.0 -m 'v0.1.0'
   git tag -v v0.1.0
   ```

4. Push only the tag: `git push origin v0.1.0`.
5. Verify the Release workflow, checksums, and GitHub build-provenance attestations.
6. Download one archive and verify it:

   ```bash
   gh attestation verify gbdata_*.tar.gz --repo zzwong/gbdata
   sha256sum -c checksums.txt
   ```

7. Smoke-test the downloaded binary before announcing the release.

If a release is compromised or materially incorrect, remove affected artifacts, issue a security advisory when appropriate, and publish a new version. Do not reuse a tag or silently replace an artifact.
