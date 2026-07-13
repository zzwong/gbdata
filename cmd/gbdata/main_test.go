package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "usage.xml")
	for _, args := range [][]string{{"inspect", fixture}, {"validate", fixture}, {"summarize", fixture}, {"convert", "--format", "json", fixture}, {"diff", fixture, fixture}, {"version"}, {"help"}} {
		var out, stderr bytes.Buffer
		if err := run(args, &out, &stderr); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if out.Len() == 0 {
			t.Fatalf("%v produced no output", args)
		}
	}
}

func TestValidateOutputUsesStableEmptyArray(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "usage.xml")
	var out bytes.Buffer
	if err := run([]string{"validate", fixture}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var result validationOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.Issues == nil || len(result.Issues) != 0 {
		t.Fatalf("unexpected validation output: %s", out.String())
	}
}

func TestConvertCSVContract(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "opower-live-shape-v1.xml")
	output := filepath.Join(t.TempDir(), "usage.csv")
	if err := run([]string{"convert", "--format", "csv", "--output", output, fixture}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(output)
		if info.Mode().Perm() != 0600 {
			t.Fatalf("mode=%#o", info.Mode().Perm())
		}
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16 {
		t.Fatalf("rows=%d", len(rows))
	}
	wantHeader := "schema_version,normalization_profile,usage_point,meter_reading,start,duration_seconds,value,unit,raw_value,power_of_ten_multiplier,uom,commodity,flow_direction,quality"
	if strings.Join(rows[0], ",") != wantHeader {
		t.Fatalf("header=%q", rows[0])
	}
	if rows[1][0] != "3" || rows[1][1] != "opower-millikwh-v1" || rows[1][7] != "kWh" {
		t.Fatalf("first row=%q", rows[1])
	}
}

func TestSummarizeEmptyFeed(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "empty.xml")
	if err := os.WriteFile(fixture, []byte(`<feed xmlns="http://www.w3.org/2005/Atom"/>`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"summarize", fixture}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var result summaryOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Records != 0 || result.Start != nil || result.End != nil || len(result.Totals) != 0 {
		t.Fatalf("unexpected empty summary: %s", out.String())
	}
}

func TestDiffDetectsSemanticChange(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", "testdata", "usage.xml"))
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(original, []byte("<espi:value>1250</espi:value>"), []byte("<espi:value>1251</espi:value>"), 1)
	second := filepath.Join(t.TempDir(), "changed.xml")
	if err := os.WriteFile(second, changed, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"diff", filepath.Join("..", "..", "testdata", "usage.xml"), second}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var result diffOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Changed) != 1 || result.Added == nil || result.Removed == nil {
		t.Fatalf("unexpected diff: %s", out.String())
	}
}

func TestAnonymizeWritesSecurelyWithoutReplacement(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "usage.xml")
	directory := t.TempDir()
	output := filepath.Join(directory, "safe.xml")
	if err := run([]string{"anonymize", "--output", output, fixture}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(output)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("mode=%#o", info.Mode().Perm())
		}
	}
	if err := run([]string{"anonymize", "--output", output, fixture}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("replaced existing output")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, after) {
		t.Fatal("existing output changed")
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".gbdata-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestAtomicWriterCleansUpAfterFailure(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "failed.xml")
	err := writeFileExclusive(output, func(_ io.Writer) error { return errors.New("stop") })
	if err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output exists after failure: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(directory, ".gbdata-*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}
