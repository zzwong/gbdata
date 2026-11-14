package espi

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseInheritedNamespaces(t *testing.T) {
	for _, location := range []string{"feed", "entry", "content"} {
		t.Run(location, func(t *testing.T) {
			binding := ` xmlns:e="` + ESPINamespace + `"`
			feed, entry, content := "", "", ""
			switch location {
			case "feed":
				feed = binding
			case "entry":
				entry = binding
			case "content":
				content = binding
			}
			input := fmt.Sprintf(`<a:feed xmlns:a="%s"%s><a:title>title</a:title><a:entry%s><a:id>x</a:id><a:content%s><e:ReadingType><e:uom>72</e:uom></e:ReadingType></a:content></a:entry></a:feed>`, AtomNamespace, feed, entry, content)
			doc, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if doc.Title != "title" || len(doc.Entries) != 1 || doc.Entries[0].ID != "x" {
				t.Fatalf("%#v", doc)
			}
			if rt, ok := doc.Entries[0].Resource.(ReadingType); !ok || rt.UOM != 72 {
				t.Fatalf("%#v", doc.Entries[0].Resource)
			}
		})
	}
	input := `<feed xmlns="` + AtomNamespace + `"><entry><a:content xmlns:a="` + AtomNamespace + `" xmlns="` + ESPINamespace + `"><ReadingType><uom>72</uom></ReadingType></a:content></entry></feed>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if rt, ok := doc.Entries[0].Resource.(ReadingType); !ok || rt.UOM != 72 {
		t.Fatalf("%#v", doc)
	}
}

func TestParseForeignAtomLookalikes(t *testing.T) {
	input := `<feed xmlns="` + AtomNamespace + `" xmlns:f="urn:foreign" xmlns:e="` + ESPINamespace + `"><title>real</title><f:title>fake</f:title><updated>real-update</updated><f:updated>fake-update</f:updated><f:entry><id>fake</id></f:entry><entry><id>real-id</id><f:id>fake-id</f:id><title>real-title</title><f:title>fake-title</f:title><published>real-published</published><f:published>fake</f:published><updated>real-updated</updated><f:updated>fake</f:updated><link rel="self" href="real"/><f:link rel="self" href="fake"/><content><e:MeterReading/></content><f:content><e:ReadingType/></f:content></entry></feed>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "real" || doc.Updated != "real-update" || len(doc.Entries) != 1 {
		t.Fatalf("%#v", doc)
	}
	e := doc.Entries[0]
	if e.ID != "real-id" || e.Title != "real-title" || e.Published != "real-published" || e.Updated != "real-updated" || len(e.Links) != 1 || e.Links[0].Href != "real" {
		t.Fatalf("%#v", e)
	}
	if _, ok := e.Resource.(MeterReading); !ok {
		t.Fatalf("%#v", e.Resource)
	}
}

func TestParseForeignResourceLookalikes(t *testing.T) {
	for _, namespace := range []string{"urn:foreign", AtomNamespace, ""} {
		t.Run(namespace, func(t *testing.T) {
			input := `<feed xmlns="` + AtomNamespace + `"><entry><content><ReadingType xmlns="` + namespace + `"><uom>72</uom></ReadingType></content></entry></feed>`
			doc, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if resource, ok := doc.Entries[0].Resource.(UnknownResource); !ok || resource.Name != "ReadingType" {
				t.Fatalf("%#v", doc)
			}
		})
	}
}

