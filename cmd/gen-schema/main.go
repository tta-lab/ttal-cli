package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/tta-lab/ttal-cli/internal/config"
)

func main() {
	r := &jsonschema.Reflector{
		FieldNameTag:               "toml",
		RequiredFromJSONSchemaTags: true,
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

	fmt.Println(string(data))
}
