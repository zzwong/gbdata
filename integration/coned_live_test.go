package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/zzwong/gbdata/espi"
)

// TestLiveConEdisonCompatibility compares two read-only representations from
// the authenticated local coned CLI. It deliberately logs no customer data.
func TestLiveConEdisonCompatibility(t *testing.T) {
	if os.Getenv("GBDATA_LIVE_CONED_TEST") != "1" {
		t.Skip("set GBDATA_LIVE_CONED_TEST=1 to run")
	}
	binary := os.Getenv("CONED_BIN")
	if binary == "" {
		binary = "coned"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	xmlData, err := exec.CommandContext(ctx, binary, "green-button", "export", "--format", "xml").Output()
	if err != nil {
		t.Fatalf("Green Button export failed: %v", err)
	}
	doc, err := espi.Parse(bytes.NewReader(xmlData))
	if err != nil {
		t.Fatalf("parse Green Button export: %v", err)
	}
	records, err := espi.Normalize(doc)
	if err != nil {
		t.Fatalf("normalize Green Button export: %v", err)
	}

	readsData, err := exec.CommandContext(ctx, binary, "--json", "usage", "reads", "--aggregate", "bill").Output()
	if err != nil {
		t.Fatalf("historical reads failed: %v", err)
	}
	var reads []struct {
		Start, End string
		Value      json.Number
	}
	decoder := json.NewDecoder(bytes.NewReader(readsData))
	decoder.UseNumber()
	if err := decoder.Decode(&reads); err != nil {
		t.Fatalf("decode historical reads: %v", err)
	}
	if len(records) != len(reads) {
		t.Fatalf("record count differs: XML=%d API=%d", len(records), len(reads))
	}
	if len(records) == 0 {
		t.Fatal("no usage records returned")
	}

	xmlTotal := new(big.Rat)
	for _, record := range records {
		if record.NormalizationProfile != espi.NormalizationProfileOpowerMilliKWh {
			t.Fatalf("compatibility profile not applied")
		}
		value, ok := new(big.Rat).SetString(record.Value)
		if !ok {
			t.Fatal("invalid normalized decimal")
		}
		xmlTotal.Add(xmlTotal, value)
	}
	apiTotal := new(big.Rat)
	for _, read := range reads {
		value, ok := new(big.Rat).SetString(read.Value.String())
		if !ok {
			t.Fatal("invalid API decimal")
		}
		apiTotal.Add(apiTotal, value)
	}
	if xmlTotal.Cmp(apiTotal) != 0 {
		t.Fatalf("aggregate totals differ")
	}

	sort.Slice(reads, func(i, j int) bool { return reads[i].Start < reads[j].Start })
	firstXMLDate := records[0].Start.Format("2006-01-02")
	lastRecord := records[len(records)-1]
	lastXMLEndDate := lastRecord.Start.Add(time.Duration(lastRecord.DurationSeconds) * time.Second).Format("2006-01-02")
	if firstXMLDate != reads[0].Start[:10] || lastXMLEndDate != reads[len(reads)-1].End[:10] {
		t.Fatal("billing-period boundaries differ")
	}
}
