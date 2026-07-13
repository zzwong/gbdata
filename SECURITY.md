# Security Policy

## Reporting a vulnerability

Use GitHub's private security-advisory feature for vulnerabilities. If that is unavailable, use the private contact method on the maintainer's GitHub profile. Do not open a public issue before a report has been assessed.

Do not send real utility exports, credentials, OAuth tokens, account or meter identifiers, addresses, bills, signed URLs, HAR files, or raw provider responses. Build the smallest synthetic reproduction possible.

Relevant reports include:

- XML or ZIP denial of service
- Archive traversal, symlink, encryption, or size-limit bypasses
- Redaction leaks through text, attributes, URLs, comments, or processing instructions
- Unsafe file replacement, permissions, or partial-output behavior
- Incorrect normalization that materially changes energy values
- Release-workflow or artifact-integrity weaknesses

Receipt should be acknowledged within seven days. Timing for a fix and disclosure depends on severity and reproducibility. Please allow a reasonable remediation period before public disclosure.

## Supported versions

Before 1.0, security fixes are provided for the latest released minor version. Older pre-1.0 releases may not receive backports.

## Data-handling boundary

`gbdata` processes local files and performs no network requests. Its output can contain sensitive usage information unless the `anonymize` command was used. Anonymization is defense in depth, not proof that a document is anonymous; document shape and retained metadata may remain identifying. Review every file before sharing it.