func TestParseForeignESPIFields(t *testing.T) {
	input := `<feed xmlns="` + AtomNamespace + `" xmlns:e="` + ESPINamespace + `" xmlns:f="urn:foreign"><entry><content><e:UsagePoint><e:ServiceCategory><e:kind>1</e:kind><f:kind>99</f:kind></e:ServiceCategory><f:ServiceCategory><e:kind>99</e:kind></f:ServiceCategory><e:status>1</e:status><f:status>99</f:status></e:UsagePoint></content></entry><entry><content><e:ReadingType><e:uom>72</e:uom><f:uom>99</f:uom><e:powerOfTenMultiplier>-3</e:powerOfTenMultiplier><f:powerOfTenMultiplier>99</f:powerOfTenMultiplier></e:ReadingType></content></entry><entry><content><e:IntervalBlock><e:IntervalReading><e:timePeriod><e:start>123</e:start><f:start>99</f:start><e:duration>900</e:duration></e:timePeriod><f:timePeriod><e:start>99</e:start></f:timePeriod><e:value>12</e:value><f:value>99</f:value><e:ReadingQuality><e:quality>17</e:quality><f:quality>99</f:quality></e:ReadingQuality><f:ReadingQuality><e:quality>99</e:quality></f:ReadingQuality></e:IntervalReading><f:IntervalReading><e:value>99</e:value></f:IntervalReading></e:IntervalBlock></content></entry></feed>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	up := doc.Entries[0].Resource.(UsagePoint)
	if up.ServiceCategory != 1 || up.Status != 1 {
		t.Fatalf("%#v", up)
	}
	rt := doc.Entries[1].Resource.(ReadingType)
	if rt.UOM != 72 || rt.PowerOfTenMultiplier != -3 {
		t.Fatalf("%#v", rt)
	}
	block := doc.Entries[2].Resource.(IntervalBlock)
	if len(block.Readings) != 1 {
		t.Fatalf("%#v", block)
	}
	r := block.Readings[0]
	if r.Value != 12 || r.TimePeriod.Start != 123 || r.TimePeriod.Duration != 900 || len(r.Qualities) != 1 || r.Qualities[0] != 17 {
		t.Fatalf("%#v", r)
	}
}

func TestParseReboundAndUndeclaredPrefixes(t *testing.T) {
	for _, resource := range []string{
		`<espi:ReadingType xmlns:espi="urn:foreign"><espi:uom>72</espi:uom></espi:ReadingType>`,
		`<missing:ReadingType><missing:uom>72</missing:uom></missing:ReadingType>`,
	} {
		input := `<feed xmlns="` + AtomNamespace + `" xmlns:espi="` + ESPINamespace + `"><entry><content>` + resource + `</content></entry></feed>`
		doc, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := doc.Entries[0].Resource.(UnknownResource); !ok {
			t.Fatalf("%#v", doc)
		}
	}
}

func TestParseForeignLinkAttributes(t *testing.T) {
	for _, attributes := range []string{
		`rel="self" href="real" type="real-type" f:rel="up" f:href="fake" f:type="fake-type"`,
		`f:rel="up" f:href="fake" f:type="fake-type" rel="self" href="real" type="real-type"`,
	} {
		input := `<feed xmlns="` + AtomNamespace + `" xmlns:f="urn:foreign"><entry><link ` + attributes + `/></entry></feed>`
		doc, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.Entries[0].Links; len(got) != 1 || got[0] != (Link{Rel: "self", Href: "real", Type: "real-type"}) {
			t.Fatalf("%#v", got)
		}
	}
	input := `<feed xmlns="` + AtomNamespace + `" xmlns:f="urn:foreign"><entry><link f:rel="self" f:href="fake" f:type="fake-type"/></entry></feed>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Entries[0].Links; len(got) != 1 || got[0] != (Link{}) {
		t.Fatalf("%#v", got)
	}
}

func TestParseNamespaceBindingRestoredBetweenSiblings(t *testing.T) {
	input := `<feed xmlns="` + AtomNamespace + `" xmlns:e="` + ESPINamespace + `"><entry><content><e:ReadingType xmlns:e="urn:foreign"><e:uom>99</e:uom></e:ReadingType></content></entry><entry><content><e:ReadingType><e:uom xmlns:e="urn:foreign">99</e:uom><e:uom>72</e:uom><e:commodity xmlns:e="urn:foreign">99</e:commodity><e:commodity>1</e:commodity></e:ReadingType></content></entry><entry><content><e:MeterReading/></content></entry></feed>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 3 {
		t.Fatalf("%#v", doc)
	}
	if _, ok := doc.Entries[0].Resource.(UnknownResource); !ok {
		t.Fatalf("%#v", doc.Entries[0])
	}
	if rt, ok := doc.Entries[1].Resource.(ReadingType); !ok || rt.UOM != 72 || rt.Commodity != 1 {
		t.Fatalf("%#v", doc.Entries[1])
	}
	if _, ok := doc.Entries[2].Resource.(MeterReading); !ok {
		t.Fatalf("%#v", doc.Entries[2])
	}
}
