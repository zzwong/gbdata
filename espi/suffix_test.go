package espi

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseRejectsFeedSuffix(t *testing.T) {
	const feed = `<feed xmlns="http://www.w3.org/2005/Atom"/>`
	for _, tc := range []struct {
		name   string
		suffix string
	}{
		{"second feed", feed},
		{"other element", `<CANARY_SUFFIX/>`},
		{"truncated element", `<CANARY_SUFFIX`},
		{"unmatched close", `</CANARY_SUFFIX>`},
		{"text", `CANARY_SUFFIX`},
		{"malformed comment", `<!--CANARY_SUFFIX`},
		{"malformed PI", `<?CANARY_SUFFIX`},
		{"PI missing separator", `<?ok=CANARY_SUFFIX?>`},
		{"PI slash separator", `<?ok/CANARY_SUFFIX?>`},
		{"PI non XML separator", "<?ok\u00a0CANARY_SUFFIX?>"},
		{"PI invalid character", "<?ok \x00?>"},
		{"PI invalid UTF8", "<?ok \xff?>"},
		{"comment invalid character", "<!--\x00-->"},
		{"comment invalid UTF8", "<!--\xff-->"},
		{"comment forbidden codepoint", "<!--\ufffe-->"},
		{"unknown entity", `&CANARY_SUFFIX;`},
		{"escaped whitespace", `&#32;`},
		{"CDATA whitespace", `<![CDATA[ ]]>`},
		{"empty CDATA", `<![CDATA[]]>`},
		{"non XML whitespace", "\u00a0"},
		{"declaration", `<?xml version="1.0"?>`},
		{"reserved PI target", `<?XmL CANARY_SUFFIX?>`},
		{"directive", `<!DOCTYPE CANARY_SUFFIX>`},
		{"invalid after Misc", " \n<!--ok--><?ok?>&CANARY_SUFFIX;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse(strings.NewReader(feed + tc.suffix))
			if err == nil {
				t.Fatal("accepted invalid feed suffix")
			}
			if err.Error() != "parse ESPI XML: invalid content after Atom feed" {
				t.Fatalf("unexpected or unsanitized error: %v", err)
			}
			if !reflect.DeepEqual(doc, Document{}) {
				t.Fatal("returned a partial document")
			}
		})
	}
}

func TestParseAllowsTrailingMisc(t *testing.T) {
	for _, path := range []string{"../testdata/usage.xml", "../testdata/opower-live-shape-v1.xml"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		original, err := Parse(strings.NewReader(string(data)))
		if err != nil {
			t.Fatal(err)
		}
		want, err := Normalize(original)
		if err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"", " \t\r\n", "<!--ok-->", "<?ok?>", "<?ok \t?>", "<!--\u03b1\U0001f600--><?ok \u03b1\U0001f600?>", `<?xml-stylesheet href="example.invalid"?>`, " \n<!--ok--><?ok data?>\t"} {
			doc, err := Parse(strings.NewReader(string(data) + suffix))
			if err != nil {
				t.Fatalf("%s: valid trailing Misc rejected: %v", path, err)
			}
			got, err := Normalize(doc)
			if err != nil || !reflect.DeepEqual(doc, original) || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: trailing Misc changed document or records: %v", path, err)
			}
		}
	}
}
