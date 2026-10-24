// Command ans2html draws an ANSI art file (CP437, ANSI.SYS escapes, SAUCE)
// as a standalone HTML page.
//
//	ans2html [-w cols] [-encoding auto|cp437|utf8] [file.ans] > out.html
//
// With no file it reads standard input, which is how Forgejo and Gitea call
// an external renderer.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// maxInput caps how much is read; real art is a few hundred kilobytes at most.
const maxInput = 16 << 20

func main() {
	width := flag.Int("w", 0, "columns; default is the SAUCE width, else 80")
	encoding := flag.String("encoding", "auto", "auto, cp437 or utf8; auto reads valid UTF-8 as UTF-8 and anything else as CP437")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ans2html [-w cols] [-encoding auto|cp437|utf8] [file.ans]")
		flag.PrintDefaults()
	}
	flag.Parse()

	in := io.Reader(os.Stdin)
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if flag.NArg() == 1 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		in = f
	}
	data, err := io.ReadAll(io.LimitReader(in, maxInput))
	if err != nil {
		fatal(err)
	}
	if err := render(os.Stdout, data, *width, *encoding); err != nil {
		fatal(err)
	}
}

func render(out io.Writer, data []byte, width int, encoding string) error {
	art, rec := splitSauce(data)
	ice := true // a browser cannot blink sensibly; most art wants bright backgrounds
	if rec != nil {
		ice = rec.ICE
		if width == 0 {
			width = rec.Width
		}
	}
	var asUTF8 bool
	switch encoding {
	case "auto":
		asUTF8 = utf8.Valid(art)
	case "utf8":
		asUTF8 = true
	case "cp437":
	default:
		return fmt.Errorf("unknown encoding %q", encoding)
	}
	s := newScreen(width, ice)
	s.run(art, asUTF8)
	return writeDocument(out, s, rec)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ans2html:", err)
	os.Exit(1)
}
