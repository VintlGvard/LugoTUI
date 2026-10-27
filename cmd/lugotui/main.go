package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/VintlGvard/LugoTUI/tui"
)

func main() {
	if err := tui.Run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
