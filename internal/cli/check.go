package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jeliasson/dns-propagation-checker/internal/resolve"
)

// errDiff is returned when -fail-on-diff is set and servers disagreed.
var errDiff = errors.New("name servers disagree")

type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var types, servers stringList
	fs.Var(&types, "t", "record types for bare names, comma separated (default A)")
	fs.Var(&types, "type", "alias for -t")
	fs.Var(&servers, "s", "name server to query, repeatable (also accepted as @server)")
	fs.Var(&servers, "server", "alias for -s")
	asJSON := fs.Bool("json", false, "emit the raw JSON result instead of a table")
	timeout := fs.Duration("timeout", resolve.DefaultTimeout, "per-query timeout")
	failOnDiff := fs.Bool("fail-on-diff", false, "exit with code 2 when servers disagree")

	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: dnspc check [flags] <name>[:TYPE]... [@server]...\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}

	names, positionalServers, err := splitPositional(fs.Args())
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%w: no name to check", errUsage)
	}

	req := resolve.Request{
		Records: buildRecords(names, types),
		Servers: buildServers(append([]string(servers), positionalServers...)),
		Timeout: *timeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	result, err := resolve.Do(ctx, req)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	} else {
		writeTable(stdout, result)
	}

	if result.Diff && *failOnDiff {
		return errDiff
	}
	return nil
}

// nameSpec is one positional name argument, with the type it carried inline.
type nameSpec struct {
	name string
	typ  string // empty when the name had no ":TYPE" suffix
}

// splitPositional separates "@server" arguments from name specs. A name may
// carry its type inline as "example.com:MX" or "example.com/MX".
func splitPositional(args []string) ([]nameSpec, []string, error) {
	var names []nameSpec
	var servers []string

	for _, arg := range args {
		if after, ok := strings.CutPrefix(arg, "@"); ok {
			if after == "" {
				return nil, nil, fmt.Errorf("%w: empty server in %q", errUsage, arg)
			}
			servers = append(servers, after)
			continue
		}

		spec := nameSpec{name: arg}
		for _, sep := range []string{"/", ":"} {
			if name, typ, ok := strings.Cut(arg, sep); ok {
				// Don't mistake an IPv6 literal or a port for a type.
				if sep == ":" && !isRecordType(typ) {
					break
				}
				spec = nameSpec{name: name, typ: typ}
				break
			}
		}
		names = append(names, spec)
	}
	return names, servers, nil
}

func isRecordType(candidate string) bool {
	candidate = strings.ToUpper(candidate)
	for _, t := range resolve.RecordTypes {
		if t == candidate {
			return true
		}
	}
	return false
}

// buildRecords expands each bare name across the requested types, while names
// that specified their own type are used as given.
func buildRecords(names []nameSpec, types stringList) []resolve.Record {
	if len(types) == 0 {
		types = stringList{"A"}
	}

	records := make([]resolve.Record, 0, len(names))
	for _, spec := range names {
		if spec.typ != "" {
			records = append(records, resolve.Record{FQDN: spec.name, Type: spec.typ})
			continue
		}
		for _, typ := range types {
			records = append(records, resolve.Record{FQDN: spec.name, Type: typ})
		}
	}
	return records
}

func buildServers(addresses []string) []resolve.Server {
	servers := make([]resolve.Server, 0, len(addresses))
	for _, address := range addresses {
		// "label=address" lets a column be named in the output.
		if label, addr, ok := strings.Cut(address, "="); ok {
			servers = append(servers, resolve.Server{Name: label, Address: addr})
			continue
		}
		servers = append(servers, resolve.Server{Address: address})
	}
	return servers
}

// writeTable prints one block per record: the values each server returned,
// with a MATCH/DIFF marker.
func writeTable(w io.Writer, result *resolve.Result) {
	color := useColor(w)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	for i, record := range result.Records {
		if i > 0 {
			// A line with no tabs ends the alignment block, so each record
			// gets columns sized to its own values.
			fmt.Fprintln(tw)
		}

		marker, tint := "MATCH", green
		if record.Diff {
			marker, tint = "DIFF", yellow
		}
		fmt.Fprintf(tw, "%s %s\t%s\n",
			record.Record.FQDN, record.Record.Type, paint(marker, tint, color))

		for _, answer := range record.Answers {
			label := answer.Server.Name
			if label != answer.Server.Address {
				label = fmt.Sprintf("%s (%s)", label, answer.Server.Address)
			}

			switch {
			case answer.Error != "":
				fmt.Fprintf(tw, "  %s\t%s\n", label, paint(answer.Error, red, color))
			case len(answer.Values) == 0:
				fmt.Fprintf(tw, "  %s\t%s\t%s\n", label, "(no records)", answer.RCode)
			default:
				for j, value := range answer.Values {
					if j == 0 {
						fmt.Fprintf(tw, "  %s\t%s\t%s %dms\n",
							label, value, answer.RCode, answer.DurationMS)
					} else {
						fmt.Fprintf(tw, "  \t%s\t\n", value)
					}
				}
			}
		}
	}

	tw.Flush()
	fmt.Fprintf(w, "\n%d record(s) in %s\n",
		len(result.Records), time.Duration(result.DurationMS)*time.Millisecond)
}
