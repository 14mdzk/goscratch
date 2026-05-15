package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: scaffold <subcommand> [args]\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  module <name>   Scaffold a new module under internal/module/<name>\n")
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "module":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "error: module name is required (usage: scaffold module <name>)")
			os.Exit(1)
		}
		if err := runModule(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown subcommand %q\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}
