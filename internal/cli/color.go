package cli

import (
	"io"
	"os"
)

// ANSI colors, used only when writing to a terminal.
const (
	red    = "31"
	green  = "32"
	yellow = "33"
)

// useColor reports whether w is a terminal that wants ANSI codes. It honours
// the NO_COLOR convention.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func paint(text, color string, enabled bool) string {
	if !enabled {
		return text
	}
	return "\x1b[" + color + "m" + text + "\x1b[0m"
}
