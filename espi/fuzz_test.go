package espi

import (
	"bytes"
	"os"
	"testing"
)

func FuzzParse(f *testing.F) {
	for _, path := range []string{"../testdata/usage.xml", "../testdata/opower-live-shape-v1.xml"} {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("<feed>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(bytes.NewReader(data))
	})
}

func FuzzAnonymize(f *testing.F) {
	data, err := os.ReadFile("../testdata/usage.xml")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte("not xml"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var output bytes.Buffer
		_ = Anonymize(bytes.NewReader(data), &output)
	})
}
