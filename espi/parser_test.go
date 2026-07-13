package espi

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestParseNormalizeAndValidate(t *testing.T) {
	f, err := os.Open("../testdata/usage.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	d, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Entries) != 4 || len(Validate(d)) != 0 {
		t.Fatalf("entries=%d issues=%#v", len(d.Entries), Validate(d))
	}
	records, err := Normalize(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Value != "1.25" || records[0].Unit != "Wh" || records[0].MeterReading != "mr1" || records[0].UsagePoint != "up1" {
		t.Fatalf("%#v", records)
	}
}
func TestOpowerRealShapeFixture(t *testing.T) {
	file, err := os.Open("../testdata/opower-live-shape-v1.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	doc, err := Parse(file)
	if err != nil {
		t.Fatal(err)
	}
	records, err := Normalize(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 6 || len(records) != 15 {
		t.Fatalf("entries=%d records=%d", len(doc.Entries), len(records))
	}
	total := new(big.Rat)
	for _, record := range records {
		value, ok := new(big.Rat).SetString(record.Value)
		if !ok {
			t.Fatalf("invalid decimal %q", record.Value)
		}
		total.Add(total, value)
		if record.PowerOfTenMultiplier != 3 || record.Unit != "kWh" || record.NormalizationProfile != NormalizationProfileOpowerMilliKWh {
			t.Fatalf("unexpected Opower record: %#v", record)
		}
	}
	if records[0].RawValue != 300000 || records[0].Value != "300" || total.RatString() != "7030" {
		t.Fatalf("unexpected normalized values: first=%#v total=%s", records[0], total.RatString())
	}
}

func TestOpowerProfileRequiresExactSignature(t *testing.T) {
	value, unit, profile, err := normalizedReading("Another Feed", ReadingType{UOM: 72, PowerOfTenMultiplier: 3}, 1250)
	if err != nil {
		t.Fatal(err)
	}
	if value != "1250000" || unit != "Wh" || profile != "" {
		t.Fatalf("unexpected generic normalization: %q %q %q", value, unit, profile)
	}
}

func TestScaleDecimal(t *testing.T) {
	for _, tc := range []struct {
		v int64
		m int
		w string
	}{{1250, -3, "1.25"}, {1, -3, "0.001"}, {-12, -1, "-1.2"}, {12, 2, "1200"}, {0, -3, "0"}} {
		got, err := ScaleDecimal(tc.v, tc.m)
		if err != nil {
			t.Errorf("%d %d: %v", tc.v, tc.m, err)
			continue
		}
		if got != tc.w {
			t.Errorf("%d %d = %s", tc.v, tc.m, got)
		}
	}
}
func TestInvalidXML(t *testing.T) {
	for _, input := range []string{
		"<feed>",
		"<feed xmlns=\"urn:not-atom\"/>",
		"<not-feed xmlns=\"http://www.w3.org/2005/Atom\"/>",
	} {
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted invalid XML %q", input)
		}
	}
}

func TestRejectsMultipleResourcesInOneEntry(t *testing.T) {
	input := `<feed xmlns="http://www.w3.org/2005/Atom" xmlns:espi="http://naesb.org/espi"><entry><id>x</id><content><espi:UsagePoint/><espi:MeterReading/></content></entry></feed>`
	if _, err := Parse(strings.NewReader(input)); err == nil {
		t.Fatal("accepted multiple resources in one entry")
	}
}

func TestValidationRejectsUnsafeNumericRanges(t *testing.T) {
	doc := Document{Entries: []Entry{
		{ID: "type", Resource: ReadingType{PowerOfTenMultiplier: MaxPowerOfTenMultiplier + 1}},
		{ID: "block", Resource: IntervalBlock{Readings: []IntervalReading{{TimePeriod: DateTimeInterval{Start: math.MaxInt64, Duration: math.MaxInt64}}}}},
	}}
	issues := Validate(doc)
	if len(issues) < 3 {
		t.Fatalf("expected multiplier, duration, time, and relationship issues: %#v", issues)
	}
	if _, err := Normalize(doc); err == nil {
		t.Fatal("normalized invalid numeric ranges")
	}
	if _, err := ScaleDecimal(1, MaxPowerOfTenMultiplier+1); err == nil {
		t.Fatal("scaled an unbounded multiplier")
	}
}

func TestNormalizeRequiresRelationships(t *testing.T) {
	doc := Document{Entries: []Entry{{ID: "block", Resource: IntervalBlock{Readings: []IntervalReading{{TimePeriod: DateTimeInterval{Start: 1, Duration: 1}}}}}}}
	if _, err := Normalize(doc); err == nil || !strings.Contains(err.Error(), "MeterReading") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidationDetectsDuplicateLinksAndIntervals(t *testing.T) {
	meterHref := "https://example.invalid/espi/1_1/resource/UsagePoint/u/MeterReading/m"
	block := IntervalBlock{Readings: []IntervalReading{{TimePeriod: DateTimeInterval{Start: 1, Duration: 60}}}}
	doc := Document{Entries: []Entry{
		{ID: "meter-a", Links: []Link{{Rel: "self", Href: meterHref}}, Resource: MeterReading{}},
		{ID: "meter-b", Links: []Link{{Rel: "self", Href: meterHref}}, Resource: MeterReading{}},
		{ID: "block-a", Links: []Link{{Rel: "up", Href: meterHref + "/IntervalBlock/a"}}, Resource: block},
		{ID: "block-b", Links: []Link{{Rel: "up", Href: meterHref + "/IntervalBlock/b"}}, Resource: block},
	}}
	issues := Validate(doc)
	messages := fmt.Sprint(issues)
	if !strings.Contains(messages, "duplicate self link") || !strings.Contains(messages, "duplicate interval reading") {
		t.Fatalf("missing duplicate issues: %#v", issues)
	}
}
func TestAnonymizeRemovesEveryTextChannel(t *testing.T) {
	input := `<?xml version="1.0"?><?secret CANARY_PI?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:espi="http://naesb.org/espi"><!--CANARY_COMMENT--><title>CANARY_TITLE</title><entry private="CANARY_ATTRIBUTE"><id>CANARY_ID</id><link rel="self" href="https://utility.example/espi/1_1/resource/UsagePoint/account123?token=CANARY_QUERY"/><content><espi:UsagePoint><espi:description>CANARY_TEXT</espi:description></espi:UsagePoint></content></entry></feed>`
	var first, second bytes.Buffer
	if err := Anonymize(strings.NewReader(input), &first); err != nil {
		t.Fatal(err)
	}
	if err := Anonymize(strings.NewReader(input), &second); err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{"CANARY", "utility.example", "account123"} {
		if strings.Contains(first.String(), canary) {
			t.Fatalf("anonymized output leaked %q", canary)
		}
	}
	if !strings.Contains(first.String(), "example.invalid") {
		t.Fatal("URL authority was not replaced")
	}
	if first.String() == second.String() {
		t.Fatal("separate anonymization runs reused pseudonyms")
	}
	if _, err := Parse(bytes.NewReader(first.Bytes())); err != nil {
		t.Fatalf("anonymized canary XML invalid: %v", err)
	}
}

func TestAnonymizeRealShapeFixture(t *testing.T) {
	data, err := os.ReadFile("../testdata/opower-live-shape-v1.xml")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Anonymize(bytes.NewReader(data), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), ">300000<") {
		t.Fatal("anonymized output retained a reading")
	}
	if _, err := Parse(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatalf("anonymized real-shape XML invalid: %v", err)
	}
}

func TestAnonymizeRemovesSensitiveValues(t *testing.T) {
	data, _ := os.ReadFile("../testdata/usage.xml")
	var out bytes.Buffer
	if err := Anonymize(bytes.NewReader(data), &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, s := range []string{"usage-1", "up1", "mr1", ">1250<", ">1767225600<"} {
		if strings.Contains(text, s) {
			t.Fatalf("leaked %q", s)
		}
	}
	if _, err := Parse(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatalf("anonymized XML invalid: %v", err)
	}
}
