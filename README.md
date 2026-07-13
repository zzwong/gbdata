# gbdata

`gbdata` is a Go library and command-line tool for inspecting, validating, converting, comparing, and redacting local Green Button ESPI files.

It reads Atom XML documents and ZIP archives that contain one XML document. It does not authenticate with utilities, call customer portals, or transmit input data.

## Features

- Parse core ESPI usage resources from bounded local input
- Normalize interval readings into chronological records
- Preserve raw integers and apply power-of-ten scaling without floating-point loss
- Convert readings to JSON Lines or CSV
- Inspect resource counts, validate relationships, summarize totals, and compare revisions
- Redact identifiers, readings, costs, timestamps, free text, URL authorities, comments, and processing instructions
- Reject malformed XML, ambiguous or unsafe ZIPs, excessive entries, and unsafe numeric ranges
- Handle the observed Opower milli-kWh convention through an explicit compatibility profile

This project is pre-1.0. CLI and Go API compatibility is documented but may change between minor versions.

## Install

After the first release:

```bash
go install github.com/zzwong/gbdata/cmd/gbdata@latest
```

Or build from a checkout:

```bash
go build -trimpath -o gbdata ./cmd/gbdata
```

Go 1.25.12 or newer is required.

## Usage

```bash
gbdata inspect usage.xml
gbdata validate usage.zip
gbdata summarize usage.xml

gbdata convert --format json --output usage.ndjson usage.xml
gbdata convert --format csv --output usage.csv usage.xml

gbdata anonymize --output redacted.xml usage.xml
gbdata diff older.xml newer.xml
```

Run `gbdata help` for the command synopsis. JSON conversion emits one object per line; other commands emit one indented JSON document. See [docs/output.md](docs/output.md) for versioned output contracts.

### Go library

```go
file, err := os.Open("usage.xml")
if err != nil {
    return err
}
defer file.Close()

document, err := espi.Parse(file)
if err != nil {
    return err
}
records, err := espi.Normalize(document)
```

```go
import "github.com/zzwong/gbdata/espi"
```

## Supported ESPI resources

`gbdata` currently interprets:

- `UsagePoint`
- `LocalTimeParameters`
- `MeterReading`
- `ReadingType`
- `IntervalBlock` and `IntervalReading`

Other resources remain visible by resource name during inspection but are not decoded into typed fields. Validation is intentionally structural and normalization-focused; it is not an XSD validator and does not replace Green Button Alliance certification tools.

## Canonical records

Each normalized record includes its schema version, relationship identifiers, UTC interval, duration, raw integer, ESPI multiplier, exact decimal value, UOM, readable unit, commodity, flow direction, reading quality, and any compatibility profile.

Decimal values are strings so conversion never introduces binary floating-point error.

### Opower compatibility

Some Opower-generated feeds identify readings as kWh with UOM 72 and `powerOfTenMultiplier=3` while encoding interval integers in milli-kWh. For feeds titled exactly `Opower ESPI Third Party Batch Feed v1`, `gbdata` converts those values to kWh and sets:

```json
"normalization_profile": "opower-millikwh-v1"
```

The exact title and metadata combination is required. Generic ESPI scaling remains unchanged. A sanitized real-shape fixture covers this behavior in CI.

An optional local integration test compares an authenticated `coned` XML export with its normalized bill readings without logging identifiers or values:

```bash
GBDATA_LIVE_CONED_TEST=1 \
CONED_BIN="$HOME/go/bin/coned" \
go test ./integration -run TestLiveConEdisonCompatibility -v
```

## Security and privacy

Inputs are limited to 100 MiB, ZIPs must contain exactly one XML file, and archives with unsafe paths, symlinks, encryption, excessive members, or oversized members are rejected. Go's XML decoder does not resolve external entities.

`convert --output` and `anonymize --output` create owner-only output through same-directory temporary files and never replace an existing destination. Standard-output redirection uses permissions chosen by your shell and umask. It uses fresh random pseudonyms for each run, preserving repeated identifiers only within that output. It removes common customer-data channels, but redaction is not a formal anonymity guarantee. Metadata and document shape can still identify a provider or unusual installation. Always review output before sharing it.

See [SECURITY.md](SECURITY.md) for vulnerability reporting and data-handling guidance.

## Development

```bash
make check
govulncheck ./...
goreleaser check
```

Fixtures must be synthetic or thoroughly sanitized. Never commit customer exports, account identifiers, addresses, access tokens, bills, or production response values.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/releasing.md](docs/releasing.md).

## Standards and trademarks

Green Button is based on the NAESB REQ.21 Energy Services Provider Interface and the Atom Syndication Format. This project is not affiliated with or certified by the Green Button Alliance, NAESB, Opower, Oracle, or any utility.

“Green Button” and related marks belong to their respective owners.

## License

Licensed under the [Apache License 2.0](LICENSE). Copyright 2026 `zzwong`.
