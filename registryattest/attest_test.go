package registryattest

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/statement.golden.json")

const (
	testDigest = "sha256:1a667f19ec657dd0697e3de63fcf80f8393375c4baa74fe6c53e81b12823cd22"
	otherHex   = "dea7ac181bf220b3fa64a556149193fba34ba7345264361740472179410211aa"
)

var testChannel = PublishChannel{
	Channel:    ChannelOIDC,
	Repository: "acme/reports",
	Workflow:   ".github/workflows/publish.yml",
	Ref:        "refs/tags/v1.2.0",
	Commit:     "0123456789abcdef0123456789abcdef01234567",
	RunURL:     "https://github.com/acme/reports/actions/runs/42",
}

func testStatement(t *testing.T) []byte {
	t.Helper()
	pred, err := NewPredicate(testChannel, "https://bino.bi/cli")
	if err != nil {
		t.Fatalf("NewPredicate: %v", err)
	}
	st, err := NewStatement("acme", "revenue-table", "1.2.0", testDigest, pred)
	if err != nil {
		t.Fatalf("NewStatement: %v", err)
	}
	return st
}

func TestSubjectName(t *testing.T) {
	if got := SubjectName("acme", "revenue-table", "1.2.0"); got != "@acme/revenue-table@1.2.0" {
		t.Errorf("got %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	st := testStatement(t)
	if err := CheckStatement(st, "acme", "revenue-table", "1.2.0", testDigest); err != nil {
		t.Fatalf("CheckStatement: %v", err)
	}
	ch, err := ReadPublishChannel(st)
	if err != nil {
		t.Fatalf("ReadPublishChannel: %v", err)
	}
	if ch != testChannel {
		t.Errorf("channel: got %+v, want %+v", ch, testChannel)
	}
}

func TestGolden(t *testing.T) {
	got := testStatement(t)
	path := filepath.Join("testdata", "statement.golden.json")
	if *update {
		if err := os.WriteFile(path, append(got, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The file ends in a newline; the payload does not.
	if string(got)+"\n" != string(want) {
		t.Errorf("statement bytes drift:\n got %s\nwant %s", got, want)
	}
}

func TestNewStatement_Deterministic(t *testing.T) {
	if a, b := testStatement(t), testStatement(t); string(a) != string(b) {
		t.Errorf("not deterministic:\n%s\n%s", a, b)
	}
}

func TestCheckStatement_Mismatch(t *testing.T) {
	st := testStatement(t)
	cases := []struct {
		name                  string
		scope, pkg, ver, dgst string
		want                  error
	}{
		{"scope", "other", "revenue-table", "1.2.0", testDigest, ErrSubjectName},
		{"name", "acme", "other", "1.2.0", testDigest, ErrSubjectName},
		{"version", "acme", "revenue-table", "1.2.1", testDigest, ErrSubjectVersion},
		{"digest", "acme", "revenue-table", "1.2.0", "sha256:" + otherHex, ErrSubjectDigest},
		{"malformed digest", "acme", "revenue-table", "1.2.0", "sha256:ABC", ErrMalformedDigest},
		{"digest without algorithm", "acme", "revenue-table", "1.2.0", otherHex, ErrMalformedDigest},
		{"uppercase digest", "acme", "revenue-table", "1.2.0", "sha256:" + strings.ToUpper(otherHex), ErrMalformedDigest},
		{"scope with @", "@acme", "revenue-table", "1.2.0", testDigest, ErrInvalidSubject},
		{"empty version", "acme", "revenue-table", "", testDigest, ErrInvalidSubject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckStatement(st, tc.scope, tc.pkg, tc.ver, tc.dgst)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// mutate decodes the test statement, applies f and encodes it again.
func mutate(t *testing.T, f func(st *statement)) []byte {
	t.Helper()
	var st statement
	if err := json.Unmarshal(testStatement(t), &st); err != nil {
		t.Fatal(err)
	}
	f(&st)
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCheckStatement_BadPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    error
	}{
		{"not json", []byte("{"), ErrMalformedStatement},
		{"statement type", mutate(t, func(st *statement) { st.Type = "https://in-toto.io/Statement/v0.1" }), ErrStatementType},
		{"predicate type", mutate(t, func(st *statement) { st.PredicateType = "https://slsa.dev/provenance/v0.2" }), ErrPredicateType},
		{"two subjects", mutate(t, func(st *statement) { st.Subject = append(st.Subject, st.Subject[0]) }), ErrSubjectCount},
		{"no subject", mutate(t, func(st *statement) { st.Subject = nil }), ErrSubjectCount},
		{"extra digest algorithm", mutate(t, func(st *statement) { st.Subject[0].Digest["sha512"] = "00" }), ErrMalformedDigest},
		{"short subject digest", mutate(t, func(st *statement) { st.Subject[0].Digest["sha256"] = "abc" }), ErrMalformedDigest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckStatement(tc.payload, "acme", "revenue-table", "1.2.0", testDigest)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewStatement_Invalid(t *testing.T) {
	pred, err := NewPredicate(PublishChannel{Channel: ChannelUI}, "https://bino.bi/cli")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name                  string
		scope, pkg, ver, dgst string
		pred                  json.RawMessage
		want                  error
	}{
		{"scope with @", "@acme", "x", "1.0.0", testDigest, pred, ErrInvalidSubject},
		{"name with /", "acme", "a/b", "1.0.0", testDigest, pred, ErrInvalidSubject},
		{"empty name", "acme", "", "1.0.0", testDigest, pred, ErrInvalidSubject},
		{"malformed digest", "acme", "x", "1.0.0", "sha256:xyz", pred, ErrMalformedDigest},
		{"array predicate", "acme", "x", "1.0.0", testDigest, json.RawMessage(`[]`), ErrInvalidPredicate},
		{"empty predicate", "acme", "x", "1.0.0", testDigest, nil, ErrInvalidPredicate},
		{"broken predicate", "acme", "x", "1.0.0", testDigest, json.RawMessage(`{"a":`), ErrInvalidPredicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewStatement(tc.scope, tc.pkg, tc.ver, tc.dgst, tc.pred)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewPredicate_Invalid(t *testing.T) {
	if _, err := NewPredicate(PublishChannel{Channel: "email"}, "https://bino.bi/cli"); !errors.Is(err, ErrInvalidChannel) {
		t.Errorf("channel: got %v", err)
	}
	if _, err := NewPredicate(PublishChannel{Channel: ChannelPAT}, ""); !errors.Is(err, ErrInvalidBuilder) {
		t.Errorf("builder: got %v", err)
	}
}

func TestNewPredicate_MinimalChannel(t *testing.T) {
	pred, err := NewPredicate(PublishChannel{Channel: ChannelUI}, "https://bino.bi/cli")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"buildDefinition":{"buildType":"https://bino.bi/registry/publish/v1","externalParameters":{"channel":"ui"}},"runDetails":{"builder":{"id":"https://bino.bi/cli"}}}`
	if string(pred) != want {
		t.Errorf("got %s\nwant %s", pred, want)
	}
}

func TestReadPublishChannel_Invalid(t *testing.T) {
	setPred := func(p string) []byte {
		return mutate(t, func(st *statement) { st.Predicate = json.RawMessage(p) })
	}
	cases := []struct {
		name    string
		payload []byte
		want    error
	}{
		{"build type", setPred(`{"buildDefinition":{"buildType":"https://example.com/other","externalParameters":{"channel":"ui"}}}`), ErrBuildType},
		{"channel", setPred(`{"buildDefinition":{"buildType":"https://bino.bi/registry/publish/v1","externalParameters":{"channel":"email"}}}`), ErrInvalidChannel},
		{"predicate not object", setPred(`[]`), ErrInvalidPredicate},
		{"predicate type", mutate(t, func(st *statement) { st.PredicateType = "x" }), ErrPredicateType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadPublishChannel(tc.payload)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}
