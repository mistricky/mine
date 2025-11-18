package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var (
	infoColor    = color.New(color.FgBlue)
	errorColor   = color.New(color.FgRed)
	successColor = color.New(color.FgGreen)
)

// Info prints informational messages in blue to stdout.
func Info(format string, args ...any) {
	log(os.Stdout, infoColor, "INFO", format, args...)
}

// Error prints error messages in red to stderr.
func Error(format string, args ...any) {
	log(os.Stderr, errorColor, "ERROR", format, args...)
}

// Warning prints warning messages in the default style to stderr.
func Warning(format string, args ...any) {
	log(os.Stderr, nil, "WARNING", format, args...)
}

// Success prints success messages in green to stdout.
func Success(format string, args ...any) {
	log(os.Stdout, successColor, "SUCCESS", format, args...)
}

// Default prints neutral messages in the default style to stdout.
func Default(format string, args ...any) {
	log(os.Stdout, nil, "", format, args...)
}

func log(w io.Writer, clr *color.Color, prefix string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if prefix != "" {
		message = fmt.Sprintf("[%s] %s", prefix, message)
	}

	if clr != nil {
		clr.Fprint(w, message)
		return
	}
	fmt.Fprint(w, message)
}
