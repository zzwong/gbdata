// Package espi parses bounded Green Button Atom feeds and normalizes interval
// usage resources into exact canonical records.
//
// Parse performs XML and resource-count checks. Validate reports structural
// issues, and Normalize requires resolvable MeterReading and ReadingType links.
// The package is intentionally not a complete ESPI 4.0 or XSD validator.
package espi
