package main

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// A colour is a palette index (0-255) or, with trueColor set, 0xRRGGBB.
type colour uint32

const trueColor colour = 1 << 24

const (
	defaultFG colour = 7
	defaultBG colour = 0
)

// Limits on how far cursor movement can grow the screen, so a stray
// ESC[9999B costs nothing.
const (
	maxWidth = 1000
	maxRows  = 20000
)

type cell struct {
	r      rune
	fg, bg colour
	blink  bool // only set when blink means blink, not bright background
}

var blank = cell{r: ' ', fg: defaultFG, bg: defaultBG}

type pen struct {
	fg, bg               colour
	bold, blink, inverse bool
}

// screen interprets ANSI.SYS-style output onto a grid of fixed width.
type screen struct {
	width   int
	ice     bool
	rows    [][]cell
	x, y    int
	pending bool // the last column was written; wrap before the next glyph
	sx, sy  int
	pen     pen
}

func newScreen(width int, ice bool) *screen {
	if width <= 0 {
		width = 80
	}
	width = min(width, maxWidth)
	return &screen{width: width, ice: ice, pen: pen{fg: defaultFG, bg: defaultBG}}
}

// run feeds data to the screen. With utf8Input the data is read as UTF-8;
// otherwise every byte is a CP437 glyph.
func (s *screen) run(data []byte, utf8Input bool) {
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b == 0x1b:
			i = s.escape(data, i+1)
			continue
		case b == '\r':
			s.x, s.pending = 0, false
		case b == '\n':
			s.x, s.pending = 0, false
			s.down(1)
		case b == '\t':
			s.pending = false
			s.x = min((s.x/8+1)*8, s.width-1)
		case utf8Input && b >= 0x80:
			r, n := utf8.DecodeRune(data[i:])
			s.put(r)
			i += n
			continue
		default:
			s.put(cp437[b])
		}
		i++
	}
}

// escape handles the sequence after an ESC at data[i-1] and returns the index
// just past it. Only CSI sequences do anything; any other ESC eats one byte.
func (s *screen) escape(data []byte, i int) int {
	if i >= len(data) {
		return i
	}
	if data[i] != '[' {
		return i + 1
	}
	i++
	start := i
	for i < len(data) && data[i] >= 0x20 && data[i] <= 0x3f {
		i++
	}
	if i >= len(data) {
		return i
	}
	params, final := string(data[start:i]), data[i]
	i++
	if strings.HasPrefix(params, "?") || strings.ContainsAny(params, " !\"#$%&'()*+,-./<=>") {
		return i // private modes and intermediates: nothing a picture needs
	}
	s.csi(splitParams(params), final)
	return i
}

func splitParams(p string) []int {
	if p == "" {
		return nil
	}
	parts := strings.Split(p, ";")
	out := make([]int, len(parts))
	for k, v := range parts {
		out[k], _ = strconv.Atoi(v) // empty means default, which is 0 here
	}
	return out
}

// arg returns parameter k, or def when it is missing or zero.
func arg(p []int, k, def int) int {
	if k < len(p) && p[k] > 0 {
		return p[k]
	}
	return def
}

func (s *screen) csi(p []int, final byte) {
	switch final {
	case 'm':
		s.sgr(p)
	case 'A':
		s.y, s.pending = max(s.y-arg(p, 0, 1), 0), false
	case 'B':
		s.down(arg(p, 0, 1))
	case 'C':
		s.x, s.pending = min(s.x+arg(p, 0, 1), s.width-1), false
	case 'D':
		s.x, s.pending = max(s.x-arg(p, 0, 1), 0), false
	case 'H', 'f':
		s.y = min(arg(p, 0, 1)-1, maxRows-1)
		s.x = min(arg(p, 1, 1)-1, s.width-1)
		s.pending = false
	case 'J':
		if arg(p, 0, 0) == 2 {
			s.rows, s.x, s.y, s.pending = nil, 0, 0, false
		}
	case 'K':
		s.eraseLine(arg(p, 0, 0))
	case 's':
		s.sx, s.sy = s.x, s.y
	case 'u':
		s.x, s.y, s.pending = s.sx, s.sy, false
	case 't':
		// PabloDraw 24-bit colour: ESC[0;R;G;Bt sets the background, ESC[1;R;G;Bt the foreground.
		if len(p) == 4 {
			c := rgb(p[1], p[2], p[3])
			if p[0] == 0 {
				s.pen.bg = c
			} else {
				s.pen.fg = c
			}
		}
	}
}

