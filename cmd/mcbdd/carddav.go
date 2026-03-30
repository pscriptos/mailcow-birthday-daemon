package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

// vlistFilterClient wraps a webdav.HTTPClient and removes DAV <response>
// elements whose address-data contains a BEGIN:VLIST block before the
// carddav/vcard decoder processes them.
type vlistFilterClient struct {
	inner webdav.HTTPClient
}

func (c *vlistFilterClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.Do(req)
	if err != nil {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	body, err = stripVListResponses(body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

// stripVListResponses scans a WebDAV multistatus XML body and removes any
// <D:response> elements whose <C:address-data> chardata contains BEGIN:VLIST.
// It uses the XML decoder only to find byte offsets; it never re-encodes, so
// namespace declarations and formatting are preserved exactly.
// Non-multistatus bodies (no VLIST present) are returned unchanged.
func stripVListResponses(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte("BEGIN:VLIST")) {
		return body, nil
	}

	const davNS = "DAV:"
	const cardNS = "urn:ietf:params:xml:ns:carddav"

	dec := xml.NewDecoder(bytes.NewReader(body))

	// ranges holds [start, end) byte offsets of <D:response> blocks to drop.
	type byteRange struct{ start, end int64 }
	var drop []byteRange

	var responseStart int64
	responseDepth := 0
	inAddrData := false
	addrDataDepth := 0
	isVList := false

	for {
		offset := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return body, nil // unparseable — return original unchanged
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == davNS && t.Name.Local == "response" && responseDepth == 0 {
				responseStart = offset
				responseDepth = 1
				isVList = false
			} else if responseDepth > 0 {
				// Increment depth before checking the element name so that
				// addrDataDepth records the post-increment value, matching the
				// depth at which the corresponding EndElement will fire.
				responseDepth++
				if t.Name.Space == cardNS && t.Name.Local == "address-data" {
					inAddrData = true
					addrDataDepth = responseDepth
				}
			}
		case xml.EndElement:
			if responseDepth > 0 {
				if inAddrData && responseDepth == addrDataDepth {
					inAddrData = false
				}
				responseDepth--
				if responseDepth == 0 {
					if isVList {
						drop = append(drop, byteRange{responseStart, dec.InputOffset()})
					}
				}
			}
		case xml.CharData:
			if inAddrData && bytes.Contains(t, []byte("BEGIN:VLIST")) {
				isVList = true
			}
		}
	}

	if len(drop) == 0 {
		return body, nil
	}

	var out bytes.Buffer
	out.Grow(len(body))
	pos := int64(0)
	for _, r := range drop {
		out.Write(body[pos:r.start])
		pos = r.end
	}
	out.Write(body[pos:])
	return out.Bytes(), nil
}

// ContactType unterscheidet zwischen Geburtstagen und Jahrestagen.
type ContactType int

const (
	ContactTypeBirthday    ContactType = iota
	ContactTypeAnniversary
)

type BirthdayContact struct {
	FamilyName string
	GivenName  string
	Date       time.Time
	YearKnown  bool
	Type       ContactType
}

func (d *Daemon) getBirthdays(ctx context.Context, httpClient webdav.HTTPClient, user string) ([]BirthdayContact, error) {
	endpoint, err := url.JoinPath(d.baseURL, "SOGo/dav", user, "Contacts/")
	if err != nil {
		return nil, err
	}
	cl, err := carddav.NewClient(&vlistFilterClient{inner: httpClient}, endpoint)
	if err != nil {
		return nil, err
	}
	bb, err := cl.FindAddressBooks(ctx, "")
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "found address books", "user", user, "count", len(bb))
	contacts := make([]BirthdayContact, 0)
	for _, b := range bb {
		slog.DebugContext(ctx, "querying address book", "user", user, "path", b.Path)
		oo, err := cl.QueryAddressBook(ctx, b.Path, &carddav.AddressBookQuery{})
		if err != nil {
			if err.Error() == "501 Not Implemented" {
				continue
			}
			return nil, err
		}
		for _, v := range oo {
			nn := v.Card.Names()
			if len(nn) == 0 {
				continue
			}
			bdayprop := v.Card.Value(vcard.FieldBirthday)
			if len(bdayprop) > 0 {
				yyyy, mm, dd, err := sanitizeDate(bdayprop)
				if err != nil {
					return nil, err
				}
				contacts = append(contacts, BirthdayContact{
					GivenName:  nn[0].GivenName,
					FamilyName: nn[0].FamilyName,
					Date:       time.Date(int(yyyy), time.Month(int(mm)), int(dd), 0, 0, 0, 0, time.UTC),
					YearKnown:  yyyy != 0,
					Type:       ContactTypeBirthday,
				})
				slog.DebugContext(ctx, "found birthday contact", "user", user, "name", nn[0].GivenName+" "+nn[0].FamilyName, "date", bdayprop)
			}
			{
				annivprop := v.Card.Value("ANNIVERSARY")
				if len(annivprop) > 0 {
					yyyy, mm, dd, err := sanitizeDate(annivprop)
					if err != nil {
						return nil, err
					}
					contacts = append(contacts, BirthdayContact{
						GivenName:  nn[0].GivenName,
						FamilyName: nn[0].FamilyName,
						Date:       time.Date(int(yyyy), time.Month(int(mm)), int(dd), 0, 0, 0, 0, time.UTC),
						YearKnown:  yyyy != 0,
						Type:       ContactTypeAnniversary,
					})
					slog.DebugContext(ctx, "found anniversary contact", "user", user, "name", nn[0].GivenName+" "+nn[0].FamilyName, "date", annivprop)
				}
			}
		}
	}
	slog.DebugContext(ctx, "collected contacts with dates", "user", user, "count", len(contacts))
	return contacts, nil
}
