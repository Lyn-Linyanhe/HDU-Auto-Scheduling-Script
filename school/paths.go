package school

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const outputDirectoryEnv = "HDU_OUTPUT_DIR"

// OutputFilePath resolves a data filename independently from the process cwd.
// HDU_OUTPUT_DIR is the explicit override; packaged binaries use their own
// directory, while source/test binaries keep the working-directory behavior.
func OutputFilePath(name string) (string, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" || filepath.Base(cleanName) != cleanName || cleanName == "." || cleanName == ".." {
		return "", fmt.Errorf("invalid output filename %q", name)
	}
	directory, err := outputDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, cleanName), nil
}

// EnsureOutputFilePath resolves a data filename and creates its parent
// directory before a write operation.
func EnsureOutputFilePath(name string) (string, error) {
	path, err := OutputFilePath(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	return path, nil
}

func outputDirectory() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return os.Getwd()
	}
	return outputDirectoryForExecutable(executable)
}

// outputDirectoryForExecutable resolves the output directory for a given
// executable path. It is separated so tests can cover packaged exe, temp exe
// and HDU_OUTPUT_DIR without depending on the real os.Executable result.
func outputDirectoryForExecutable(executable string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv(outputDirectoryEnv)); configured != "" {
		return filepath.Abs(configured)
	}
	if executable != "" && !isTemporaryExecutable(executable) {
		return filepath.Abs(filepath.Dir(executable))
	}
	return os.Getwd()
}

func isTemporaryExecutable(executable string) bool {
	name := strings.ToLower(filepath.Base(executable))
	if strings.Contains(name, ".test") {
		return true
	}
	temporaryRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	executableDir, err := filepath.Abs(filepath.Dir(executable))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(temporaryRoot, executableDir)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
