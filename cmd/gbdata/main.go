package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zzwong/gbdata/espi"
	"github.com/zzwong/gbdata/internal/input"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
func run(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return usage(errOut)
	}
	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(out, "gbdata", version)
		return err
	case "inspect":
		return inspect(args[1:], out)
	case "validate":
		return validate(args[1:], out)
	case "convert":
		return convert(args[1:], out)
	case "summarize":
		return summarize(args[1:], out)
	case "anonymize":
		return anonymize(args[1:])
	case "diff":
		return diff(args[1:], out)
	case "help", "--help", "-h":
		return usage(out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func usage(w io.Writer) error {
	_, err := fmt.Fprint(w, `gbdata inspects and converts Green Button ESPI files.

Usage:
  gbdata inspect FILE
  gbdata validate FILE
  gbdata convert [--format json|csv] [--output FILE] INPUT
  gbdata summarize FILE
  gbdata anonymize --output FILE INPUT
  gbdata diff OLD NEW
  gbdata version

FILE may be an Atom XML document or a ZIP containing exactly one XML file.
JSON conversion emits one object per line.
`)
	return err
}
func document(path string) (espi.Document, error) {
	r, err := input.Open(path)
	if err != nil {
		return espi.Document{}, err
	}
	document, parseErr := espi.Parse(r)
	closeErr := r.Close()
	if parseErr != nil {
		return espi.Document{}, parseErr
	}
	if closeErr != nil {
		return espi.Document{}, fmt.Errorf("close input: %w", closeErr)
	}
	return document, nil
}

type inspectOutput struct {
	SchemaVersion int            `json:"schema_version"`
	Title         string         `json:"title"`
	Updated       string         `json:"updated"`
	Entries       int            `json:"entries"`
	Resources     map[string]int `json:"resources"`
}

type validationOutput struct {
	Valid  bool         `json:"valid"`
	Issues []espi.Issue `json:"issues"`
}
type summaryOutput struct {
	SchemaVersion int               `json:"schema_version"`
	Records       int               `json:"records"`
	Start         *time.Time        `json:"start"`
	End           *time.Time        `json:"end"`
	Totals        map[string]string `json:"totals"`
}
type diffOutput struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

func inspect(args []string, w io.Writer) error {
	if len(args) != 1 {
		return errors.New("inspect requires one XML or ZIP file")
	}
	d, err := document(args[0])
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, e := range d.Entries {
		if e.Resource != nil {
			counts[e.Resource.ResourceType()]++
		}
	}
	return writeJSON(w, inspectOutput{SchemaVersion: espi.SchemaVersion, Title: d.Title, Updated: d.Updated, Entries: len(d.Entries), Resources: counts})
}
func validate(args []string, w io.Writer) error {
	if len(args) != 1 {
		return errors.New("validate requires one XML or ZIP file")
	}
	d, err := document(args[0])
	if err != nil {
		return err
	}
	issues := espi.Validate(d)
	if err := writeJSON(w, validationOutput{Valid: !hasErrors(issues), Issues: issues}); err != nil {
		return err
	}
	for _, i := range issues {
		if i.Severity == "error" {
			return errors.New("validation failed")
		}
	}
	return nil
}
func hasErrors(issues []espi.Issue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func convert(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	format := fs.String("format", "json", "json or csv")
	output := fs.String("output", "", "owner-only output path (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("convert requires one XML or ZIP file")
	}
	d, err := document(fs.Arg(0))
	if err != nil {
		return err
	}
	records, err := espi.Normalize(d)
	if err != nil {
		return err
	}
	writeRecords := func(writer io.Writer) error {
		switch *format {
		case "json":
			encoder := json.NewEncoder(writer)
			for _, record := range records {
				if err := encoder.Encode(record); err != nil {
					return err
				}
			}
			return nil
		case "csv":
			return writeCSV(writer, records)
		default:
			return errors.New("format must be json or csv")
		}
	}
	if *output != "" {
		return writeFileExclusive(*output, writeRecords)
	}
	return writeRecords(w)
}
func writeCSV(w io.Writer, records []espi.Record) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"schema_version", "normalization_profile", "usage_point", "meter_reading", "start", "duration_seconds", "value", "unit", "raw_value", "power_of_ten_multiplier", "uom", "commodity", "flow_direction", "quality"}); err != nil {
		return err
	}
	for _, r := range records {
		quality := r.Quality
		if quality == nil {
			quality = []int{}
		}
		q, err := json.Marshal(quality)
		if err != nil {
			return err
		}
		row := []string{strconv.Itoa(r.SchemaVersion), r.NormalizationProfile, r.UsagePoint, r.MeterReading, r.Start.Format(time.RFC3339), strconv.FormatInt(r.DurationSeconds, 10), r.Value, r.Unit, strconv.FormatInt(r.RawValue, 10), strconv.Itoa(r.PowerOfTenMultiplier), strconv.Itoa(r.UOM), strconv.Itoa(r.Commodity), strconv.Itoa(r.FlowDirection), string(q)}
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
func summarize(args []string, w io.Writer) error {
	if len(args) != 1 {
		return errors.New("summarize requires one XML or ZIP file")
	}
	d, err := document(args[0])
	if err != nil {
		return err
	}
	records, err := espi.Normalize(d)
	if err != nil {
		return err
	}
	totals := map[string]*big.Rat{}
	var min, max time.Time
	for _, r := range records {
		x, ok := new(big.Rat).SetString(r.Value)
		if !ok {
			return fmt.Errorf("normalized record contains invalid decimal %q", r.Value)
		}
		if totals[r.Unit] == nil {
			totals[r.Unit] = new(big.Rat)
		}
		totals[r.Unit].Add(totals[r.Unit], x)
		if min.IsZero() || r.Start.Before(min) {
			min = r.Start
		}
		end := r.Start.Add(time.Duration(r.DurationSeconds) * time.Second)
		if end.After(max) {
			max = end
		}
	}
	display := map[string]string{}
	for unit, total := range totals {
		display[unit] = decimalString(total)
	}
	result := summaryOutput{SchemaVersion: espi.SchemaVersion, Records: len(records), Totals: display}
	if len(records) > 0 {
		result.Start = &min
		result.End = &max
	}
	return writeJSON(w, result)
}
func decimalString(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	// ESPI scaling is bounded to twelve decimal places, so this is exact.
	return strings.TrimRight(strings.TrimRight(value.FloatString(12), "0"), ".")
}

func anonymize(args []string) error {
	fs := flag.NewFlagSet("anonymize", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	output := fs.String("output", "", "output XML path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || *output == "" {
		return errors.New("anonymize requires an input file and --output")
	}
	in, err := input.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	writeErr := writeFileExclusive(*output, func(writer io.Writer) error { return espi.Anonymize(in, writer) })
	closeErr := in.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close input: %w", closeErr)
	}
	return nil
}
func writeFileExclusive(destination string, write func(io.Writer) error) error {
	directory := filepath.Dir(destination)
	temporary, err := os.CreateTemp(directory, ".gbdata-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary output: %w", err)
	}
	if err := write(temporary); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := os.Link(temporaryName, destination); err != nil {
		return fmt.Errorf("publish output without replacing an existing file: %w", err)
	}
	if err := os.Remove(temporaryName); err != nil {
		return fmt.Errorf("remove temporary output: %w", err)
	}
	return nil
}

