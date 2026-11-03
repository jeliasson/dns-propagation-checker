package resolve

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	// DefaultTimeout bounds a single query against a single server.
	DefaultTimeout = 5 * time.Second

	// maxInFlight caps how many queries are on the wire at once.
	maxInFlight = 16
)

// DefaultServers are used when a request names no servers of its own.
var DefaultServers = []Server{
	{Name: "Cloudflare", Address: "1.1.1.1"},
	{Name: "Google", Address: "8.8.8.8"},
	{Name: "Quad9", Address: "9.9.9.9"},
}

// Do runs every record against every server concurrently and reports where
// the servers disagree.
//
// A failure to reach one server is recorded on that Answer rather than
// failing the run; only a malformed request returns an error.
func Do(ctx context.Context, req Request) (*Result, error) {
	servers, records, err := normalizeRequest(req)
	if err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	result := &Result{Records: make([]RecordResult, len(records))}
	for i, rec := range records {
		result.Records[i] = RecordResult{
			Record:  rec.Record,
			Answers: make([]Answer, len(servers)),
		}
	}

	started := time.Now()
	sem := make(chan struct{}, maxInFlight)
	var wg sync.WaitGroup

	for i, rec := range records {
		for j, srv := range servers {
			wg.Add(1)
			go func(i, j int, rec question, srv target) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					result.Records[i].Answers[j] = Answer{
						Server: srv.Server,
						Values: []string{},
						Error:  ctx.Err().Error(),
					}
					return
				}
				result.Records[i].Answers[j] = query(ctx, rec, srv, timeout)
			}(i, j, rec, srv)
		}
	}
	wg.Wait()

	for i := range result.Records {
		result.Records[i].Diff = hasDiff(result.Records[i].Answers)
		if result.Records[i].Diff {
			result.Diff = true
		}
	}
	result.DurationMS = time.Since(started).Milliseconds()
	return result, nil
}

// question is a validated Record plus its wire type.
type question struct {
	Record Record
	QType  uint16
}

// target is a validated Server plus its dialable "host:port".
type target struct {
	Server Server
	Addr   string
}

func normalizeRequest(req Request) ([]target, []question, error) {
	servers := req.Servers
	if len(servers) == 0 {
		servers = DefaultServers
	}
	if len(servers) > MaxServers {
		return nil, nil, fmt.Errorf("too many servers: %d (max %d)", len(servers), MaxServers)
	}
	if len(req.Records) == 0 {
		return nil, nil, errNoRecords
	}
	if len(req.Records) > MaxRecords {
		return nil, nil, fmt.Errorf("too many records: %d (max %d)", len(req.Records), MaxRecords)
	}

	targets := make([]target, 0, len(servers))
	for _, srv := range servers {
		addr, err := normalizeServer(srv.Address)
		if err != nil {
			return nil, nil, err
		}
		name := strings.TrimSpace(srv.Name)
		if name == "" {
			name = srv.Address
		}
		targets = append(targets, target{
			Server: Server{Name: name, Address: strings.TrimSpace(srv.Address)},
			Addr:   addr,
		})
	}
	if len(targets) == 0 {
		return nil, nil, errNoServers
	}

	questions := make([]question, 0, len(req.Records))
	for _, rec := range req.Records {
		name, err := normalizeName(rec.FQDN)
		if err != nil {
			return nil, nil, err
		}
		rtype, qtype, err := normalizeType(rec.Type)
		if err != nil {
			return nil, nil, err
		}
		questions = append(questions, question{
			Record: Record{FQDN: name, Type: rtype},
			QType:  qtype,
		})
	}
	return targets, questions, nil
}

// query asks one server one question, retrying over TCP if the UDP reply was
// truncated.
func query(ctx context.Context, q question, srv target, timeout time.Duration) Answer {
	answer := Answer{Server: srv.Server, Values: []string{}}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(q.Record.FQDN), q.QType)
	msg.RecursionDesired = true

	client := &dns.Client{Timeout: timeout}
	started := time.Now()

	reply, _, err := client.ExchangeContext(ctx, msg, srv.Addr)
	if err == nil && reply.Truncated {
		client.Net = "tcp"
		if tcpReply, _, tcpErr := client.ExchangeContext(ctx, msg, srv.Addr); tcpErr == nil {
			reply = tcpReply
		}
	}
	answer.DurationMS = time.Since(started).Milliseconds()

	if err != nil {
		answer.Error = cleanErr(err)
		return answer
	}

	answer.RCode = rcodeName(reply.Rcode)
	for _, rr := range reply.Answer {
		if rr.Header().Rrtype != q.QType {
			continue // CNAME in the chain to the requested type
		}
		answer.Values = append(answer.Values, rrValue(rr))
	}
	sort.Strings(answer.Values)
	return answer
}

// rrValue renders an RR's data without the owner/TTL/class preamble.
func rrValue(rr dns.RR) string {
	text := rr.String()
	header := rr.Header().String()
	return strings.TrimSpace(strings.TrimPrefix(text, header))
}

func rcodeName(code int) string {
	if name, ok := dns.RcodeToString[code]; ok {
		return name
	}
	return fmt.Sprintf("RCODE%d", code)
}

// cleanErr strips the noisy address prefix Go puts on network errors.
func cleanErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && strings.HasPrefix(msg, "read ") {
		return msg[i+2:]
	}
	return msg
}

// hasDiff reports whether the answers disagree. Two answers agree when they
// carry the same sorted value set and the same response code.
//
// Any server that failed to answer makes the record a diff: a server we could
// not reach has not been shown to agree, and reporting consensus on partial
// data is exactly the mistake this tool exists to prevent.
func hasDiff(answers []Answer) bool {
	for _, a := range answers {
		if !a.OK() {
			return true
		}
	}
	if len(answers) < 2 {
		return false
	}
	first := signature(answers[0])
	for _, a := range answers[1:] {
		if signature(a) != first {
			return true
		}
	}
	return false
}

func signature(a Answer) string {
	return a.RCode + "\x00" + strings.Join(a.Values, "\x00")
}
