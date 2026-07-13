package espi

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MaxDocumentBytes int64 = 100 << 20
const MaxEntries = 100000
const MaxReadings = 5000000

const opowerFeedTitle = "Opower ESPI Third Party Batch Feed v1"

type feedXML struct {
	XMLName xml.Name
	Title   string     `xml:"title"`
	Updated string     `xml:"updated"`
	Entries []entryXML `xml:"entry"`
}
type entryXML struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Links     []Link `xml:"link"`
	Content   struct {
		Inner []byte `xml:",innerxml"`
	} `xml:"content"`
}

// Parse reads one bounded Atom XML feed and decodes supported ESPI resources.
func Parse(r io.Reader) (Document, error) {
	limited := io.LimitReader(r, MaxDocumentBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Document{}, err
	}
	if int64(len(data)) > MaxDocumentBytes {
		return Document{}, errors.New("ESPI document exceeds 100 MiB limit")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	var feed feedXML
	if err = decoder.Decode(&feed); err != nil {
		return Document{}, fmt.Errorf("parse ESPI XML: %w", err)
	}
	if feed.XMLName.Local != "feed" || feed.XMLName.Space != AtomNamespace {
		return Document{}, errors.New("ESPI document root must be an Atom feed")
	}
	if len(feed.Entries) > MaxEntries {
		return Document{}, errors.New("ESPI document has too many entries")
	}
	doc := Document{Title: feed.Title, Updated: feed.Updated, Entries: make([]Entry, 0, len(feed.Entries))}
	readings := 0
	for _, raw := range feed.Entries {
		resource, err := decodeResource(raw.Content.Inner)
		if err != nil {
			return Document{}, fmt.Errorf("entry %q: %w", raw.ID, err)
		}
		if block, ok := resource.(IntervalBlock); ok {
			readings += len(block.Readings)
			if readings > MaxReadings {
				return Document{}, errors.New("ESPI document has too many interval readings")
			}
		}
		doc.Entries = append(doc.Entries, Entry{raw.ID, raw.Title, raw.Published, raw.Updated, raw.Links, resource})
	}
	return doc, nil
}
func decodeResource(inner []byte) (Resource, error) {
	decoder := xml.NewDecoder(bytes.NewReader(inner))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return UnknownResource{Name: "empty"}, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		var resource Resource
		switch start.Name.Local {
		case "UsagePoint":
			var value UsagePoint
			err = decoder.DecodeElement(&value, &start)
			resource = value
		case "LocalTimeParameters":
			var value LocalTimeParameters
			err = decoder.DecodeElement(&value, &start)
			resource = value
		case "MeterReading":
			var value MeterReading
			err = decoder.DecodeElement(&value, &start)
			resource = value
		case "ReadingType":
			var value ReadingType
			err = decoder.DecodeElement(&value, &start)
			resource = value
		case "IntervalBlock":
			var value IntervalBlock
			err = decoder.DecodeElement(&value, &start)
			resource = value
		default:
			var discard struct{}
			err = decoder.DecodeElement(&discard, &start)
			resource = UnknownResource{Name: start.Name.Local}
		}
		if err != nil {
			return nil, err
		}
		for {
			trailing, trailingErr := decoder.Token()
			if trailingErr == io.EOF {
				return resource, nil
			}
			if trailingErr != nil {
				return nil, trailingErr
			}
			if _, ok := trailing.(xml.StartElement); ok {
				return nil, errors.New("entry content contains multiple resources")
			}
		}
	}
}

// Issue describes a structural validation error or warning.
type Issue struct {
	Severity string `json:"severity"`
	Entry    string `json:"entry,omitempty"`
	Message  string `json:"message"`
}

