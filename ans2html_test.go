package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"
)

func withSauce(art []byte, title, author string, width uint16, flags byte, comments ...string) []byte {
	rec := make([]byte, sauceLen)
	copy(rec, "SAUCE00")
	pad := func(b []byte, s string) {
		for i := range b {
			b[i] = ' '
		}
		copy(b, s)
	}
	pad(rec[7:42], title)
	pad(rec[42:62], author)
	pad(rec[62:82], "")
	rec[94], rec[95] = 1, 1
	binary.LittleEndian.PutUint16(rec[96:98], width)
	rec[104] = byte(len(comments))
	rec[105] = flags
	out := append(append([]byte{}, art...), 0x1a)
	if len(comments) > 0 {
		out = append(out, "COMNT"...)
		for _, c := range comments {
			line := make([]byte, 64)
			pad(line, c)
			out = append(out, line...)
		}
	}
	return append(out, rec...)
}

func draw(t *testing.T, data []byte) *screen {
	t.Helper()
	art, rec := splitSauce(data)
	width, ice := 0, true
	if rec != nil {
		width, ice = rec.Width, rec.ICE
	}
	s := newScreen(width, ice)
	s.run(art, false)
	return s
}

func line(s *screen, y int) string {
	var b strings.Builder
	for _, c := range s.rows[y] {
		b.WriteRune(c.r)
	}
	return strings.TrimRight(b.String(), " ")
}

func TestCP437(t *testing.T) {
	for b, want := range map[byte]rune{0x01: '☺', 'A': 'A', 0x7f: '⌂', 0xb0: '░', 0xc4: '─', 0xdb: '█', 0xdf: '▀', 0xfe: '■'} {
		if cp437[b] != want {
			t.Errorf("cp437[%#x] = %q, want %q", b, cp437[b], want)
		}
	}
}

func TestSauce(t *testing.T) {
	data := withSauce([]byte("hi"), "Title", "someone", 40, 1, "a comment")
	art, rec := splitSauce(data)
	if string(art) != "hi" {
		t.Fatalf("art = %q", art)
	}
	if rec == nil || rec.Title != "Title" || rec.Author != "someone" || rec.Width != 40 || !rec.ICE {
		t.Fatalf("rec = %+v", rec)
	}
}

func TestNoSauceStopsAtEOF(t *testing.T) {
	art, rec := splitSauce([]byte("drawn\x1ahidden"))
	if string(art) != "drawn" || rec != nil {
		t.Fatalf("art = %q, rec = %v", art, rec)
	}
}

func TestBoldBrightensAndICE(t *testing.T) {
	s := draw(t, []byte("\x1b[1;31;5;44mX"))
	if c := s.rows[0][0]; c.fg != 9 || c.bg != 12 || c.blink {
		t.Fatalf("iCE cell = %+v", c)
	}
	s = draw(t, withSauce([]byte("\x1b[5;44mX"), "", "", 80, 0))
	if c := s.rows[0][0]; c.bg != 4 || !c.blink {
		t.Fatalf("blinking cell = %+v", c)
	}
}

// A full-width line followed by CRLF must not leave an empty line behind it,
// and one without CRLF must still wrap.
func TestDeferredWrap(t *testing.T) {
	s := draw(t, withSauce([]byte("abcd\r\nefgh"), "", "", 4, 1))
	if len(s.rows) != 2 || line(s, 0) != "abcd" || line(s, 1) != "efgh" {
		t.Fatalf("with CRLF: %d rows", len(s.rows))
	}
	s = draw(t, withSauce([]byte("abcdefgh"), "", "", 4, 1))
	if len(s.rows) != 2 || line(s, 1) != "efgh" {
		t.Fatalf("without CRLF: %d rows", len(s.rows))
	}
}

func TestCursor(t *testing.T) {
	s := draw(t, []byte("a\x1b[3Cb\x1b[2;2Hc\x1b[s\x1b[1;1Hd\x1b[ue"))
	if line(s, 0) != "d   b" || line(s, 1) != " ce" {
		t.Fatalf("got %q / %q", line(s, 0), line(s, 1))
	}
}

func TestExtendedColour(t *testing.T) {
	s := draw(t, []byte("\x1b[38;5;196;48;2;1;2;3mX"))
	c := s.rows[0][0]
	if c.fg != 196 || c.bg != rgb(1, 2, 3) {
		t.Fatalf("cell = %+v", c)
	}
	if c.fg.hex() != "#ff0000" || c.bg.hex() != "#010203" {
		t.Fatalf("hex = %s %s", c.fg.hex(), c.bg.hex())
	}
}

func TestHTML(t *testing.T) {
	var out bytes.Buffer
	data := withSauce([]byte("\x1b[1;33m<&>\x1b[0m \x1b[34m\xdb\xdf"), "T<", "", 80, 1)
	if err := render(&out, data, 0, "auto"); err != nil {
		t.Fatal(err)
	}
	h := out.String()
	for _, want := range []string{
		`<span class="f11">&lt;&amp;&gt; </span>`,
		`<span class="b4"> </span>`,    // full block drawn as background
		`<span class="f4 hu"> </span>`, // upper half drawn with a gradient
		`<title>T&lt;</title>`,
		`style="width:80ch"`,
	} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %s in\n%s", want, h[strings.Index(h, "<body>"):])
		}
	}
}

func TestUTF8Input(t *testing.T) {
	var out bytes.Buffer
	if err := render(&out, []byte("\x1b[32m░▒▓\x1b[0m"), 0, "auto"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `<span class="f2">░▒▓</span>`) {
		t.Fatalf("UTF-8 shades not passed through:\n%s", out.String())
	}
}

func TestRunsIgnoreForegroundOfSpaces(t *testing.T) {
	s := draw(t, []byte("\x1b[44m\x1b[31m \x1b[32m x"))
	got := s.body()
	if got != `<span class="f2 b4">  x</span>` {
		t.Fatalf("body = %q", got)
	}
}

func TestTestCard(t *testing.T) {
	data, err := os.ReadFile("testdata/testcard.ans")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := render(&out, data, 0, "auto"); err != nil {
		t.Fatal(err)
	}
	h := out.String()
	for _, want := range []string{"<title>ans2html test card</title>", "by funkpower", "╔══", "░░░░"} {
		if !strings.Contains(h, want) {
			t.Errorf("test card output is missing %q", want)
		}
	}
}
