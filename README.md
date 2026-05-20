# vt-backup

[![CI](https://github.com/kairoaraujo/vt-backup/actions/workflows/ci.yml/badge.svg)](https://github.com/kairoaraujo/vt-backup/actions/workflows/ci.yml)
[![Release](https://github.com/kairoaraujo/vt-backup/actions/workflows/release.yml/badge.svg)](https://github.com/kairoaraujo/vt-backup/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kairoaraujo/vt-backup)](https://goreportcard.com/report/github.com/kairoaraujo/vt-backup)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

CLI tool for backups of an OpenLink Virtuoso server.

Single static Go binary. Drives Virtuoso through the bundled `isql` CLI, so the
only runtime dependency is Virtuoso itself.

## Install

Download the binary for your platform from the
[releases page](https://github.com/kairoaraujo/vt-backup/releases) and put it on
`$PATH`:

```bash
sudo install -m 0755 vt-backup-linux-amd64 /usr/local/bin/vt-backup
```

`isql` is auto-detected in common Virtuoso install dirs. Otherwise pass
`--isql /absolute/path/to/isql`.

## Usage

```bash
vt-backup full        --password-file /etc/virtuoso/dba.pwd
vt-backup incremental --password-file /etc/virtuoso/dba.pwd --require-full
vt-backup restore-cmd --week 202519
```

Exit codes: `0` ok, `1` backup failed, `2` usage error.

See `vt-backup --help` and `vt-backup <command> --help` for all flags.

## Build

```bash
just build           # native
just build-all       # all platforms
just test            # race tests
just lint            # golangci-lint v2
just check           # vet + lint + tests
```

## Releases

Releases are cut by pushing a semver tag; [GoReleaser](https://goreleaser.com)
builds the cross-platform binaries and publishes them to the GitHub releases
page:

```bash
git tag v1.2.3
git push origin v1.2.3
```

Every release ships with supply-chain metadata:

- **Checksums** signed keylessly with [Sigstore](https://www.sigstore.dev/)
  `cosign`, emitted as a `*_checksums.txt.bundle` (signature + certificate +
  transparency-log entry).
- **SBOMs** (CycloneDX) generated per archive.
- **Build provenance attestations** for every archive and the checksums file.

### Verify a download

Verify the signed checksums:

```bash
cosign verify-blob \
  --bundle vt-backup_<version>_checksums.txt.bundle \
  --certificate-identity-regexp 'https://github.com/kairoaraujo/vt-backup/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  vt-backup_<version>_checksums.txt
```

Verify an archive's build provenance:

```bash
gh attestation verify vt-backup_<version>_linux_amd64.tar.gz --owner kairoaraujo
```

## License

MIT — see [LICENSE](LICENSE).
