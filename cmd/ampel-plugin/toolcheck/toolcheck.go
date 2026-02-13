package toolcheck

import (
	"fmt"
	"os/exec"
	"strings"
)

// RequiredTools lists the external tools the plugin depends on.
var RequiredTools = []string{"snappy", "ampel"}

// CheckTools verifies that all required tools are available on the system PATH.
// It returns a list of missing tool names.
func CheckTools() ([]string, error) {
	var missing []string
	for _, tool := range RequiredTools {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}
	return missing, nil
}

// FormatMissingToolsError constructs an error message listing each missing tool.
func FormatMissingToolsError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"required tools not found: %s. Ensure the following tools are installed and available on your PATH: %s. See AMPEL documentation for installation instructions",
		strings.Join(missing, ", "),
		strings.Join(missing, ", "),
	)
}
