package job

import (
	"fmt"
	"strings"
)

// Command represents a parsed workflow command from step output.
// GitHub Actions uses ::command:: syntax in stdout for side effects.
type Command struct {
	// Name is the command name (e.g., "set-output", "add-mask", "set-env").
	Name string
	// Properties contains key=value parameters for the command.
	Properties map[string]string
	// Value is the command value (text after ::).
	Value string
}

// ParseCommand parses a workflow command from a log line.
// Format: ::command-name property1=value1,property2=value2::value
// Returns nil if the line is not a command.
func ParseCommand(line string) *Command {
	if !strings.HasPrefix(line, "::") {
		return nil
	}

	// Find the closing :: after the command name and properties.
	rest := line[2:]
	closingIdx := strings.Index(rest, "::")
	if closingIdx == -1 {
		return nil
	}

	header := rest[:closingIdx]
	value := rest[closingIdx+2:]

	// Split header into command name and properties.
	name := header
	properties := make(map[string]string)

	if spaceIdx := strings.Index(header, " "); spaceIdx != -1 {
		name = header[:spaceIdx]
		propsStr := header[spaceIdx+1:]
		for _, prop := range strings.Split(propsStr, ",") {
			kv := strings.SplitN(prop, "=", 2)
			if len(kv) == 2 {
				properties[kv[0]] = kv[1]
			}
		}
	}

	return &Command{
		Name:       name,
		Properties: properties,
		Value:      value,
	}
}

// KnownCommands lists the supported workflow commands.
var KnownCommands = map[string]string{
	"set-output": "Sets a step output parameter",
	"set-env":    "Sets an environment variable for subsequent steps",
	"add-path":   "Prepends a directory to PATH for subsequent steps",
	"add-mask":   "Masks a value in log output",
	"debug":      "Writes a debug message to the log",
	"warning":    "Creates a warning annotation",
	"error":      "Creates an error annotation",
	"notice":     "Creates a notice annotation",
	"group":      "Creates an expandable log group",
	"endgroup":   "Ends the current log group",
	"stop-commands": "Stops processing workflow commands",
}

// ValidateCommand checks if a command is known and has valid properties.
func ValidateCommand(cmd *Command) error {
	if cmd == nil {
		return fmt.Errorf("nil command")
	}
	if _, ok := KnownCommands[cmd.Name]; !ok {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}

	switch cmd.Name {
	case "set-output":
		if _, ok := cmd.Properties["name"]; !ok {
			return fmt.Errorf("set-output requires 'name' property")
		}
	case "set-env":
		if _, ok := cmd.Properties["name"]; !ok {
			return fmt.Errorf("set-env requires 'name' property")
		}
	}

	return nil
}
