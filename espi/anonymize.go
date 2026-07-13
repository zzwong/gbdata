package espi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const anonymizedEpoch int64 = 1577836800 // 2020-01-01T00:00:00Z
const secondsPerLeapYear uint64 = 366 * 24 * 60 * 60

// Anonymize writes a structurally similar XML document with customer content
// replaced. Pseudonyms are consistent only within one invocation.
func Anonymize(in io.Reader, out io.Writer) error {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("create anonymization salt: %w", err)
	}

	limited := &io.LimitedReader{R: in, N: MaxDocumentBytes + 1}
	decoder := xml.NewDecoder(limited)
	decoder.Strict = true
	encoder := xml.NewEncoder(out)
	state := anonymizer{salt: salt}

	for {
		token, err := decoder.Token()
		if limited.N < 0 || limited.N == 0 {
			return errors.New("ESPI document exceeds 100 MiB limit")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("parse XML for anonymization: %w", err)
		}

		token, keep := state.transform(token)
		if !keep {
			continue
		}
		if err := encoder.EncodeToken(token); err != nil {
			return fmt.Errorf("write anonymized XML: %w", err)
		}
	}
	if err := encoder.Flush(); err != nil {
		return fmt.Errorf("finish anonymized XML: %w", err)
	}
	return nil
}

type anonymizer struct {
	salt     []byte
	elements []string
}

func (a *anonymizer) transform(token xml.Token) (xml.Token, bool) {
	switch value := token.(type) {
	case xml.StartElement:
		a.elements = append(a.elements, value.Name.Local)
		attributes := value.Attr[:0]
		for _, attribute := range value.Attr {
			if attribute.Name.Space == "xmlns" || (attribute.Name.Space == "" && attribute.Name.Local == "xmlns") {
				continue
			}
			switch attribute.Name.Local {
			case "href":
				attribute.Value = a.anonymizeHref(attribute.Value)
			case "rel", "type":
			default:
				attribute.Value = "anon-" + a.pseudonym(attribute.Value)
			}
			attributes = append(attributes, attribute)
		}
		value.Attr = attributes
		return value, true
	case xml.EndElement:
		if len(a.elements) > 0 {
			a.elements = a.elements[:len(a.elements)-1]
		}
		return value, true
	case xml.CharData:
		return xml.CharData([]byte(a.anonymizeText(string(value)))), true
	case xml.ProcInst:
		if strings.EqualFold(value.Target, "xml") {
			return xml.ProcInst{Target: "xml", Inst: []byte(`version="1.0" encoding="UTF-8"`)}, true
		}
		return nil, false
	case xml.Comment, xml.Directive:
		return nil, false
	default:
		return token, true
	}
}

func (a *anonymizer) anonymizeText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	element := ""
	if len(a.elements) > 0 {
		element = a.elements[len(a.elements)-1]
	}
	switch element {
	case "id":
		return "urn:uuid:" + a.pseudonym(trimmed)
	case "title":
		return "Anonymized Green Button Data"
	case "published", "updated":
		return "2020-01-01T00:00:00Z"
	case "value", "cost":
		return "0"
	case "start":
		digest := a.digest(trimmed)
		return strconv.FormatInt(anonymizedEpoch+int64(binary.BigEndian.Uint64(digest[:8])%secondsPerLeapYear), 10)
	default:
		if safeMetadataValue(element, trimmed) {
			return trimmed
		}
		return "REDACTED"
	}
}

func (a *anonymizer) digest(value string) []byte {
	mac := hmac.New(sha256.New, a.salt)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func (a *anonymizer) pseudonym(value string) string {
	hash := hex.EncodeToString(a.digest(value)[:16])
	return hash[:8] + "-" + hash[8:12] + "-" + hash[12:16] + "-" + hash[16:20] + "-" + hash[20:32]
}

func (a *anonymizer) anonymizeHref(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "urn" || (parsed.Scheme != "" && parsed.Host == "") {
		return "urn:gbdata:" + a.pseudonym(value)
	}
	parts := strings.Split(parsed.Path, "/")
	for i, part := range parts {
		if part != "" && !isStructuralPathSegment(part) {
			parts[i] = a.pseudonym(part)
		}
	}
	parsed.Path = strings.Join(parts, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	if parsed.IsAbs() {
		parsed.Scheme = "https"
		parsed.Host = "example.invalid"
	}
	return parsed.String()
}

func safeMetadataValue(element, value string) bool {
	switch element {
	case "dstStartRule", "dstEndRule":
		if len(value) == 0 || len(value) > 32 {
			return false
		}
		for _, character := range value {
			if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
				return false
			}
		}
		return true
	case "accumulationBehaviour", "commodity", "flowDirection", "intervalLength", "kind", "powerOfTenMultiplier", "uom", "duration", "quality", "roleFlags", "status", "dstOffset", "tzOffset":
		_, err := strconv.ParseInt(value, 10, 64)
		return err == nil
	default:
		return false
	}
}

func isStructuralPathSegment(value string) bool {
	switch value {
	case "espi", "1_1", "resource", "Batch", "Subscription", "Bulk", "RetailCustomer", "UsagePoint", "MeterReading", "ReadingType", "IntervalBlock", "LocalTimeParameters", "UsageSummary", "ApplicationInformation", "Authorization":
		return true
	}
	return false
}
