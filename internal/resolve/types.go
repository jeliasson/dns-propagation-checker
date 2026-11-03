// Package resolve queries a set of DNS records against a set of name servers
// and reports whether the servers agree with each other.
package resolve

import "time"

// Server is a name server to query. Address may be an IPv4/IPv6 literal or a
// hostname, with an optional ":port" suffix (port 53 is assumed).
type Server struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Record is a single question: a fully qualified domain name and an RR type
// such as "A", "MX" or "TXT".
type Record struct {
	FQDN string `json:"fqdn"`
	Type string `json:"type"`
}

// Answer is the outcome of asking one Server about one Record.
type Answer struct {
	Server Server `json:"server"`

	// Values holds the RR data in presentation format, one entry per record,
	// sorted so that servers returning the same set in a different order
	// still compare equal.
	Values []string `json:"values"`

	// RCode is the response code name, e.g. "NOERROR" or "NXDOMAIN". Empty
	// when the query never got a reply.
	RCode string `json:"rcode"`

	// Error is a human readable transport or validation failure, if any.
	Error string `json:"error,omitempty"`

	DurationMS int64 `json:"duration_ms"`
}

// OK reports whether the server produced a usable reply.
func (a Answer) OK() bool { return a.Error == "" }

// RecordResult collects every server's answer for one record.
type RecordResult struct {
	Record  Record   `json:"record"`
	Answers []Answer `json:"answers"`

	// Diff is true when the servers did not all return the same thing, which
	// is the signal that a zone has not finished propagating.
	Diff bool `json:"diff"`
}

// Result is a complete run across every record and server.
type Result struct {
	Records    []RecordResult `json:"records"`
	Diff       bool           `json:"diff"`
	DurationMS int64          `json:"duration_ms"`
}

// Request describes a run. Zero values fall back to package defaults.
type Request struct {
	Servers []Server `json:"servers"`
	Records []Record `json:"records"`

	// Timeout bounds each individual query. Defaults to DefaultTimeout.
	Timeout time.Duration `json:"-"`
}
