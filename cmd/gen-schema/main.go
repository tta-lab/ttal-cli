package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/tta-lab/ttal-cli/internal/config"
)

func main() {
	// Use draft-07 for broader tool compatibility (taplo, tombi, etc.)
	jsonschema.Version = "http://json-schema.org/draft-07/schema#"

	r := &jsonschema.Reflector{
		FieldNameTag:               "toml",
		RequiredFromJSONSchemaTags: true,
		ExpandedStruct:             true,
	}

	schema := r.Reflect(&config.Config{})
	schema.ID = "https://ttal.guion.io/schema/config.schema.json"
	schema.Title = "ttal CLI configuration"
	schema.Description = "Configuration file for ttal CLI (~/.config/ttal/config.toml)"

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Convert 2020-12 $defs to draft-07 definitions for tool compatibility
	output := strings.ReplaceAll(string(data), `"$defs"`, `"definitions"`)
	output = strings.ReplaceAll(output, `#/$defs/`, `#/definitions/`)

	fmt.Println(output)
}