func diff(args []string, w io.Writer) error {
	if len(args) != 2 {
		return errors.New("diff requires two XML or ZIP files")
	}
	a, err := document(args[0])
	if err != nil {
		return err
	}
	b, err := document(args[1])
	if err != nil {
		return err
	}
	ar, err := espi.Normalize(a)
	if err != nil {
		return err
	}
	br, err := espi.Normalize(b)
	if err != nil {
		return err
	}
	am, err := recordMap(ar)
	if err != nil {
		return fmt.Errorf("first document: %w", err)
	}
	bm, err := recordMap(br)
	if err != nil {
		return fmt.Errorf("second document: %w", err)
	}
	var added, removed, changed []string
	for k, v := range am {
		if n, ok := bm[k]; !ok {
			removed = append(removed, k)
		} else if !recordsEqual(n, v) {
			changed = append(changed, k)
		}
	}
	for k := range bm {
		if _, ok := am[k]; !ok {
			added = append(added, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	if added == nil {
		added = []string{}
	}
	if removed == nil {
		removed = []string{}
	}
	if changed == nil {
		changed = []string{}
	}
	return writeJSON(w, diffOutput{Added: added, Removed: removed, Changed: changed})
}
func recordMap(records []espi.Record) (map[string]espi.Record, error) {
	result := map[string]espi.Record{}
	for _, record := range records {
		key := record.UsagePoint + "|" + record.MeterReading + "|" + record.Start.Format(time.RFC3339) + "|" + strconv.FormatInt(record.DurationSeconds, 10)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate canonical record %q", key)
		}
		result[key] = record
	}
	return result, nil
}
func recordsEqual(a, b espi.Record) bool {
	return a.SchemaVersion == b.SchemaVersion && a.NormalizationProfile == b.NormalizationProfile &&
		a.UsagePoint == b.UsagePoint && a.MeterReading == b.MeterReading && a.Start.Equal(b.Start) &&
		a.DurationSeconds == b.DurationSeconds && a.RawValue == b.RawValue &&
		a.PowerOfTenMultiplier == b.PowerOfTenMultiplier && a.Value == b.Value && a.UOM == b.UOM &&
		a.Unit == b.Unit && a.Commodity == b.Commodity && a.FlowDirection == b.FlowDirection &&
		slices.Equal(a.Quality, b.Quality)
}
func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
