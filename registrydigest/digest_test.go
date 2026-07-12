package registrydigest

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// ---- Reformat invariance (the headline property) ----

func TestReformatInvariance(t *testing.T) {
	base, err := Digest([]byte(revenueTableYAML))
	if err != nil {
		t.Fatalf("base digest: %v", err)
	}
	variants := map[string]string{
		"reindented_reordered_comments_stripped": revenueTableReformattedYAML,
		"json_input":                             revenueTableJSON,
	}
	for name, doc := range variants {
		t.Run(name, func(t *testing.T) {
			got, err := Digest([]byte(doc))
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if got != base {
				t.Errorf("variant digest differs from base:\n got %s\nbase %s", got, base)
			}
		})
	}
}

func TestSemanticChangeFlipsDigest(t *testing.T) {
	base, err := Digest([]byte(revenueTableYAML))
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(revenueTableYAML, "grouped: true", "grouped: false", 1)
	got, err := Digest([]byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	if got == base {
		t.Errorf("semantic change did not change the digest")
	}
}

// ---- Rejection cases (every sentinel) ----

func TestReject_Cases(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"empty_string", "", ErrEmptyDocument},
		{"whitespace_only", "  \n# c\n", ErrEmptyDocument},
		{"multidoc", "a: 1\n---\nb: 2", ErrMultiDocument},
		{"anchor_alias", "a: &x 1\nb: *x", ErrAnchorsUnsupported},
		{"merge_key", "base: &base {a: 1}\nx:\n  <<: *base\n  b: 2", ErrAnchorsUnsupported},
		{"explicit_tag_binary", "x: !!binary aGk=", ErrExplicitTag},
		{"explicit_tag_timestamp", "x: !!timestamp 2026-01-01", ErrExplicitTag},
		{"custom_tag", "x: !Custom {}", ErrExplicitTag},
		{"dup_key", "a: 1\na: 2", ErrDuplicateKey},
		{"dup_key_nested", "m:\n  k: 1\n  k: 2", ErrDuplicateKey},
		{"dup_key_normalized", "m:\n  1: a\n  1: b", ErrDuplicateKey},
		{"non_string_key", "? [a, b]\n: v", ErrNonStringKey},
		{"invalid_yaml", "kind: [unterminated", ErrInvalidYAML},
		{"int_2pow53", "n: 9007199254740992", ErrUnsafeNumber},
		{"int_neg_2pow53", "n: -9007199254740992", ErrUnsafeNumber},
		{"int_huge", "n: 1" + strings.Repeat("0", 40), ErrUnsafeNumber},
		{"float_high_precision", "n: 0.1234567890123456789", ErrUnsafeNumber},
		{"float_nan", "n: .nan", ErrUnsafeNumber},
		{"float_inf", "n: .inf", ErrUnsafeNumber},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Digest([]byte(tc.input))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got err %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

// ---- Number boundary + canonical forms ----

func TestNumberAndScalarCanonical(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string // canonical bytes
	}{
		{"int_max_safe", "n: 9007199254740991", `{"n":9007199254740991}`},
		{"int_min_safe", "n: -9007199254740991", `{"n":-9007199254740991}`},
		{"float_zero_forms", "a: 0\nb: 0.0\nc: -0.0", `{"a":0,"b":0,"c":0}`},
		{"float_exponent", "n: 1e3", `{"n":1000}`},
		{"float_small", "n: 1.5", `{"n":1.5}`},
		{"string_escaping", "s: \"a\\\"b\\\\c\td\"", `{"s":"a\"b\\c\td"}`},
		{"bool_null", "a: true\nb: false\nc: null\nd: ~", `{"a":true,"b":false,"c":null,"d":null}`},
		{"nested_arrays", "xs: [[1,2],[3]]", `{"xs":[[1,2],[3]]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Canonicalize([]byte(tc.input))
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNumberBoundary_SafeAccepted(t *testing.T) {
	for _, in := range []string{"n: 9007199254740991", "n: -9007199254740991"} {
		if _, err := Digest([]byte(in)); err != nil {
			t.Errorf("%q should be accepted, got %v", in, err)
		}
	}
}

// ---- UTF-16 key ordering (S14) ----

func TestUTF16KeyOrder(t *testing.T) {
	// A supplementary-plane char (U+1F600) encodes to a high surrogate (0xD83D)
	// in UTF-16, which sorts BEFORE a BMP char in the private-use range (U+E000).
	// By UTF-8 byte order and by rune value the supplementary char sorts AFTER.
	doc := "\"\": 1\n\"\U0001F600\": 2"
	got, err := Canonicalize([]byte(doc))
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	want := "{\"\U0001F600\":2,\"\":1}"
	if string(got) != want {
		t.Errorf("UTF-16 key order wrong:\n got %q\nwant %q", got, want)
	}
}

// ---- SelfCheck idempotency ----

func TestSelfCheck_Idempotent(t *testing.T) {
	valid := []string{
		revenueTableYAML, revenueTableReformattedYAML, revenueTableJSON,
		styleAYAML, cohortRetentionYAML,
		"n: 1.5", "xs: [[1,2],[3]]", "a: true\nb: null",
	}
	for _, doc := range valid {
		if err := SelfCheck([]byte(doc)); err != nil {
			t.Errorf("SelfCheck failed for valid doc: %v", err)
		}
		c1, err := Canonicalize([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		c2, err := Canonicalize(c1)
		if err != nil {
			t.Fatal(err)
		}
		if string(c1) != string(c2) {
			t.Errorf("Canonicalize not byte-idempotent for %q", doc)
		}
	}
}

// ---- Concurrency ----

func TestConcurrent_RaceClean(t *testing.T) {
	want, err := Digest([]byte(revenueTableYAML))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Digest([]byte(revenueTableYAML))
			if err != nil || got != want {
				t.Errorf("concurrent digest mismatch: %v / %s", err, got)
			}
		}()
	}
	wg.Wait()
}

// ---- Fuzz ----

func FuzzCanonicalize(f *testing.F) {
	for _, seed := range []string{
		revenueTableYAML, "a: 1", "n: 1.5", "xs: [1,2,3]", "", "k: v",
		"a: &x 1\nb: *x", "x: !!binary aGk=", "n: 9007199254740992",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := Canonicalize(data)
		if err != nil {
			return // a rejected input is fine; we only require no panic
		}
		if !utf8.Valid(out) {
			t.Errorf("canonical output is not valid UTF-8: %q", out)
		}
		again, err := Canonicalize(out)
		if err != nil {
			t.Errorf("re-canonicalize of accepted output failed: %v", err)
		}
		if string(again) != string(out) {
			t.Errorf("canonicalize not idempotent on accepted output")
		}
	})
}
