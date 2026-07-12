// Package registrydigest is the single shared RFC 8785 (JCS) canonicalizer and
// sha256 content-digest used by the bino Registry. The service calls it at
// publish to compute and freeze a version's digest; the CLI plugin calls the
// identical function at verify/pull to recompute and compare against the
// lockfile. There is exactly one implementation, so server and plugin can never
// drift. The digest is SEMANTIC: reformatting, re-commenting, or re-ordering map
// keys of a stored bino Document does not change it.
package registrydigest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"gopkg.in/yaml.v3"
)

// Sentinel errors. Every one is reported by callers as the registry error code
// "not_canonicalizable" (HTTP 422). Match with errors.Is.
var (
	// ErrMultiDocument is returned when the input is a YAML multi-document
	// stream (more than one document separated by "---").
	ErrMultiDocument = errors.New("registrydigest: multi-document stream")

	// ErrAnchorsUnsupported is returned for YAML anchors, aliases, or merge
	// keys ("&a", "*a", "<<").
	ErrAnchorsUnsupported = errors.New("registrydigest: anchors/aliases/merge keys unsupported")

	// ErrExplicitTag is returned for an explicit or custom YAML tag outside the
	// YAML 1.2 core schema (e.g. "!!binary", "!Foo", "!!timestamp").
	ErrExplicitTag = errors.New("registrydigest: explicit/custom tag unsupported")

	// ErrDuplicateKey is returned when a mapping contains the same key twice.
	ErrDuplicateKey = errors.New("registrydigest: duplicate map key")

	// ErrNonStringKey is returned when a mapping key does not resolve to a string.
	ErrNonStringKey = errors.New("registrydigest: non-string map key")

	// ErrUnsafeNumber is returned for a number that is not JCS-safe: not a
	// finite IEEE-754 double, or an integer with magnitude greater than 2^53-1.
	ErrUnsafeNumber = errors.New("registrydigest: number not JCS-safe")

	// ErrEmptyDocument is returned when the input contains no YAML document.
	ErrEmptyDocument = errors.New("registrydigest: empty document")

	// ErrInvalidYAML is returned when the input is not well-formed YAML/JSON.
	ErrInvalidYAML = errors.New("registrydigest: invalid YAML")
)

// maxSafeInteger is 2^53-1, the largest integer a JCS number may carry.
const maxSafeInteger = 9007199254740991.0

// Digest canonicalizes a stored bino Document (YAML or JSON bytes) per RFC 8785
// (JCS) and returns its content digest as "sha256:<hex>" (lowercase hex). On any
// non-canonicalizable input it returns a wrapped sentinel error (see the package
// vars); callers report these as not_canonicalizable.
func Digest(doc []byte) (string, error) {
	digest, _, err := DigestWithCanonical(doc)
	return digest, err
}

// Canonicalize parses a stored bino Document and returns its RFC 8785 (JCS)
// canonical JSON bytes (UTF-8, sorted keys, minimal escaping, RFC 8785 numbers).
// It applies the same parse/reject rules as Digest. The returned bytes are
// deterministic and byte-identical across builds.
func Canonicalize(doc []byte) ([]byte, error) {
	root, err := parse(doc)
	if err != nil {
		return nil, err
	}
	v, err := build(root)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	serialize(&buf, v)
	return buf.Bytes(), nil
}

// DigestWithCanonical canonicalizes once and returns both the "sha256:<hex>"
// digest and the canonical JSON bytes, so a caller that needs both (the publish
// hook stores the canonical document and the digest together) does not parse
// twice.
func DigestWithCanonical(doc []byte) (digest string, canonical []byte, err error) {
	canonical, err = Canonicalize(doc)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), canonical, nil
}

// SelfCheck verifies canonicalization is idempotent for doc: it canonicalizes
// doc, re-canonicalizes that output, and returns a non-nil error if the two
// digests differ or either step fails. The service runs SelfCheck at publish
// before freezing a version. A nil return means the digest is stable.
func SelfCheck(doc []byte) error {
	first, err := Canonicalize(doc)
	if err != nil {
		return err
	}
	second, err := Canonicalize(first)
	if err != nil {
		return fmt.Errorf("registrydigest: re-canonicalization failed: %w", err)
	}
	if !bytes.Equal(first, second) {
		return errors.New("registrydigest: canonicalization is not idempotent")
	}
	return nil
}

// parse decodes the input as a single YAML 1.2 document and returns its root
// content node. It rejects empty input and multi-document streams.
func parse(doc []byte) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(doc))

	var root yaml.Node
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyDocument
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidYAML, err.Error())
	}

	// A second successful decode means a multi-document stream.
	var second yaml.Node
	if err := dec.Decode(&second); err == nil {
		return nil, ErrMultiDocument
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidYAML, err.Error())
	}

	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, ErrEmptyDocument
		}
		return root.Content[0], nil
	}
	return &root, nil
}

// ---- generic value tree ----

type jNull struct{}

type jString string

// jNumber holds an already-canonical RFC 8785 number literal.
type jNumber string

type jMember struct {
	key string
	val any
}

type jObject []jMember

type jArray []any

