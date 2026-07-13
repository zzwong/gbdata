# Contributing

Thank you for helping improve `gbdata`.

## Before opening an issue

Search existing issues and confirm the behavior with the latest release or `main`. Bug reports should include the operating system, `gbdata version`, the command used, expected behavior, and sanitized output.

Do not attach an original Green Button file. Utility exports may contain names, addresses, account and meter identifiers, detailed occupancy patterns, costs, and access URLs. Prefer a minimal synthetic reproduction. Output from `gbdata anonymize` must still be reviewed manually before sharing.

## Development

Go 1.25.12 or newer is required.

```bash
git clone https://github.com/zzwong/gbdata.git
cd gbdata
make check
govulncheck ./...
```

Keep changes focused. Parser changes should include tests for valid input, malformed input, optional fields, and additive resources where applicable. Changes to JSON or CSV must update `docs/output.md` and schema-version tests.

Fixtures must be synthetic or sanitized so no production identifiers, timestamps, readings, costs, addresses, free text, tokens, signed URLs, or provider response values remain. Describe fixture provenance without identifying a customer.

## Pull requests

Before submitting:

```bash
make check
govulncheck ./...
goreleaser check
```

Explain the user-visible behavior and testing performed. Substantial new resource types or compatibility profiles should be discussed in an issue first. Provider-specific behavior must fail closed behind a precise, tested signature rather than changing generic ESPI semantics.

Contributions are licensed under Apache-2.0.
