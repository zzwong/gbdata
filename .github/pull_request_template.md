## Summary

## Testing

- [ ] `make check`
- [ ] `govulncheck ./...`
- [ ] Output changes update `docs/output.md` and schema-version tests

## Security and privacy

- [ ] No credentials, tokens, signed URLs, customer identifiers, addresses, original exports, readings, costs, HAR files, or production response values are included
- [ ] Fixtures are synthetic or reviewed after sanitization
- [ ] Sensitive file output remains owner-only, atomic, and non-replacing
- [ ] Provider-specific behavior is guarded by a precise compatibility signature
