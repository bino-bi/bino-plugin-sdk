// Package registryattest defines the in-toto Statement that describes a signed
// bino Registry release. The CLI builds it before signing and reads it back when
// it verifies; the registry reads it at publish. Both sides use this one
// package, so they cannot disagree on the format and fail a valid package.
//
// The statement is the DSSE payload of a Sigstore bundle. This package only
// builds and checks the payload bytes. Signing and verifying the bundle is the
// caller's job (sigstore-go).
//
// A statement has exactly one subject. Its name is "@scope/name@version" and
// its digest is the manifest digest from registrydigest, written as
// {"sha256": "<hex>"}. The predicate is SLSA Provenance v1 with the build type
// BuildType; its externalParameters hold the PublishChannel record.
//
// Scope is always passed without the leading "@". Scope, name and version must
// not be empty and must not contain "@" or "/".
package registryattest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// StatementType is the in-toto Statement v1 "_type".
	StatementType = "https://in-toto.io/Statement/v1"

	// PayloadType is the DSSE payload type of the statement.
	PayloadType = "application/vnd.in-toto+json"

	// PredicateType is SLSA Provenance v1.
	PredicateType = "https://slsa.dev/provenance/v1"

	// BuildType identifies a bino Registry publish in buildDefinition.buildType.
	BuildType = "https://bino.bi/registry/publish/v1"
)

// Sentinel errors. Match with errors.Is.
var (
	// ErrInvalidSubject is returned when scope, name or version is empty or
	// contains "@" or "/".
	ErrInvalidSubject = errors.New("registryattest: invalid subject")

	// ErrMalformedDigest is returned when a digest is not "sha256:" followed by
	// 64 lowercase hex characters, or a subject digest map is not
	// {"sha256": "<64 lowercase hex>"}.
	ErrMalformedDigest = errors.New("registryattest: malformed digest")

	// ErrInvalidPredicate is returned when the predicate is not a JSON object.
	ErrInvalidPredicate = errors.New("registryattest: invalid predicate")

	// ErrMalformedStatement is returned when the payload is not a JSON statement.
	ErrMalformedStatement = errors.New("registryattest: malformed statement")

	// ErrStatementType is returned when "_type" is not StatementType.
	ErrStatementType = errors.New("registryattest: wrong statement type")

	// ErrPredicateType is returned when "predicateType" is not PredicateType.
	ErrPredicateType = errors.New("registryattest: wrong predicate type")

	// ErrBuildType is returned when buildDefinition.buildType is not BuildType.
	ErrBuildType = errors.New("registryattest: wrong build type")

	// ErrSubjectCount is returned when the statement does not have exactly one
	// subject.
	ErrSubjectCount = errors.New("registryattest: statement must have exactly one subject")

	// ErrSubjectName is returned when the subject names another package.
	ErrSubjectName = errors.New("registryattest: subject name mismatch")

	// ErrSubjectVersion is returned when the subject names another version of
	// the same package.
	ErrSubjectVersion = errors.New("registryattest: subject version mismatch")

	// ErrSubjectDigest is returned when the subject digest is not the manifest
	// digest.
	ErrSubjectDigest = errors.New("registryattest: subject digest mismatch")

	// ErrInvalidChannel is returned for a publish channel other than the
	// Channel constants.
	ErrInvalidChannel = errors.New("registryattest: invalid publish channel")

	// ErrInvalidBuilder is returned when the builder id is empty.
	ErrInvalidBuilder = errors.New("registryattest: empty builder id")
)

// Channel is the way a release was published.
type Channel string

// Publish channels.
const (
	ChannelUI   Channel = "ui"
	ChannelPAT  Channel = "pat"
	ChannelOIDC Channel = "oidc"
)

// PublishChannel records how a release was published. The fields after Channel
// are optional and describe the CI run for ChannelOIDC.
type PublishChannel struct {
	Channel    Channel `json:"channel"`
	Repository string  `json:"repository,omitempty"`
	Workflow   string  `json:"workflow,omitempty"`
	Ref        string  `json:"ref,omitempty"`
	Commit     string  `json:"commit,omitempty"`
	RunURL     string  `json:"runUrl,omitempty"`
}

func (c PublishChannel) validate() error {
	switch c.Channel {
	case ChannelUI, ChannelPAT, ChannelOIDC:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidChannel, c.Channel)
	}
}

