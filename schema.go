package pluginsdk

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

// SchemaFor returns a JSON Schema (draft 2020-12) for the Go type T, for use
// as Kind.Schema. The host validates every spec of the kind against it, so
// deriving it from the struct the plugin decodes keeps the two in step.
//
// Rules:
//   - Property names come from json tags.
//   - Fields with omitempty or omitzero are optional; all others are required.
//   - Structs reject unknown properties (additionalProperties false).
//   - The jsonschema struct tag is plain description text. A tag that starts
//     with "WORD=" is rejected.
//   - Slices are typed ["null","array"], so null passes for a slice field.
//   - A json.RawMessage field accepts any value. A []byte field is a string
//     or null, as encoding/json writes it in base64.
//
// SchemaFor panics if T cannot be expressed as a schema (for example a func
// field or a rejected jsonschema tag). Call it at init so such a mistake
// shows up at once.
func SchemaFor[T any]() []byte {
	s, err := jsonschema.For[T](schemaOptions)
	if err != nil {
		panic(fmt.Sprintf("pluginsdk.SchemaFor[%s]: %v", reflect.TypeFor[T](), err))
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("pluginsdk.SchemaFor[%s]: %v", reflect.TypeFor[T](), err))
	}
	return b
}

// schemaOptions fix types the default inference gets wrong: it reads
// json.RawMessage and []byte as arrays of integers.
var schemaOptions = &jsonschema.ForOptions{
	TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[json.RawMessage](): {},
		reflect.TypeFor[[]byte]():          {Types: []string{"null", "string"}},
	},
}
