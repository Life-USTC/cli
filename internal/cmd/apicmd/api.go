package apicmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

var knownAPIPaths = generatedAPIPaths

func NewCmdAPI() *cobra.Command {
	var (
		method    string
		headers   []string
		rawFields []string
		fields    []string
		inputFile string
		include   bool
	)

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Make a raw API request",
		Long: `Make a raw request against the Life@USTC API.

This is the escape hatch for endpoints that do not have a first-class command
yet. It follows the same design intent as 'gh api': keep the command model
clean, but always leave a low-level path open for scripting and exploration.`,
		Example: `  # Fetch the current semester
  life-ustc api semesters/current

  # Create a todo with form-like fields
  life-ustc api todos -F title='Write report' -F priority=high

  # Send an exact JSON body from a file
  life-ustc api -X POST todos --input ./todo.json

  # Script a response with jq
  life-ustc api sections --jq '.data[].code'`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAPIPaths,
		RunE: func(cmd *cobra.Command, args []string) error {
			if include && output.IsJSON() {
				return fmt.Errorf("--include cannot be combined with --json or --jq")
			}

			client, err := api.NewClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}

			if inputFile != "" && (len(fields) > 0 || len(rawFields) > 0) {
				return fmt.Errorf("--input cannot be combined with --field or --raw-field")
			}

			requestMethod := strings.ToUpper(strings.TrimSpace(method))
			if requestMethod == "" {
				if inputFile != "" || len(fields) > 0 || len(rawFields) > 0 {
					requestMethod = http.MethodPost
				} else {
					requestMethod = http.MethodGet
				}
			}

			params := url.Values{}
			var (
				body        io.Reader
				contentType string
			)

			if inputFile != "" {
				payload, err := readInputFile(inputFile)
				if err != nil {
					return err
				}
				body = bytes.NewReader(payload)
				contentType = "application/json"
			} else if len(fields) > 0 || len(rawFields) > 0 {
				values, err := buildFields(fields, rawFields)
				if err != nil {
					return err
				}
				if requestMethod == http.MethodGet || requestMethod == http.MethodDelete {
					for key, value := range values {
						params.Set(key, fmt.Sprint(value))
					}
				} else {
					payload, err := json.Marshal(values)
					if err != nil {
						return err
					}
					body = bytes.NewReader(payload)
					contentType = "application/json"
				}
			}

			extraHeaders, err := parseHeaders(headers)
			if err != nil {
				return err
			}
			if ct := extraHeaders.Get("Content-Type"); ct != "" {
				contentType = ct
				extraHeaders.Del("Content-Type")
			}

			resp, err := client.DoRaw(api.Ctx(), requestMethod, normalizeAPIPath(args[0]), params, body, contentType, extraHeaders)
			if err != nil {
				return err
			}
			if include {
				_, _ = fmt.Fprintf(output.Writer(), "HTTP %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
				for key, values := range resp.Header {
					_, _ = fmt.Fprintf(output.Writer(), "%s: %s\n", key, strings.Join(values, ", "))
				}
				_, _ = fmt.Fprintln(output.Writer())
			}

			bodyBytes, contentType, err := api.ReadResponse(resp, err)
			if err != nil {
				return err
			}

			if len(bodyBytes) == 0 {
				return nil
			}

			if api.IsJSONContentType(contentType) {
				decoded, err := api.DecodeResponseBody(bodyBytes, contentType, true)
				if err != nil {
					return err
				}
				return output.JSON(decoded)
			}

			_, err = fmt.Fprintln(output.Writer(), string(bodyBytes))
			return err
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", "", "HTTP method to use (default: GET, or POST when fields/input are provided)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Add an HTTP header in 'Key: Value' form")
	cmd.Flags().StringArrayVarP(&rawFields, "raw-field", "f", nil, "Add a string parameter in key=value form")
	cmd.Flags().StringArrayVarP(&fields, "field", "F", nil, "Add a typed parameter in key=value form")
	cmd.Flags().StringVar(&inputFile, "input", "", "Read the request body from a file ('-' for stdin)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Print response status and headers before the body")

	return cmd
}

func completeAPIPaths(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	absolute := strings.HasPrefix(toComplete, "/")
	needle := toComplete
	if !absolute && strings.HasPrefix(needle, "api/") {
		needle = "/" + needle
		absolute = true
	}

	for _, path := range knownAPIPaths {
		candidate := path
		if !absolute && strings.HasPrefix(path, "/api/") {
			candidate = strings.TrimPrefix(path, "/api/")
		}
		if strings.HasPrefix(candidate, toComplete) || strings.HasPrefix(path, needle) {
			completions = append(completions, candidate)
		}
	}
	if len(completions) == 0 {
		completions = cobra.AppendActiveHelp(completions, "Use a raw path like '/api/courses' or a shorthand like 'courses'.")
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func normalizeAPIPath(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	if strings.HasPrefix(path, "api/") {
		return "/" + path
	}
	return "/api/" + path
}

func buildFields(typedFields, rawFields []string) (map[string]any, error) {
	values := make(map[string]any, len(typedFields)+len(rawFields))
	for _, raw := range rawFields {
		key, value, err := splitField(raw)
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	for _, raw := range typedFields {
		key, value, err := splitField(raw)
		if err != nil {
			return nil, err
		}
		parsed, err := parseTypedValue(value)
		if err != nil {
			return nil, err
		}
		values[key] = parsed
	}
	return values, nil
}

func splitField(raw string) (string, string, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("invalid field %q: expected key=value", raw)
	}
	return strings.TrimSpace(key), value, nil
}

func parseTypedValue(raw string) (any, error) {
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}

	if strings.HasPrefix(raw, "@") {
		payload, err := readInputFile(strings.TrimPrefix(raw, "@"))
		if err != nil {
			return nil, err
		}
		return string(payload), nil
	}

	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, nil
	}
	return raw, nil
}

func readInputFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func parseHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, raw := range values {
		key, value, ok := strings.Cut(raw, ":")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid header %q: expected 'Key: Value'", raw)
		}
		headers.Add(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	return headers, nil
}
