package pluginsdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type testChartSpec struct {
	Dataset string   `json:"dataset"`
	Series  []string `json:"series,omitempty"`
}

func TestSchemaFor(t *testing.T) {
	raw := SchemaFor[testChartSpec]()

	var got struct {
		Type                 string   `json:"type"`
		Required             []string `json:"required"`
		AdditionalProperties any      `json:"additionalProperties"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("schema is not valid JSON: %v\n%s", err, raw)
	}
	if got.Type != "object" {
		t.Fatalf("got type %q, want object", got.Type)
	}
	if !slices.Equal(got.Required, []string{"dataset"}) {
		t.Fatalf("got required %v, want [dataset]", got.Required)
	}
	if got.AdditionalProperties != false {
		t.Fatalf("got additionalProperties %v, want false", got.AdditionalProperties)
	}

	checkInstances(t, raw, []schemaCase{
		{`{"dataset":"sales","series":["a","b"]}`, true},
		{`{"dataset":"sales","series":null}`, true},
		{`{"dataset":"sales"}`, true},
		{`{"series":["a"]}`, false},
		{`{"dataset":"sales","extra":1}`, false},
		{`{"dataset":1}`, false},
	})
}

type rawFieldsSpec struct {
	Options json.RawMessage `json:"options,omitempty"`
	Blob    []byte          `json:"blob,omitempty"`
}

func TestSchemaFor_RawMessageAndBytes(t *testing.T) {
	raw := SchemaFor[rawFieldsSpec]()

	blob, err := json.Marshal(rawFieldsSpec{Blob: []byte{0, 1, 255}})
	if err != nil {
		t.Fatal(err)
	}
	checkInstances(t, raw, []schemaCase{
		{`{"options":{"chart":{"type":"bar"}}}`, true},
		{`{"options":[1,"a"]}`, true},
		{`{"options":"x"}`, true},
		{string(blob), true},
		{`{"blob":null}`, true},
		{`{"blob":[1,2]}`, false},
	})
}

type schemaCase struct {
	instance string
	valid    bool
}

// checkInstances compiles raw with the validator the host uses and checks
// each instance against it.
func checkInstances(t *testing.T, raw []byte, cases []schemaCase) {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("spec.json", doc); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	sch, err := c.Compile("spec.json")
	if err != nil {
		t.Fatalf("compile schema: %v\n%s", err, raw)
	}

	for _, tt := range cases {
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(tt.instance))
		if err != nil {
			t.Fatalf("unmarshal instance %s: %v", tt.instance, err)
		}
		err = sch.Validate(inst)
		if tt.valid && err != nil {
			t.Errorf("%s: expected valid, got %v", tt.instance, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("%s: expected invalid", tt.instance)
		}
	}
}

type badTagSpec struct {
	Title string `json:"title" jsonschema:"description=x"`
}

func TestSchemaFor_PanicsOnBadTag(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic for a WORD= jsonschema tag")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "badTagSpec") {
			t.Fatalf("panic message does not name the type: %s", msg)
		}
	}()
	SchemaFor[badTagSpec]()
}
