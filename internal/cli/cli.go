// Package cli implements the command line interface: a `check` subcommand for
// one-off and CI use, and `serve` for the web app.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes. ExitDiff is only used when `check -fail-on-diff` is set, so
// that plain interactive use never looks like a failure.
const (
	ExitOK    = 0
	ExitError = 1
	ExitDiff  = 2
)

// Version is stamped at build time with -ldflags "-X ...cli.Version=...".
var Version = "dev"

// errUsage signals that usage should be printed alongside the error.
var errUsage = errors.New("usage")

// Run dispatches args (without the program name) and returns an exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitError
	}

	var err error
	switch args[0] {
	case "check":
		err = runCheck(args[1:], stdout, stderr)
	case "serve":
		err = runServe(args[1:], stderr)
	case "healthcheck":
		err = runHealthcheck(args[1:], stderr)
	case "version", "--version", "-version":
		fmt.Fprintf(stdout, "dnspc %s\n", Version)
	case "help", "--help", "-h":
		usage(stdout)
	default:
		// Bare invocation: `dnspc example.com @1.1.1.1` means check.
		err = runCheck(args, stdout, stderr)
	}

	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, errDiff):
		return ExitDiff
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "dnspc: %v\n\n", err)
		usage(stderr)
		return ExitError
	default:
		fmt.Fprintf(stderr, "dnspc: %v\n", err)
		return ExitError
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `dnspc checks DNS propagation across many name servers.

Usage:
  dnspc check [flags] <name>[:TYPE]... [@server]...
  dnspc serve [flags]
  dnspc healthcheck [flags]
  dnspc version

Examples:
  dnspc check example.com                          # A record, default resolvers
  dnspc check example.com:NS example.com:MX        # several records
  dnspc check example.com -t A,AAAA,MX             # types applied to bare names
  dnspc check example.com @8.8.8.8 @ns1.example.com
  dnspc check example.com:NS --json | jq .
  dnspc serve --addr :3000

Run "dnspc check -h" or "dnspc serve -h" for flags.
`)
}

// Main is the entry point used by cmd/dnspc.
func Main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