// Field order is the JSON key order, so the bytes are stable.
type statement struct {
	Type          string          `json:"_type"`
	Subject       []subject       `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

type subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type provenance struct {
	BuildDefinition buildDefinition `json:"buildDefinition"`
	RunDetails      runDetails      `json:"runDetails"`
}

type buildDefinition struct {
	BuildType          string         `json:"buildType"`
	ExternalParameters PublishChannel `json:"externalParameters"`
}

type runDetails struct {
	Builder builder `json:"builder"`
}

type builder struct {
	ID string `json:"id"`
}

// SubjectName returns the subject name "@scope/name@version". scope is passed
// without the leading "@".
func SubjectName(scope, name, version string) string {
	return "@" + scope + "/" + name + "@" + version
}

// NewPredicate returns the SLSA Provenance v1 predicate for a publish through
// channel, built by builderID (runDetails.builder.id).
func NewPredicate(channel PublishChannel, builderID string) (json.RawMessage, error) {
	if err := channel.validate(); err != nil {
		return nil, err
	}
	if builderID == "" {
		return nil, ErrInvalidBuilder
	}
	return marshal(provenance{
		BuildDefinition: buildDefinition{BuildType: BuildType, ExternalParameters: channel},
		RunDetails:      runDetails{Builder: builder{ID: builderID}},
	})
}

// NewStatement returns the in-toto Statement v1 for one release. manifestDigest
// has the form "sha256:<64 lowercase hex>", as registrydigest.Digest returns
// it. predicate must be a JSON object, normally from NewPredicate.
func NewStatement(scope, name, version, manifestDigest string, predicate json.RawMessage) ([]byte, error) {
	if err := checkSubject(scope, name, version); err != nil {
		return nil, err
	}
	hex, err := digestHex(manifestDigest)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(predicate)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, ErrInvalidPredicate
	}
	return marshal(statement{
		Type:          StatementType,
		Subject:       []subject{{Name: SubjectName(scope, name, version), Digest: map[string]string{"sha256": hex}}},
		PredicateType: PredicateType,
		Predicate:     trimmed,
	})
}

// CheckStatement checks that payload is a statement for exactly this release:
// the right statement and predicate types, one subject, and the subject name
// and sha256 digest equal to the given ones. It does not check the predicate
// body; use ReadPublishChannel for that.
func CheckStatement(payload []byte, scope, name, version, manifestDigest string) error {
	if err := checkSubject(scope, name, version); err != nil {
		return err
	}
	want, err := digestHex(manifestDigest)
	if err != nil {
		return err
	}
	st, err := parseStatement(payload)
	if err != nil {
		return err
	}
	if len(st.Subject) != 1 {
		return fmt.Errorf("%w: got %d", ErrSubjectCount, len(st.Subject))
	}
	sub := st.Subject[0]

	pkg := "@" + scope + "/" + name
	gotPkg, gotVersion := cutLast(sub.Name, "@")
	if gotPkg != pkg {
		return fmt.Errorf("%w: got %q, want %q", ErrSubjectName, sub.Name, SubjectName(scope, name, version))
	}
	if gotVersion != version {
		return fmt.Errorf("%w: got %q, want %q", ErrSubjectVersion, gotVersion, version)
	}

	got, ok := sub.Digest["sha256"]
	if len(sub.Digest) != 1 || !ok || !isHex64(got) {
		return fmt.Errorf("%w: subject digest %v", ErrMalformedDigest, sub.Digest)
	}
	if got != want {
		return fmt.Errorf("%w: got sha256:%s, want %s", ErrSubjectDigest, got, manifestDigest)
	}
	return nil
}

// ReadPublishChannel returns the publish channel record from a statement's
// predicate. It checks the statement, predicate and build types, but not the
// subject; call CheckStatement for that.
func ReadPublishChannel(payload []byte) (PublishChannel, error) {
	st, err := parseStatement(payload)
	if err != nil {
		return PublishChannel{}, err
	}
	var p provenance
	if err := json.Unmarshal(st.Predicate, &p); err != nil {
		return PublishChannel{}, fmt.Errorf("%w: %s", ErrInvalidPredicate, err.Error())
	}
	if p.BuildDefinition.BuildType != BuildType {
		return PublishChannel{}, fmt.Errorf("%w: %q", ErrBuildType, p.BuildDefinition.BuildType)
	}
	ch := p.BuildDefinition.ExternalParameters
	if err := ch.validate(); err != nil {
		return PublishChannel{}, err
	}
	return ch, nil
}

func parseStatement(payload []byte) (statement, error) {
	var st statement
	if err := json.Unmarshal(payload, &st); err != nil {
		return statement{}, fmt.Errorf("%w: %s", ErrMalformedStatement, err.Error())
	}
	if st.Type != StatementType {
		return statement{}, fmt.Errorf("%w: %q", ErrStatementType, st.Type)
	}
	if st.PredicateType != PredicateType {
		return statement{}, fmt.Errorf("%w: %q", ErrPredicateType, st.PredicateType)
	}
	return st, nil
}

func checkSubject(scope, name, version string) error {
	for _, part := range []string{scope, name, version} {
		if part == "" || strings.ContainsAny(part, "@/") {
			return fmt.Errorf("%w: %q", ErrInvalidSubject, SubjectName(scope, name, version))
		}
	}
	return nil
}

func digestHex(d string) (string, error) {
	hex, ok := strings.CutPrefix(d, "sha256:")
	if !ok || !isHex64(hex) {
		return "", fmt.Errorf("%w: %q", ErrMalformedDigest, d)
	}
	return hex, nil
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func cutLast(s, sep string) (before, after string) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+len(sep):]
}

// marshal encodes without HTML escaping, so "&", "<" and ">" in a field stay
// readable and the bytes match what other JSON encoders emit.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
