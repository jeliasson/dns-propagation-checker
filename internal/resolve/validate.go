package resolve

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

// RecordTypes are the RR types offered in the UI and accepted by the API, in
// the order they should be presented.
var RecordTypes = []string{
	"A", "AAAA", "CAA", "CNAME", "DNSKEY", "DS", "MX",
	"NS", "PTR", "SOA", "SRV", "TLSA", "TXT",
}

var recordTypeSet = func() map[string]uint16 {
	m := make(map[string]uint16, len(RecordTypes))
	for _, t := range RecordTypes {
		m[t] = dns.StringToType[t]
	}
	return m
}()

// Limits on a single request, to keep one caller from fanning out
// unboundedly against third-party resolvers.
const (
	MaxServers = 16
	MaxRecords = 32
	MaxNameLen = 253
)

var (
	errNoServers = errors.New("at least one server is required")
	errNoRecords = errors.New("at least one record is required")
)

func normalizeType(t string) (string, uint16, error) {
	t = strings.ToUpper(strings.TrimSpace(t))
	if t == "" {
		return "", 0, errors.New("record type is required")
	}
	qtype, ok := recordTypeSet[t]
	if !ok {
		return "", 0, fmt.Errorf("unsupported record type %q", t)
	}
	return t, qtype, nil
}

// normalizeName trims and validates a domain name. It does not require a dot,
// so single-label names such as "localhost" are allowed, but every label must
// be well formed.
func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > MaxNameLen {
		return "", fmt.Errorf("name %q is longer than %d characters", name, MaxNameLen)
	}
	for _, label := range strings.Split(name, ".") {
		if err := validLabel(label); err != nil {
			return "", fmt.Errorf("invalid name %q: %w", name, err)
		}
	}
	return name, nil
}

func validLabel(label string) error {
	if label == "" {
		return errors.New("empty label")
	}
	if len(label) > 63 {
		return errors.New("label longer than 63 characters")
	}
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '*':
		default:
			return fmt.Errorf("illegal character %q", r)
		}
	}
	return nil
}

// normalizeServer validates a server address and returns it as "host:port",
// ready to hand to a DNS client.
func normalizeServer(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("server address is required")
	}

	host, port := address, "53"
	if h, p, err := net.SplitHostPort(address); err == nil {
		host, port = h, p
	} else if ip := net.ParseIP(address); ip == nil && strings.Count(address, ":") > 1 {
		// A bare IPv6 literal splits as a host/port error; keep it whole.
		return "", fmt.Errorf("invalid server address %q", address)
	}

	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid port in server address %q", address)
	}

	if ip := net.ParseIP(host); ip == nil {
		if _, err := normalizeName(host); err != nil {
			return "", fmt.Errorf("invalid server address %q", address)
		}
	}
	return net.JoinHostPort(host, port), nil
}
