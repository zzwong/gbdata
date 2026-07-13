# Output contracts

`gbdata` output is intended for scripts as well as people. Canonical records carry `schema_version`; the current version is **3**. Before 1.0, an incompatible output change increments this number and the project's minor release version.

Commands return exit status 0 on success and nonzero on parsing, validation, file, or output errors. Diagnostics go to standard error. Customer data can appear in normal command output, so redirect it only to appropriately protected destinations.

## `inspect`

Emits one JSON object:

```json
{
  "schema_version": 3,
  "title": "Green Button Usage Feed",
  "updated": "2026-01-02T00:00:00Z",
  "entries": 4,
  "resources": {"IntervalBlock": 1, "MeterReading": 1}
}
```

`title` and `updated` are copied from the input and may be sensitive.

## `validate`

Emits one JSON object. `issues` is always an array, including when empty.

```json
{
  "valid": true,
  "issues": []
}
```

Warnings do not make the document invalid. Errors produce `valid: false` and a nonzero exit status after the JSON result is written.

Both conversion formats write to standard output by default. `--output FILE` uses an owner-only atomic file and refuses to replace an existing destination.

## `convert --format json`

Emits JSON Lines: one compact canonical record per line. It does not emit a surrounding array. Consumers should use `schema_version` rather than infer fields from a project release number.

## `convert --format csv`

The header is:

```text
schema_version,normalization_profile,usage_point,meter_reading,start,duration_seconds,value,unit,raw_value,power_of_ten_multiplier,uom,commodity,flow_direction,quality
```

`quality` contains a JSON array within the CSV cell. Timestamps are UTC RFC 3339 values. Decimal `value` fields remain strings.

## `summarize`

Emits record count, interval boundaries, and exact totals grouped by unit. For an empty feed, `start` and `end` are `null` and `totals` is empty.

```json
{
  "schema_version": 3,
  "records": 0,
  "start": null,
  "end": null,
  "totals": {}
}
```

Totals with different units are never combined.

## `diff`

Emits sorted canonical record keys in three arrays:

```json
{
  "added": [],
  "removed": [],
  "changed": []
}
```

A key consists of usage point, meter reading, start, and duration separated by `|`. A record is changed when any canonical value or metadata field differs. Duplicate canonical keys are rejected rather than silently overwritten.
