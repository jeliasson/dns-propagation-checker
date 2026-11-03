package resolve

import "testing"

func TestNormalizeServer(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "8.8.8.8", want: "8.8.8.8:53"},
		{in: " 1.1.1.1 ", want: "1.1.1.1:53"},
		{in: "8.8.8.8:5353", want: "8.8.8.8:5353"},
		{in: "ns1.example.com", want: "ns1.example.com:53"},
		{in: "2001:4860:4860::8888", want: "[2001:4860:4860::8888]:53"},
		{in: "[2001:4860:4860::8888]:5353", want: "[2001:4860:4860::8888]:5353"},
		{in: "::1", want: "[::1]:53"},
		{in: "", wantErr: true},
		{in: "8.8.8.8:0", wantErr: true},
		{in: "8.8.8.8:99999", wantErr: true},
		{in: "8.8.8.8:http", wantErr: true},
		{in: "bad host", wantErr: true},
		{in: "a;rm -rf /", wantErr: true},
		{in: "$(whoami).example.com", wantErr: true},
		{in: "foo:bar:baz", wantErr: true},
	}

	for _, tc := range tests {
		got, err := normalizeServer(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeServer(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeServer(%q) returned %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeServer(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "example.com", want: "example.com"},
		{in: "example.com.", want: "example.com"},
		{in: "  EXAMPLE.com  ", want: "EXAMPLE.com"},
		{in: "_dmarc.example.com", want: "_dmarc.example.com"},
		{in: "*.example.com", want: "*.example.com"},
		{in: "localhost", want: "localhost"},
		{in: "", wantErr: true},
		{in: ".", wantErr: true},
		{in: "example..com", wantErr: true},
		{in: "exa mple.com", wantErr: true},
		{in: "a;id.com", wantErr: true},
		{in: "example.com/../etc", wantErr: true},
	}

	for _, tc := range tests {
		got, err := normalizeName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeName(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeName(%q) returned %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeNameRejectsOverlongLabel(t *testing.T) {
	label := make([]byte, 64)
	for i := range label {
		label[i] = 'a'
	}
	if _, err := normalizeName(string(label) + ".com"); err == nil {
		t.Fatal("expected 64-character label to be rejected")
	}
}

func TestNormalizeType(t *testing.T) {
	if name, qtype, err := normalizeType(" mx "); err != nil || name != "MX" || qtype == 0 {
		t.Fatalf("normalizeType(\" mx \") = %q, %d, %v", name, qtype, err)
	}
	for _, bad := range []string{"", "NOPE", "ANY", "A;B"} {
		if _, _, err := normalizeType(bad); err == nil {
			t.Errorf("normalizeType(%q) should have failed", bad)
		}
	}
}

func TestRecordTypesAllResolveToWireTypes(t *testing.T) {
	for _, name := range RecordTypes {
		if recordTypeSet[name] == 0 {
			t.Errorf("record type %q has no wire type", name)
		}
	}
}
