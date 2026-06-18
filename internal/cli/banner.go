package cli

import (
	"fmt"
	"io"

	"github.com/nficano/github-runner/internal/version"
)

// banner is the ASCII art logo printed at startup. It is suppressed when the
// global hide_banner configuration option is set to true.
//
// The literal is split around a single backtick (the tail of the lowercase
// "g") so the surrounding raw strings—which let the backslashes in the art
// stand for themselves—remain valid.
const banner = `
        _ _   _           _
   __ _(_) |_| |__  _   _| |__       _ __ _   _ _ __  _ __   ___ _ __
  / _` + "`" + ` | | __| '_ \| | | | '_ \ ____| '__| | | | '_ \| '_ \ / _ \ '__|
 | (_| | | |_| | | | |_| | |_) |____| |  | |_| | | | | | | |  __/ |
  \__, |_|\__|_| |_|\__,_|_.__/     |_|   \__,_|_| |_|_| |_|\___|_|
  |___/`

// printBanner writes the ASCII art banner followed by a tagline and version
// line to w. It is a no-op when hidden is true, which corresponds to the
// global hide_banner configuration option.
func printBanner(w io.Writer, hidden bool) {
	if hidden {
		return
	}
	fmt.Fprintln(w, banner)
	fmt.Fprintf(w, "  self-hosted GitHub Actions runner · %s\n\n", version.Get().Version)
}
