package cli

import (
	"encoding/json"
	"os"
)

func outputJSONValue(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func resolveFormat(localFormat string) string {
	if jsonOut {
		return "json"
	}
	return localFormat
}
