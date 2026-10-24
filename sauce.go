package main

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// sauce is the part of a SAUCE record (https://www.acid.org/info/sauce/sauce.htm)
// that changes how a file is drawn or captioned.
type sauce struct {
	Title, Author, Group string
	Width                int  // TInfo1 for character files; 0 when not given
	ICE                  bool // flag bit 0: blink selects bright backgrounds
}

const sauceLen = 128

// splitSauce returns the drawable part of data and its SAUCE record, if any.
// The drawable part stops at the first EOF byte (0x1A), the way DOS viewers
// stop, so the record and any comment block never reach the screen.
func splitSauce(data []byte) ([]byte, *sauce) {
	var rec *sauce
	if n := len(data); n >= sauceLen && bytes.HasPrefix(data[n-sauceLen:], []byte("SAUCE00")) {
		r := data[n-sauceLen:]
		rec = &sauce{
			Title:  field(r[7:42]),
			Author: field(r[42:62]),
			Group:  field(r[62:82]),
			ICE:    r[105]&1 == 1,
		}
		// DataType 1 is Character; ASCII (0), ANSi (1) and ANSiMation (2)
		// keep their width in TInfo1.
		if r[94] == 1 && r[95] <= 2 {
			rec.Width = int(binary.LittleEndian.Uint16(r[96:98]))
		}
		data = data[:n-sauceLen]
		if comments := int(r[104]); comments > 0 {
			if c := len(data) - 5 - 64*comments; c >= 0 && bytes.HasPrefix(data[c:], []byte("COMNT")) {
				data = data[:c]
			}
		}
	}
	if i := bytes.IndexByte(data, 0x1a); i >= 0 {
		data = data[:i]
	}
	return data, rec
}

// field decodes a space- or NUL-padded SAUCE string, which is CP437 like the art.
func field(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		s.WriteRune(cp437[c])
	}
	return strings.TrimSpace(s.String())
}