func rgb(r, g, b int) colour {
	clamp := func(v int) colour { return colour(min(max(v, 0), 255)) }
	return trueColor | clamp(r)<<16 | clamp(g)<<8 | clamp(b)
}

func (s *screen) sgr(p []int) {
	if len(p) == 0 {
		p = []int{0}
	}
	for k := 0; k < len(p); k++ {
		switch n := p[k]; {
		case n == 0:
			s.pen = pen{fg: defaultFG, bg: defaultBG}
		case n == 1:
			s.pen.bold = true
		case n == 2 || n == 22:
			s.pen.bold = false
		case n == 5 || n == 6:
			s.pen.blink = true
		case n == 25:
			s.pen.blink = false
		case n == 7:
			s.pen.inverse = true
		case n == 27:
			s.pen.inverse = false
		case n >= 30 && n <= 37:
			s.pen.fg = colour(n - 30)
		case n == 39:
			s.pen.fg = defaultFG
		case n >= 40 && n <= 47:
			s.pen.bg = colour(n - 40)
		case n == 49:
			s.pen.bg = defaultBG
		case n >= 90 && n <= 97:
			s.pen.fg = colour(n - 90 + 8)
		case n >= 100 && n <= 107:
			s.pen.bg = colour(n - 100 + 8)
		case n == 38 || n == 48:
			var c colour
			var ok bool
			c, k, ok = extended(p, k)
			if ok && n == 38 {
				s.pen.fg = c
			} else if ok {
				s.pen.bg = c
			}
		}
	}
}

// extended reads a 38/48 colour (5;N or 2;R;G;B) starting at p[k] and returns
// the index of its last parameter.
func extended(p []int, k int) (colour, int, bool) {
	switch {
	case k+2 < len(p) && p[k+1] == 5:
		return colour(min(max(p[k+2], 0), 255)), k + 2, true
	case k+4 < len(p) && p[k+1] == 2:
		return rgb(p[k+2], p[k+3], p[k+4]), k + 4, true
	}
	return 0, len(p), false
}

// cell resolves the pen the way a VGA card does: bold brightens the first
// eight foregrounds, and blink brightens the first eight backgrounds when the
// art asks for iCE colours.
func (s *screen) cell(r rune) cell {
	fg, bg, blink := s.pen.fg, s.pen.bg, false
	if s.pen.bold && fg < 8 {
		fg += 8
	}
	if s.pen.blink {
		if s.ice {
			if bg < 8 {
				bg += 8
			}
		} else {
			blink = true
		}
	}
	if s.pen.inverse {
		fg, bg = bg, fg
	}
	return cell{r: r, fg: fg, bg: bg, blink: blink}
}

func (s *screen) put(r rune) {
	if s.pending {
		s.x, s.pending = 0, false
		s.down(1)
	}
	if s.y >= maxRows {
		return
	}
	s.row(s.y)[s.x] = s.cell(r)
	if s.x == s.width-1 {
		s.pending = true
	} else {
		s.x++
	}
}

func (s *screen) down(n int) {
	s.y = min(s.y+n, maxRows)
}

func (s *screen) row(y int) []cell {
	for len(s.rows) <= y {
		s.rows = append(s.rows, nil)
	}
	if s.rows[y] == nil {
		s.rows[y] = make([]cell, s.width)
		for x := range s.rows[y] {
			s.rows[y][x] = blank
		}
	}
	return s.rows[y]
}

func (s *screen) eraseLine(mode int) {
	if s.y >= maxRows {
		return
	}
	row, c := s.row(s.y), s.cell(' ')
	from, to := s.x, s.width
	switch mode {
	case 1:
		from, to = 0, s.x+1
	case 2:
		from = 0
	}
	for x := from; x < to; x++ {
		row[x] = c
	}
}
