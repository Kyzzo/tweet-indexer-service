package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func ToJSON(v interface{}, beautify bool) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if beautify {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(v); err != nil {
		return "", fmt.Errorf("failed to encode to JSON: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}
