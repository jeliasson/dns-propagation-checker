package resolve

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// testServer is a DNS server on localhost that answers from a fixed zone, so
// the resolver can be tested without touching the network.
type testServer struct {
	addr string
}

// newTestServer answers queries for "<name>. <TYPE>" from zone. A nil entry
// means NXDOMAIN.
func newTestServer(t *testing.T, zone map[string][]string) *testServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
		reply := new(dns.Msg)
		reply.SetReply(req)
		reply.Authoritative = true

		q := req.Question[0]
		key := strings.ToLower(q.Name) + " " + dns.TypeToString[q.Qtype]
		values, ok := zone[key]
		if !ok {
			reply.Rcode = dns.RcodeNameError
			_ = w.WriteMsg(reply)
			return
		}

		for _, value := range values {
			rr, err := dns.NewRR(q.Name + " 300 IN " + dns.TypeToString[q.Qtype] + " " + value)
			if err != nil {
				t.Errorf("bad zone entry %q: %v", value, err)
				continue
			}
			reply.Answer = append(reply.Answer, rr)
		}
		_ = w.WriteMsg(reply)
	})

	srv := &dns.Server{PacketConn: conn, Handler: mux}
	started := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(started) }

	go func() { _ = srv.ActivateAndServe() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("test DNS server did not start")
	}
	t.Cleanup(func() { _ = srv.Shutdown() })

	return &testServer{addr: conn.LocalAddr().String()}
}

func (s *testServer) server(name string) Server {
	return Server{Name: name, Address: s.addr}
}

