package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/fatih/color"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

var (
	InfoColor  = color.New(color.FgGreen).SprintFunc()
	ErrorColor = color.New(color.FgRed).SprintFunc()
)

var logDir = "log_files"

var ansiColorRegex = regexp.MustCompile("\x1b\\[[0-9;]*m")

// File logging keeps no permanent file handles: every line is appended inside
// a short-lived open/append/close so importing this module never leaks an
// open handle (or a log_files directory) into the working directory, and
// tests that chdir into t.TempDir() can be cleaned up on Windows.
var (
	ensureDirOnce sync.Once
	writeMutex    sync.Mutex
)

func init() {
	infoLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	errorLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
}

func ensureLogDir() {
	ensureDirOnce.Do(func() {
		_ = os.MkdirAll(logDir, 0755)
	})
}

func appendLogLine(fileName string, line string) {
	ensureLogDir()
	writeMutex.Lock()
	defer writeMutex.Unlock()
	file, err := os.OpenFile(filepath.Join(logDir, fileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line + "\n")
}

func removeAnsiColors(value string) string {
	return ansiColorRegex.ReplaceAllString(value, "")
}

func Info(v ...interface{}) {
	infoLogger.Println(append([]interface{}{InfoColor("[INFO]")}, v...)...)
	appendLogLine("app.log", "[INFO] "+removeAnsiColors(fmtArgs(v)))
}

func Error(v ...interface{}) {
	errorLogger.Println(append([]interface{}{ErrorColor("[ERROR]")}, v...)...)
	appendLogLine("app.log", "[ERROR] "+removeAnsiColors(fmtArgs(v)))
}

func Debug(v ...interface{}) {
	appendLogLine("debug.log", "[DEBUG] "+fmtArgs(v))
}

func fmtArgs(args []interface{}) string {
	// Mirrors log.Println's argument rendering (space separated).
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprint(arg)
	}
	return out
}
