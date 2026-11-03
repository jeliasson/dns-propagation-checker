package cli

import (
	"flag"
	"strings"
)

// boolFlag matches flag values that take no argument.
type boolFlag interface {
	IsBoolFlag() bool
}

// reorderArgs moves flags ahead of positional arguments.
//
// The standard flag package stops parsing at the first non-flag argument, so
// `dnspc check example.com --json` would otherwise read "--json" as a domain
// name. Everything after a bare "--" is kept positional.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case len(arg) < 2 || arg[0] != '-':
			positional = append(positional, arg)
		default:
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if strings.Contains(name, "=") {
				continue // -flag=value carries its own value
			}
			// A flag that takes a value consumes the next argument.
			if f := fs.Lookup(name); f != nil && !takesNoValue(f) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		}
	}
	return append(flags, positional...)
}

func takesNoValue(f *flag.Flag) bool {
	bf, ok := f.Value.(boolFlag)
	return ok && bf.IsBoolFlag()
}
