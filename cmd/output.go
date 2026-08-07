package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

const (
	outputText = "text"
	outputJSON = "json"
)

func commandOutputFormat(cmd *cobra.Command) (string, error) {
	value, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	return validateOutputFormat(value)
}

func validateOutputFormat(value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format == "" {
		format = outputText
	}
	if format != outputText && format != outputJSON {
		message := fmt.Sprintf("unsupported output format %q (expected text or json)", value)
		return "", newCLIError(ErrorInvalidInput, message, nil)
	}
	return format, nil
}

func writeJSON(out io.Writer, value interface{}) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to encode JSON output: %w", err)
	}
	return nil
}

func writeRawJSON(out io.Writer, body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return newInvalidResponseError(fmt.Errorf("server returned invalid JSON: %w", err))
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return newInvalidResponseError(errors.New("server returned multiple JSON values"))
		}
		return newInvalidResponseError(fmt.Errorf("server returned invalid JSON: %w", err))
	}
	return writeJSON(out, value)
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