// Validate returns structural and normalization issues in document order.
func Validate(doc Document) []Issue {
	issues := make([]Issue, 0)
	seen := map[string]bool{}
	bySelf := map[string]Entry{}
	selfOwners := map[string]string{}
	for _, entry := range doc.Entries {
		for _, link := range entry.Links {
			if link.Rel != "self" || strings.TrimSpace(link.Href) == "" {
				continue
			}
			href := cleanHref(link.Href)
			if owner, exists := selfOwners[href]; exists && owner != entry.ID {
				issues = append(issues, Issue{"error", entry.ID, "duplicate self link"})
				continue
			}
			selfOwners[href] = entry.ID
			bySelf[href] = entry
		}
	}
	intervals := map[string]string{}
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.ID) == "" {
			issues = append(issues, Issue{"error", "", "entry has no id"})
		} else if seen[e.ID] {
			issues = append(issues, Issue{"error", e.ID, "duplicate entry id"})
		}
		seen[e.ID] = true
		if e.Resource == nil {
			issues = append(issues, Issue{"error", e.ID, "entry has no resource"})
		}
		if unknown, ok := e.Resource.(UnknownResource); ok && unknown.Name == "empty" {
			issues = append(issues, Issue{"error", e.ID, "entry content has no resource"})
		}
		if readingType, ok := e.Resource.(ReadingType); ok && !validMultiplier(readingType.PowerOfTenMultiplier) {
			issues = append(issues, Issue{"error", e.ID, "powerOfTenMultiplier is outside the supported ESPI range"})
		}
		if block, ok := e.Resource.(IntervalBlock); ok {
			meterHref := parentResource(e, "MeterReading")
			meter, found := bySelf[meterHref]
			if !found {
				issues = append(issues, Issue{"warning", e.ID, "interval block has no matching meter reading"})
			} else if _, found = bySelf[relatedResource(meter, "ReadingType")]; !found {
				issues = append(issues, Issue{"warning", e.ID, "meter reading has no matching reading type"})
			}
			for _, r := range block.Readings {
				if r.TimePeriod.Duration <= 0 {
					issues = append(issues, Issue{"error", e.ID, "interval reading has non-positive duration"})
				} else if r.TimePeriod.Duration > math.MaxInt64/int64(time.Second) {
					issues = append(issues, Issue{"error", e.ID, "interval reading duration is too large"})
				}
				if !validUnixSecond(r.TimePeriod.Start) {
					issues = append(issues, Issue{"error", e.ID, "interval reading start is outside the supported time range"})
				}
				key := meterHref + "|" + strconv.FormatInt(r.TimePeriod.Start, 10) + "|" + strconv.FormatInt(r.TimePeriod.Duration, 10)
				if owner, exists := intervals[key]; exists {
					issues = append(issues, Issue{"error", e.ID, "duplicate interval reading (also in entry " + owner + ")"})
				} else {
					intervals[key] = e.ID
				}
			}
		}
	}
	return issues
}