// build converts a validated yaml.Node tree into the generic value tree,
// rejecting anchors/aliases, explicit tags, duplicate/non-string keys, and
// JCS-unsafe numbers along the way.
func build(n *yaml.Node) (any, error) {
	if n.Anchor != "" {
		return nil, ErrAnchorsUnsupported
	}
	switch n.Kind {
	case yaml.AliasNode:
		return nil, ErrAnchorsUnsupported
	case yaml.MappingNode:
		return buildMapping(n)
	case yaml.SequenceNode:
		return buildSequence(n)
	case yaml.ScalarNode:
		return buildScalar(n)
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, ErrEmptyDocument
		}
		return build(n.Content[0])
	default:
		return nil, ErrInvalidYAML
	}
}

func buildMapping(n *yaml.Node) (any, error) {
	if n.Tag != "" && n.Tag != "!!map" {
		return nil, ErrExplicitTag
	}
	obj := make(jObject, 0, len(n.Content)/2)
	seen := make(map[string]struct{}, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]

		if k.Anchor != "" || k.Kind == yaml.AliasNode {
			return nil, ErrAnchorsUnsupported
		}
		if k.Tag == "!!merge" || (k.Kind == yaml.ScalarNode && k.Value == "<<") {
			return nil, ErrAnchorsUnsupported
		}
		if k.Kind != yaml.ScalarNode {
			return nil, ErrNonStringKey
		}
		key := k.Value
		if _, dup := seen[key]; dup {
			return nil, ErrDuplicateKey
		}
		seen[key] = struct{}{}

		child, err := build(v)
		if err != nil {
			return nil, err
		}
		obj = append(obj, jMember{key: key, val: child})
	}
	return obj, nil
}

func buildSequence(n *yaml.Node) (any, error) {
	if n.Tag != "" && n.Tag != "!!seq" {
		return nil, ErrExplicitTag
	}
	arr := make(jArray, 0, len(n.Content))
	for _, c := range n.Content {
		child, err := build(c)
		if err != nil {
			return nil, err
		}
		arr = append(arr, child)
	}
	return arr, nil
}

func buildScalar(n *yaml.Node) (any, error) {
	switch n.Tag {
	case "!!str", "!", "":
		return jString(n.Value), nil
	case "!!null":
		return jNull{}, nil
	case "!!bool":
		return strings.EqualFold(n.Value, "true"), nil
	case "!!int", "!!float":
		num, err := canonicalNumber(n.Value)
		if err != nil {
			return nil, err
		}
		return num, nil
	case "!!merge":
		return nil, ErrAnchorsUnsupported
	default:
		return nil, ErrExplicitTag
	}
}

// canonicalNumber validates a numeric scalar as JCS-safe and returns its
// RFC 8785 canonical form.
func canonicalNumber(raw string) (jNumber, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", ErrUnsafeNumber
	}
	if f == math.Trunc(f) {
		if math.Abs(f) > maxSafeInteger {
			return "", ErrUnsafeNumber
		}
	} else if significantDigits(raw) > 17 {
		// float64 round-trips at most 17 significant decimal digits.
		return "", ErrUnsafeNumber
	}
	return jNumber(formatES(f)), nil
}

// significantDigits counts the significant decimal digits of a numeric literal.
func significantDigits(s string) int {
	s = strings.TrimLeft(s, "+-")
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		s = s[:i]
	}
	s = strings.Replace(s, ".", "", 1)
	s = strings.TrimLeft(s, "0")
	s = strings.TrimRight(s, "0")
	return len(s)
}

// formatES renders f per the ECMAScript Number::toString algorithm that RFC 8785
// mandates. f must already be finite and JCS-safe.
func formatES(f float64) string {
	if f == 0 {
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	i := strings.IndexByte(s, 'e')
	mant, exp := s[:i], s[i+1:]
	sign := exp[:1]
	digits := strings.TrimLeft(exp[1:], "0")
	if digits == "" {
		digits = "0"
	}
	return mant + "e" + sign + digits
}

// ---- RFC 8785 serialization ----

func serialize(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case jNull:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case jString:
		writeJSONString(buf, string(t))
	case jNumber:
		buf.WriteString(string(t))
	case jArray:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			serialize(buf, e)
		}
		buf.WriteByte(']')
	case jObject:
		sort.SliceStable(t, func(i, j int) bool {
			return utf16Less(t[i].key, t[j].key)
		})
		buf.WriteByte('{')
		for i, m := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(buf, m.key)
			buf.WriteByte(':')
			serialize(buf, m.val)
		}
		buf.WriteByte('}')
	}
}

// utf16Less reports whether a sorts before b by UTF-16 code unit, per RFC 8785.
func utf16Less(a, b string) bool {
	au := utf16.Encode([]rune(a))
	bu := utf16.Encode([]rune(b))
	n := len(au)
	if len(bu) < n {
		n = len(bu)
	}
	for i := 0; i < n; i++ {
		if au[i] != bu[i] {
			return au[i] < bu[i]
		}
	}
	return len(au) < len(bu)
}

const hexDigits = "0123456789abcdef"

// writeJSONString writes s with the minimal RFC 8785 string escaping.
func writeJSONString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\t':
			buf.WriteString(`\t`)
		case '\n':
			buf.WriteString(`\n`)
		case '\f':
			buf.WriteString(`\f`)
		case '\r':
			buf.WriteString(`\r`)
		default:
			if r < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexDigits[(r>>4)&0xF])
				buf.WriteByte(hexDigits[r&0xF])
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}
