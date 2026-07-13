package espi

import "time"

// SchemaVersion identifies the canonical Record and CLI output contract.
const SchemaVersion = 3

const (
	// AtomNamespace is the required feed namespace.
	AtomNamespace = "http://www.w3.org/2005/Atom"
	// ESPINamespace is the NAESB ESPI XML namespace.
	ESPINamespace = "http://naesb.org/espi"
)

const (
	// MinPowerOfTenMultiplier and MaxPowerOfTenMultiplier bound the SI-prefix
	// range accepted from ESPI ReadingType resources.
	MinPowerOfTenMultiplier = -12
	MaxPowerOfTenMultiplier = 12
)

// NormalizationProfileOpowerMilliKWh identifies Opower readings whose raw
// integer is encoded in milli-kWh despite ESPI metadata naming kWh.
const NormalizationProfileOpowerMilliKWh = "opower-millikwh-v1"

// Document is a parsed Atom feed containing ESPI resources.
type Document struct {
	Title   string
	Updated string
	Entries []Entry
}

// Link is an Atom link associated with an entry.
type Link struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

// Entry is an Atom entry and its decoded ESPI resource.
type Entry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Links     []Link `xml:"link"`
	Resource  Resource
}

// Resource is an ESPI resource decoded from an Atom entry.
type Resource interface{ ResourceType() string }

// UsagePoint identifies the service category and state of a usage point.
type UsagePoint struct {
	ServiceCategory int `xml:"ServiceCategory>kind"`
	RoleFlags       int `xml:"roleFlags"`
	Status          int `xml:"status"`
}

func (UsagePoint) ResourceType() string { return "UsagePoint" }

// LocalTimeParameters describes the provider's local time and DST rules.
type LocalTimeParameters struct {
	DSTEndRule   string `xml:"dstEndRule"`
	DSTOffset    int64  `xml:"dstOffset"`
	DSTStartRule string `xml:"dstStartRule"`
	TZOffset     int64  `xml:"tzOffset"`
}

func (LocalTimeParameters) ResourceType() string { return "LocalTimeParameters" }

// MeterReading is a relationship node connecting readings to a ReadingType.
type MeterReading struct{}

func (MeterReading) ResourceType() string { return "MeterReading" }

// ReadingType describes units and measurement semantics for interval readings.
type ReadingType struct {
	AccumulationBehaviour int   `xml:"accumulationBehaviour"`
	Commodity             int   `xml:"commodity"`
	FlowDirection         int   `xml:"flowDirection"`
	IntervalLength        int64 `xml:"intervalLength"`
	Kind                  int   `xml:"kind"`
	PowerOfTenMultiplier  int   `xml:"powerOfTenMultiplier"`
	UOM                   int   `xml:"uom"`
}

func (ReadingType) ResourceType() string { return "ReadingType" }

// DateTimeInterval is an ESPI Unix start and duration in seconds.
type DateTimeInterval struct {
	Duration int64 `xml:"duration"`
	Start    int64 `xml:"start"`
}

// IntervalReading contains one raw ESPI reading and its time period.
type IntervalReading struct {
	Cost       int64            `xml:"cost"`
	TimePeriod DateTimeInterval `xml:"timePeriod"`
	Value      int64            `xml:"value"`
	Qualities  []int            `xml:"ReadingQuality>quality"`
}

// IntervalBlock groups interval readings.
type IntervalBlock struct {
	Interval DateTimeInterval  `xml:"interval"`
	Readings []IntervalReading `xml:"IntervalReading"`
}

func (IntervalBlock) ResourceType() string { return "IntervalBlock" }

// UnknownResource preserves the local name of an unsupported ESPI resource.
type UnknownResource struct{ Name string }

func (u UnknownResource) ResourceType() string { return u.Name }

// Record is a canonical interval reading. Value is an exact decimal string.
type Record struct {
	SchemaVersion        int       `json:"schema_version"`
	NormalizationProfile string    `json:"normalization_profile,omitempty"`
	UsagePoint           string    `json:"usage_point,omitempty"`
	MeterReading         string    `json:"meter_reading,omitempty"`
	Start                time.Time `json:"start"`
	DurationSeconds      int64     `json:"duration_seconds"`
	RawValue             int64     `json:"raw_value"`
	PowerOfTenMultiplier int       `json:"power_of_ten_multiplier"`
	Value                string    `json:"value"`
	UOM                  int       `json:"uom"`
	Unit                 string    `json:"unit"`
	Commodity            int       `json:"commodity"`
	FlowDirection        int       `json:"flow_direction"`
	Quality              []int     `json:"quality,omitempty"`
}
