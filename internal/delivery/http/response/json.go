package response

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/labstack/echo/v5"
)

// JSONSerializer is an echo.JSONSerializer backed by encoding/json/v2.
//
// Semantics differ from encoding/json (v1):
//   - object member names match case-sensitively;
//   - duplicate object keys and invalid UTF-8 are rejected;
//   - nil slices and maps encode as [] and {} rather than null;
//   - omitempty follows the v2 rules; prefer omitzero for zero values;
//   - the encoded value is not followed by a newline.
type JSONSerializer struct{}

var _ echo.JSONSerializer = JSONSerializer{}

// NewJSONSerializer returns a serializer to assign to echo.Echo.JSONSerializer.
func NewJSONSerializer() JSONSerializer {
	return JSONSerializer{}
}

// Serialize writes target as JSON to the response, indented when indent is set.
func (JSONSerializer) Serialize(c *echo.Context, target any, indent string) error {
	if indent == "" {
		return json.MarshalWrite(c.Response(), target)
	}

	return json.MarshalWrite(c.Response(), target, jsontext.WithIndent(indent))
}

// Deserialize reads one JSON value from the request body into target.
func (JSONSerializer) Deserialize(c *echo.Context, target any) error {
	if err := json.UnmarshalRead(c.Request().Body, target); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}

	return nil
}
