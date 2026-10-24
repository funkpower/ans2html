package main

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// vga is the 16-colour text-mode palette, in ANSI order.
var vga = [16]uint32{
	0x000000, 0xaa0000, 0x00aa00, 0xaa5500, 0x0000aa, 0xaa00aa, 0x00aaaa, 0xaaaaaa,
	0x555555, 0xff5555, 0x55ff55, 0xffff55, 0x5555ff, 0xff55ff, 0x55ffff, 0xffffff,
}

func (c colour) hex() string {
	switch {
	case c&trueColor != 0:
		return fmt.Sprintf("#%06x", uint32(c&^trueColor))
	case c < 16:
		return fmt.Sprintf("#%06x", vga[c])
	case c < 232:
		level := [6]uint32{0, 95, 135, 175, 215, 255}
		n := uint32(c - 16)
		return fmt.Sprintf("#%02x%02x%02x", level[n/36], level[n/6%6], level[n%6])
	default:
		g := 8 + 10*uint32(c-232)
		return fmt.Sprintf("#%02x%02x%02x", g, g, g)
	}
}

// Half and full blocks are drawn with backgrounds rather than font glyphs,
// so they meet edge to edge whatever font the browser picks.
type kind uint8

const (
	glyph kind = iota
	upperHalf
	lowerHalf
	leftHalf
	rightHalf
)

var blockKinds = map[rune]kind{'▀': upperHalf, '▄': lowerHalf, '▌': leftHalf, '▐': rightHalf}
var kindClass = [...]string{"", "hu", "hl", "vl", "vr"}

type run struct {
	fg, bg   colour
	blink    bool
	kind     kind
	text     strings.Builder
	anyGlyph bool // false while the run holds only spaces, whose foreground does not show
}

// shape turns a cell into what is actually drawn.
func shape(c cell) (rune, kind, colour, colour) {
	if c.r == '█' {
		return ' ', glyph, c.fg, c.fg
	}
	if k, ok := blockKinds[c.r]; ok {
		return ' ', k, c.fg, c.bg
	}
	if c.r == ' ' {
		c.r = ' '
	}
	return c.r, glyph, c.fg, c.bg
}

func (r *run) accepts(ch rune, k kind, fg, bg colour, blink bool) bool {
	if k != r.kind || bg != r.bg || blink != r.blink {
		return false
	}
	return fg == r.fg || (k == glyph && (ch == ' ' || !r.anyGlyph))
}

func (r *run) add(ch rune, fg colour) {
	if ch != ' ' || r.kind != glyph {
		if !r.anyGlyph {
			r.fg = fg
		}
		r.anyGlyph = true
	}
	r.text.WriteRune(ch)
}

func (r *run) write(w *strings.Builder) {
	text := html.EscapeString(r.text.String())
	if r.kind == glyph && r.bg == defaultBG && !r.blink && (r.fg == defaultFG || !r.anyGlyph) {
		w.WriteString(text)
		return
	}
	var class, style []string
	if r.anyGlyph && r.fg != defaultFG {
		if r.fg < 16 {
			class = append(class, fmt.Sprintf("f%d", r.fg))
		} else {
			style = append(style, "color:"+r.fg.hex(), "--f:"+r.fg.hex())
		}
	}
	if r.bg != defaultBG {
		if r.bg < 16 {
			class = append(class, fmt.Sprintf("b%d", r.bg))
		} else {
			style = append(style, "background-color:"+r.bg.hex())
		}
	}
	if r.kind != glyph {
		class = append(class, kindClass[r.kind])
	}
	if r.blink {
		class = append(class, "bl")
	}
	w.WriteString("<span")
	if len(class) > 0 {
		fmt.Fprintf(w, ` class="%s"`, strings.Join(class, " "))
	}
	if len(style) > 0 {
		fmt.Fprintf(w, ` style="%s"`, strings.Join(style, ";"))
	}
	fmt.Fprintf(w, ">%s</span>", text)
}

// trailing reports whether a cell is indistinguishable from the empty screen.
func trailing(c cell) bool {
	ch, k, _, bg := shape(c)
	return ch == ' ' && k == glyph && bg == defaultBG && !c.blink
}

func (s *screen) body() string {
	rows := s.rows
	for len(rows) > 0 && blankRow(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}
	var w strings.Builder
	for y, row := range rows {
		if y > 0 {
			w.WriteByte('\n')
		}
		end := len(row)
		for end > 0 && trailing(row[end-1]) {
			end--
		}
		var cur *run
		for _, c := range row[:end] {
			ch, k, fg, bg := shape(c)
			if cur == nil || !cur.accepts(ch, k, fg, bg, c.blink) {
				if cur != nil {
					cur.write(&w)
				}
				cur = &run{fg: fg, bg: bg, blink: c.blink, kind: k}
			}
			cur.add(ch, fg)
		}
		if cur != nil {
			cur.write(&w)
		}
	}
	return w.String()
}

func blankRow(row []cell) bool {
	for _, c := range row {
		if !trailing(c) {
			return false
		}
	}
	return true
}

const css = `:root{color-scheme:dark light}
html,body{margin:0;padding:0;background:transparent}
.wrap{overflow-x:auto}
.ans{display:block;margin:0;padding:0;background:#000;color:#aaa;--f:#aaa;
font:16px/16px "DejaVu Sans Mono","Cascadia Mono","Liberation Mono",Menlo,Consolas,monospace;
white-space:pre;font-variant-ligatures:none;font-kerning:none}
.ans span{display:inline-block;height:16px;vertical-align:top}
.hu{background-image:linear-gradient(var(--f) 50%,transparent 50%)}
.hl{background-image:linear-gradient(transparent 50%,var(--f) 50%)}
.vl{background-image:linear-gradient(90deg,var(--f) 50%,transparent 50%);background-size:1ch 100%}
.vr{background-image:linear-gradient(90deg,transparent 50%,var(--f) 50%);background-size:1ch 100%}
.bl{animation:bl 1.07s step-end infinite}
@keyframes bl{50%{color:transparent}}
@media (prefers-reduced-motion:reduce){.bl{animation:none}}
.sauce{margin:6px 0 0;font:12px/1.4 system-ui,sans-serif;color:#888}
`

func palette() string {
	var w strings.Builder
	for i, c := range vga {
		fmt.Fprintf(&w, ".f%d{color:#%06x;--f:#%06x}.b%d{background-color:#%06x}\n", i, c, c, i, c)
	}
	return w.String()
}

func writeDocument(out io.Writer, s *screen, rec *sauce) error {
	title := "ANSI"
	var caption string
	if rec != nil {
		if rec.Title != "" {
			title = rec.Title
		}
		parts := []string{}
		if rec.Title != "" {
			parts = append(parts, rec.Title)
		}
		if rec.Author != "" {
			parts = append(parts, "by "+rec.Author)
		}
		if rec.Group != "" {
			parts = append(parts, "/ "+rec.Group)
		}
		caption = strings.Join(parts, " ")
	}
	var w strings.Builder
	w.WriteString("<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\">\n")
	fmt.Fprintf(&w, "<title>%s</title>\n<style>\n%s%s</style></head>\n<body>", html.EscapeString(title), css, palette())
	fmt.Fprintf(&w, "<div class=\"wrap\"><pre class=\"ans\" style=\"width:%dch\">%s</pre></div>", s.width, s.body())
	if caption != "" {
		fmt.Fprintf(&w, "\n<p class=\"sauce\">%s</p>", html.EscapeString(caption))
	}
	w.WriteString("\n</body></html>\n")
	_, err := io.WriteString(out, w.String())
	return err
}
