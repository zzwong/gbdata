package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandsRejectFeedSuffix(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "usage.xml")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	input := filepath.Join(directory, "invalid.xml")
	output := filepath.Join(directory, "output.ndjson")
	if err := os.WriteFile(input, append(data, []byte("&CANARY_SUFFIX;")...), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"inspect", input}, {"validate", input}, {"summarize", input},
		{"convert", input}, {"convert", "--output", output, input},
		{"diff", input, fixture}, {"diff", fixture, input},
	} {
		var out, stderr bytes.Buffer
		err := run(args, &out, &stderr)
		if err == nil || err.Error() != "parse ESPI XML: invalid content after Atom feed" {
			t.Fatalf("%v: unexpected or unsanitized error: %v", args, err)
		}
		if out.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: unexpected output", args)
		}
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output exists after parsing failed: %v", err)
	}
}
