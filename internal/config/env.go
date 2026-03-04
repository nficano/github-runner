package config

import (
	"log/slog"
	"os"
	"regexp"
)

// envVarPattern matches environment variable references in the form ${VAR_NAME}.
// It supports alphanumeric characters and underscores in variable names.
var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// interpolateEnv replaces all ${VAR_NAME} patterns in s with the
// corresponding environment variable value. If a referenced variable
// is not set, it is replaced with an empty string and a warning is
// logged. Nested references (a variable whose value contains ${...})
// are resolved by performing up to 10 expansion passes.
func interpolateEnv(s string) string {
	const maxPasses = 10

	for range maxPasses {
		replaced := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
			// Extract the variable name from the match (strip ${ and }).
			sub := envVarPattern.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			name := sub[1]
			val, ok := os.LookupEnv(name)
			if !ok {
				slog.Warn("environment variable not set, using empty string",
					slog.String("variable", name),
				)
				return ""
			}
			return val
		})

		// If nothing changed this pass, we are done.
		if replaced == s {
			return replaced
		}
		s = replaced
	}

	return s
}
