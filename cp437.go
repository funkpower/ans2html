package main

// cp437 maps each IBM code page 437 byte to the glyph a VGA text-mode screen
// draws for it. The C0 range is drawn as glyphs too, as it is on screen; the
// interpreter picks out the few control bytes it acts on before looking here.
var cp437 [256]rune

func init() {
	low := []rune(" ☺☻♥♦♣♠•◘○◙♂♀♪♫☼►◄↕‼¶§▬↨↑↓→←∟↔▲▼")
	high := []rune("⌂" +
		"ÇüéâäàåçêëèïîìÄÅ" +
		"ÉæÆôöòûùÿÖÜ¢£¥₧ƒ" +
		"áíóúñÑªº¿⌐¬½¼¡«»" +
		"░▒▓│┤╡╢╖╕╣║╗╝╜╛┐" +
		"└┴┬├─┼╞╟╚╔╩╦╠═╬╧" +
		"╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀" +
		"αßΓπΣσµτΦΘΩδ∞φε∩" +
		"≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ ")
	if len(low) != 0x20 || len(high) != 0x81 {
		panic("cp437 table has the wrong length")
	}
	copy(cp437[:0x20], low)
	for b := 0x20; b < 0x7f; b++ {
		cp437[b] = rune(b)
	}
	copy(cp437[0x7f:], high)
}