func TestDoAgreeingServers(t *testing.T) {
	zone := map[string][]string{"example.com. A": {"192.0.2.1", "192.0.2.2"}}
	a := newTestServer(t, zone)
	b := newTestServer(t, zone)

	result, err := Do(context.Background(), Request{
		Servers: []Server{a.server("a"), b.server("b")},
		Records: []Record{{FQDN: "example.com", Type: "A"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if result.Diff {
		t.Error("expected no diff between identical servers")
	}
	if len(result.Records) != 1 || len(result.Records[0].Answers) != 2 {
		t.Fatalf("unexpected shape: %+v", result.Records)
	}
	for _, answer := range result.Records[0].Answers {
		if answer.Error != "" {
			t.Fatalf("answer error: %s", answer.Error)
		}
		if answer.RCode != "NOERROR" {
			t.Errorf("rcode = %q, want NOERROR", answer.RCode)
		}
		if got := strings.Join(answer.Values, ","); got != "192.0.2.1,192.0.2.2" {
			t.Errorf("values = %q", got)
		}
	}
}

func TestDoValuesAreOrderInsensitive(t *testing.T) {
	// The same record set, served in opposite order, must not read as a diff.
	a := newTestServer(t, map[string][]string{"example.com. A": {"192.0.2.1", "192.0.2.2"}})
	b := newTestServer(t, map[string][]string{"example.com. A": {"192.0.2.2", "192.0.2.1"}})

	result, err := Do(context.Background(), Request{
		Servers: []Server{a.server("a"), b.server("b")},
		Records: []Record{{FQDN: "example.com", Type: "A"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if result.Diff {
		t.Errorf("same values in a different order should not be a diff: %+v", result.Records[0].Answers)
	}
}

func TestDoDetectsDiff(t *testing.T) {
	a := newTestServer(t, map[string][]string{"example.com. A": {"192.0.2.1"}})
	b := newTestServer(t, map[string][]string{"example.com. A": {"198.51.100.1"}})

	result, err := Do(context.Background(), Request{
		Servers: []Server{a.server("old"), b.server("new")},
		Records: []Record{{FQDN: "example.com", Type: "A"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !result.Diff || !result.Records[0].Diff {
		t.Error("expected a diff between servers returning different addresses")
	}
}

func TestDoNXDomainIsNotAnError(t *testing.T) {
	a := newTestServer(t, map[string][]string{})

	result, err := Do(context.Background(), Request{
		Servers: []Server{a.server("a")},
		Records: []Record{{FQDN: "missing.example.com", Type: "A"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	answer := result.Records[0].Answers[0]
	if answer.Error != "" {
		t.Fatalf("unexpected error: %s", answer.Error)
	}
	if answer.RCode != "NXDOMAIN" {
		t.Errorf("rcode = %q, want NXDOMAIN", answer.RCode)
	}
	if len(answer.Values) != 0 {
		t.Errorf("values = %v, want none", answer.Values)
	}
}

func TestDoUnreachableServerIsRecordedNotFatal(t *testing.T) {
	good := newTestServer(t, map[string][]string{"example.com. A": {"192.0.2.1"}})

	result, err := Do(context.Background(), Request{
		// Port 1 on loopback refuses or times out; either way it must not
		// fail the whole run.
		Servers: []Server{good.server("good"), {Name: "dead", Address: "127.0.0.1:1"}},
		Records: []Record{{FQDN: "example.com", Type: "A"}},
		Timeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Do should not fail on an unreachable server: %v", err)
	}

	if result.Records[0].Answers[0].Error != "" {
		t.Error("reachable server should have answered")
	}
	if result.Records[0].Answers[1].Error == "" {
		t.Error("unreachable server should have recorded an error")
	}
	if !result.Diff {
		t.Error("an unreachable server must count as a disagreement")
	}
}

func TestDoRecordsAndAnswersKeepRequestOrder(t *testing.T) {
	// The frontend lines columns up by index, so order must be stable.
	zone := map[string][]string{
		"a.example.com. A": {"192.0.2.1"},
		"b.example.com. A": {"192.0.2.2"},
	}
	s1 := newTestServer(t, zone)
	s2 := newTestServer(t, zone)

	result, err := Do(context.Background(), Request{
		Servers: []Server{s1.server("first"), s2.server("second")},
		Records: []Record{
			{FQDN: "b.example.com", Type: "A"},
			{FQDN: "a.example.com", Type: "A"},
		},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if result.Records[0].Record.FQDN != "b.example.com" || result.Records[1].Record.FQDN != "a.example.com" {
		t.Errorf("record order changed: %+v", result.Records)
	}
	for _, record := range result.Records {
		if record.Answers[0].Server.Name != "first" || record.Answers[1].Server.Name != "second" {
			t.Errorf("answer order changed: %+v", record.Answers)
		}
	}
}

func TestDoTXTValuesStripHeader(t *testing.T) {
	a := newTestServer(t, map[string][]string{`example.com. TXT`: {`"v=spf1 -all"`}})

	result, err := Do(context.Background(), Request{
		Servers: []Server{a.server("a")},
		Records: []Record{{FQDN: "example.com", Type: "TXT"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	got := result.Records[0].Answers[0].Values
	if len(got) != 1 || got[0] != `"v=spf1 -all"` {
		t.Errorf("values = %q, want the bare RR data", got)
	}
}

func TestDoDefaultsToPackageServers(t *testing.T) {
	// No servers given is legal; no records is not.
	if _, err := Do(context.Background(), Request{Records: nil}); err == nil {
		t.Error("expected an error when no records are given")
	}
}

func TestDoRejectsOversizedRequests(t *testing.T) {
	records := make([]Record, MaxRecords+1)
	for i := range records {
		records[i] = Record{FQDN: "example.com", Type: "A"}
	}
	if _, err := Do(context.Background(), Request{Records: records}); err == nil {
		t.Error("expected too-many-records error")
	}

	servers := make([]Server, MaxServers+1)
	for i := range servers {
		servers[i] = Server{Address: "1.1.1.1"}
	}
	_, err := Do(context.Background(), Request{
		Servers: servers,
		Records: []Record{{FQDN: "example.com", Type: "A"}},
	})
	if err == nil {
		t.Error("expected too-many-servers error")
	}
}

func TestDoRejectsInvalidInput(t *testing.T) {
	cases := []Request{
		{Records: []Record{{FQDN: "a;rm -rf /", Type: "A"}}},
		{Records: []Record{{FQDN: "example.com", Type: "NOPE"}}},
		{Records: []Record{{FQDN: "example.com", Type: "A"}}, Servers: []Server{{Address: "not a host"}}},
	}
	for i, req := range cases {
		if _, err := Do(context.Background(), req); err == nil {
			t.Errorf("case %d: expected a validation error", i)
		}
	}
}

func TestDoHonoursCancelledContext(t *testing.T) {
	a := newTestServer(t, map[string][]string{"example.com. A": {"192.0.2.1"}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Do(ctx, Request{
		Servers: []Server{a.server("a")},
		Records: []Record{{FQDN: "example.com", Type: "A"}},
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if result.Records[0].Answers[0].Error == "" {
		t.Error("expected the cancelled context to be reported on the answer")
	}
}

func TestHasDiff(t *testing.T) {
	ok := func(values ...string) Answer {
		return Answer{RCode: "NOERROR", Values: values}
	}

	tests := []struct {
		name    string
		answers []Answer
		want    bool
	}{
		{name: "single answer", answers: []Answer{ok("192.0.2.1")}, want: false},
		{name: "identical", answers: []Answer{ok("192.0.2.1"), ok("192.0.2.1")}, want: false},
		{name: "different values", answers: []Answer{ok("192.0.2.1"), ok("198.51.100.1")}, want: true},
		{name: "subset", answers: []Answer{ok("192.0.2.1"), ok("192.0.2.1", "192.0.2.2")}, want: true},
		{name: "both empty noerror", answers: []Answer{ok(), ok()}, want: false},
		{
			name:    "same values different rcode",
			answers: []Answer{{RCode: "NOERROR"}, {RCode: "NXDOMAIN"}},
			want:    true,
		},
		{
			name:    "one errored",
			answers: []Answer{ok("192.0.2.1"), {Error: "timeout"}},
			want:    true,
		},
		{
			name:    "every server errored the same way is still not consensus",
			answers: []Answer{{Error: "timeout"}, {Error: "timeout"}},
			want:    true,
		},
		{
			name:    "lone errored server",
			answers: []Answer{{Error: "timeout"}},
			want:    true,
		},
		{
			name:    "third server disagrees",
			answers: []Answer{ok("192.0.2.1"), ok("192.0.2.1"), ok("198.51.100.1")},
			want:    true,
		},
	}

	for _, tc := range tests {
		if got := hasDiff(tc.answers); got != tc.want {
			t.Errorf("%s: hasDiff = %v, want %v", tc.name, got, tc.want)
		}
	}
}
