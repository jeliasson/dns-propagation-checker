package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeliasson/dns-propagation-checker/internal/resolve"
)

func TestSplitPositional(t *testing.T) {
	names, servers, err := splitPositional([]string{
		"example.com",
		"example.com:MX",
		"example.com/TXT",
		"@8.8.8.8",
		"@ns1.example.com",
		"@8.8.8.8:5353",
		"@2001:4860:4860::8888",
	})
	if err != nil {
		t.Fatalf("splitPositional: %v", err)
	}

	wantNames := []nameSpec{
		{name: "example.com"},
		{name: "example.com", typ: "MX"},
		{name: "example.com", typ: "TXT"},
	}
	if len(names) != len(wantNames) {
		t.Fatalf("names = %+v, want %+v", names, wantNames)
	}
	for i, want := range wantNames {
		if names[i] != want {
			t.Errorf("names[%d] = %+v, want %+v", i, names[i], want)
		}
	}

	wantServers := []string{"8.8.8.8", "ns1.example.com", "8.8.8.8:5353", "2001:4860:4860::8888"}
	if strings.Join(servers, ",") != strings.Join(wantServers, ",") {
		t.Errorf("servers = %v, want %v", servers, wantServers)
	}
}

func TestSplitPositionalKeepsNonTypeColonSuffixes(t *testing.T) {
	// "example.com:5353" is not a type, so the whole token is the name and
	// validation can reject it later rather than silently dropping the port.
	names, _, err := splitPositional([]string{"example.com:5353"})
	if err != nil {
		t.Fatalf("splitPositional: %v", err)
	}
	if len(names) != 1 || names[0].name != "example.com:5353" || names[0].typ != "" {
		t.Errorf("names = %+v", names)
	}
}

func TestSplitPositionalRejectsEmptyServer(t *testing.T) {
	if _, _, err := splitPositional([]string{"@"}); err == nil {
		t.Error("expected an error for a bare @")
	}
}

func TestBuildRecords(t *testing.T) {
	records := buildRecords(
		[]nameSpec{{name: "a.com"}, {name: "b.com", typ: "MX"}},
		stringList{"A", "AAAA"},
	)

	want := []resolve.Record{
		{FQDN: "a.com", Type: "A"},
		{FQDN: "a.com", Type: "AAAA"},
		{FQDN: "b.com", Type: "MX"}, // inline type wins over -t
	}
	if len(records) != len(want) {
		t.Fatalf("records = %+v, want %+v", records, want)
	}
	for i := range want {
		if records[i] != want[i] {
			t.Errorf("records[%d] = %+v, want %+v", i, records[i], want[i])
		}
	}
}

func TestBuildRecordsDefaultsToA(t *testing.T) {
	records := buildRecords([]nameSpec{{name: "a.com"}}, nil)
	if len(records) != 1 || records[0].Type != "A" {
		t.Errorf("records = %+v, want a single A record", records)
	}
}

func TestBuildServersSupportsLabels(t *testing.T) {
	servers := buildServers([]string{"old=1.1.1.1", "8.8.8.8"})
	if len(servers) != 2 {
		t.Fatalf("servers = %+v", servers)
	}
	if servers[0].Name != "old" || servers[0].Address != "1.1.1.1" {
		t.Errorf("servers[0] = %+v", servers[0])
	}
	if servers[1].Name != "" || servers[1].Address != "8.8.8.8" {
		t.Errorf("servers[1] = %+v", servers[1])
	}
}

func TestStringListAccumulates(t *testing.T) {
	var list stringList
	if err := list.Set("A, AAAA ,"); err != nil {
		t.Fatal(err)
	}
	if err := list.Set("MX"); err != nil {
		t.Fatal(err)
	}
	if got := list.String(); got != "A,AAAA,MX" {
		t.Errorf("list = %q, want \"A,AAAA,MX\"", got)
	}
}

func TestIsRecordType(t *testing.T) {
	for _, good := range []string{"A", "mx", "TxT"} {
		if !isRecordType(good) {
			t.Errorf("isRecordType(%q) = false", good)
		}
	}
	for _, bad := range []string{"", "5353", "NOPE"} {
		if isRecordType(bad) {
			t.Errorf("isRecordType(%q) = true", bad)
		}
	}
}

func TestWriteTable(t *testing.T) {
	result := &resolve.Result{
		DurationMS: 12,
		Diff:       true,
		Records: []resolve.RecordResult{
			{
				Record: resolve.Record{FQDN: "example.com", Type: "A"},
				Diff:   true,
				Answers: []resolve.Answer{
					{
						Server:     resolve.Server{Name: "old", Address: "1.1.1.1"},
						Values:     []string{"192.0.2.1", "192.0.2.2"},
						RCode:      "NOERROR",
						DurationMS: 4,
					},
					{
						Server: resolve.Server{Name: "dead", Address: "127.0.0.1:1"},
						Error:  "connection refused",
					},
					{
						Server: resolve.Server{Name: "empty", Address: "9.9.9.9"},
						RCode:  "NXDOMAIN",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	writeTable(&buf, result)
	out := buf.String()

	for _, want := range []string{
		"example.com A", "DIFF", "192.0.2.1", "192.0.2.2",
		"connection refused", "(no records)", "NXDOMAIN",
		"old (1.1.1.1)", "1 record(s) in 12ms",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("table written to a buffer should carry no ANSI codes")
	}
}

func TestWriteTableMarksMatches(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, &resolve.Result{
		Records: []resolve.RecordResult{{
			Record:  resolve.Record{FQDN: "example.com", Type: "A"},
			Answers: []resolve.Answer{{Server: resolve.Server{Address: "1.1.1.1"}, RCode: "NOERROR", Values: []string{"192.0.2.1"}}},
		}},
	})
	if !strings.Contains(buf.String(), "MATCH") {
		t.Errorf("expected MATCH marker:\n%s", buf.String())
	}
}