// Normalize resolves ESPI relationships and returns chronological readings.
func Normalize(doc Document) ([]Record, error) {
	issues := Validate(doc)
	for _, i := range issues {
		if i.Severity == "error" {
			if i.Entry != "" {
				return nil, fmt.Errorf("validation failed for entry %q: %s", i.Entry, i.Message)
			}
			return nil, fmt.Errorf("validation failed: %s", i.Message)
		}
	}
	bySelf := map[string]Entry{}
	for _, e := range doc.Entries {
		for _, l := range e.Links {
			if l.Rel == "self" {
				bySelf[cleanHref(l.Href)] = e
			}
		}
	}
	var records []Record
	for _, e := range doc.Entries {
		block, ok := e.Resource.(IntervalBlock)
		if !ok {
			continue
		}
		meterHref := parentResource(e, "MeterReading")
		meter, found := bySelf[meterHref]
		if !found {
			return nil, fmt.Errorf("cannot normalize entry %q: matching MeterReading is required", e.ID)
		}
		readingTypeHref := relatedResource(meter, "ReadingType")
		readingTypeEntry, found := bySelf[readingTypeHref]
		if !found {
			return nil, fmt.Errorf("cannot normalize entry %q: matching ReadingType is required", e.ID)
		}
		rt, ok := readingTypeEntry.Resource.(ReadingType)
		if !ok {
			return nil, fmt.Errorf("cannot normalize entry %q: related resource is not a ReadingType", e.ID)
		}
		usageHref := parentResource(meter, "UsagePoint")
		for _, reading := range block.Readings {
			value, unit, profile, err := normalizedReading(doc.Title, rt, reading.Value)
			if err != nil {
				return nil, fmt.Errorf("cannot normalize entry %q: %w", e.ID, err)
			}
			records = append(records, Record{
				SchemaVersion: SchemaVersion, NormalizationProfile: profile,
				UsagePoint: lastID(usageHref), MeterReading: lastID(meterHref),
				Start: time.Unix(reading.TimePeriod.Start, 0).UTC(), DurationSeconds: reading.TimePeriod.Duration,
				RawValue: reading.Value, PowerOfTenMultiplier: rt.PowerOfTenMultiplier,
				Value: value, UOM: rt.UOM, Unit: unit,
				Commodity: rt.Commodity, FlowDirection: rt.FlowDirection,
				Quality: append([]int(nil), reading.Qualities...),
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Start.Equal(records[j].Start) {
			return records[i].MeterReading < records[j].MeterReading
		}
		return records[i].Start.Before(records[j].Start)
	})
	return records, nil
}
func normalizedReading(feedTitle string, readingType ReadingType, rawValue int64) (value, unit, profile string, err error) {
	if strings.TrimSpace(feedTitle) == opowerFeedTitle && readingType.UOM == 72 && readingType.PowerOfTenMultiplier == 3 {
		value, err = ScaleDecimal(rawValue, -3)
		return value, "kWh", NormalizationProfileOpowerMilliKWh, err
	}
	value, err = ScaleDecimal(rawValue, readingType.PowerOfTenMultiplier)
	return value, UnitName(readingType.UOM), "", err
}

func cleanHref(v string) string {
	u, err := url.Parse(v)
	if err == nil {
		u.RawQuery = ""
		u.Fragment = ""
		v = u.String()
	}
	return strings.TrimRight(v, "/")
}
func parentResource(e Entry, kind string) string {
	for _, l := range e.Links {
		if l.Rel == "up" && strings.Contains(l.Href, "/"+kind+"/") {
			h := cleanHref(l.Href)
			marker := "/" + kind + "/"
			i := strings.Index(h, marker)
			tail := h[i+len(marker):]
			id := strings.Split(tail, "/")[0]
			return h[:i] + marker + id
		}
	}
	return ""
}
func relatedResource(e Entry, kind string) string {
	for _, l := range e.Links {
		if l.Rel == "related" && strings.Contains(l.Href, "/"+kind+"/") {
			return cleanHref(l.Href)
		}
	}
	return ""
}
func lastID(h string) string {
	parts := strings.Split(strings.TrimRight(h, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// ScaleDecimal applies an ESPI power-of-ten multiplier without floating-point
// loss. Multipliers outside the supported ESPI SI-prefix range are rejected.
func ScaleDecimal(value int64, multiplier int) (string, error) {
	if !validMultiplier(multiplier) {
		return "", fmt.Errorf("power-of-ten multiplier %d is outside [%d, %d]", multiplier, MinPowerOfTenMultiplier, MaxPowerOfTenMultiplier)
	}
	digits := strconv.FormatInt(value, 10)
	negative := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")
	if multiplier >= 0 {
		digits += strings.Repeat("0", multiplier)
	} else {
		places := -multiplier
		if len(digits) <= places {
			digits = strings.Repeat("0", places-len(digits)+1) + digits
		}
		at := len(digits) - places
		digits = digits[:at] + "." + digits[at:]
		digits = strings.TrimRight(strings.TrimRight(digits, "0"), ".")
	}
	if negative {
		return "-" + digits, nil
	}
	return digits, nil
}

func validMultiplier(multiplier int) bool {
	return multiplier >= MinPowerOfTenMultiplier && multiplier <= MaxPowerOfTenMultiplier
}
func validUnixSecond(second int64) bool {
	year := time.Unix(second, 0).UTC().Year()
	return year >= 1 && year <= 9999
}

// UnitName returns a readable name for the supported ESPI UOM code.
func UnitName(uom int) string {
	switch uom {
	case 38:
		return "W"
	case 72:
		return "Wh"
	case 119:
		return "ft3"
	case 169:
		return "therm"
	default:
		return fmt.Sprintf("uom:%d", uom)
	}
}
