package logger

import (
	"fmt"
	"strings"
	"time"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	blue   = "\033[34m"
	purple = "\033[35m"
	gray   = "\033[90m"
)

func Section(title string) {
	line := strings.Repeat("=", 60)
	fmt.Printf("\n%s%s%s\n", bold+cyan, line, reset)
	fmt.Printf("%s%s  %s  %s%s\n", bold+cyan, "║", title, "║", reset)
	fmt.Printf("%s%s%s\n\n", bold+cyan, line, reset)
}

func Lesson(title string) {
	fmt.Printf("\n%s━━━ %s %s━━━%s\n", bold+blue, title, bold+blue, reset)
}

func SQL(query string) {
	q := strings.TrimSpace(query)
	if len(q) > 500 {
		q = q[:500] + "..."
	}
	fmt.Printf("%s[SQL]%s %s%s%s\n", bold+yellow, reset, gray, q, reset)
}

func Result(label string, value any) {
	fmt.Printf("%s  ✓ %-28s%s %v\n", green, label+":", reset, value)
}

// Row prints a table row (key=value pairs).
func Row(data map[string]any) {
	fmt.Printf("%s  │%s", gray, reset)
	for k, v := range data {
		fmt.Printf(" %s%s%s=%v", bold, k, reset, v)
	}
	fmt.Println()
}

// Explain prints an explanation/tip box.
func Explain(text string) {
	lines := strings.Split(text, "\n")
	fmt.Printf("\n%s┌─ 💡 CONCEPT ─────────────────────────────────%s\n", purple, reset)
	for _, l := range lines {
		fmt.Printf("%s│%s  %s\n", purple, reset, l)
	}
	fmt.Printf("%s└───────────────────────────────────────────────%s\n\n", purple, reset)
}

// Error prints an error message.
func Error(err error) {
	fmt.Printf("%s  ✗ ERROR: %v%s\n", red, err, reset)
}

// Fatal prints an error and exits.
func Fatal(err error) {
	fmt.Printf("\n%s%s  FATAL: %v%s\n", bold+red, "✗ ", err, reset)
}

// Step prints a numbered step.
func Step(n int, msg string) {
	fmt.Printf("%s  [%d]%s %s\n", bold+cyan, n, reset, msg)
}

// Divider prints a thin dividing line.
func Divider() {
	fmt.Printf("%s%s%s\n", gray, strings.Repeat("─", 50), reset)
}

// Time prints a timing result.
func Time(label string, d time.Duration) {
	fmt.Printf("%s  ⏱  %-28s%s %v\n", yellow, label+":", reset, d)
}

// Header prints the main program header.
func Header(module, title string) {
	fmt.Printf("\n%s╔══════════════════════════════════════════════════╗%s\n", bold+cyan, reset)
	fmt.Printf("%s║%s  %-48s%s║%s\n", bold+cyan, bold+green, module, bold+cyan, reset)
	fmt.Printf("%s║%s  %-48s%s║%s\n", bold+cyan, bold, title, bold+cyan, reset)
	fmt.Printf("%s╚══════════════════════════════════════════════════╝%s\n\n", bold+cyan, reset)
}

// Success prints a green success message.
func Success(msg string) {
	fmt.Printf("%s  ✓ %s%s\n", green, msg, reset)
}

// Warning prints a yellow warning.
func Warning(msg string) {
	fmt.Printf("%s  ⚠  %s%s\n", yellow, msg, reset)
}

// Info prints a blue info line.
func Info(msg string) {
	fmt.Printf("%s  ℹ  %s%s\n", blue, msg, reset)
}

// TableHeader prints a formatted table header row.
func TableHeader(cols ...string) {
	fmt.Printf("%s  ", gray)
	for _, c := range cols {
		fmt.Printf("%-20s", c)
	}
	fmt.Printf("%s\n", reset)
	fmt.Printf("%s  %s%s\n", gray, strings.Repeat("-", 20*len(cols)), reset)
}

// TableRow prints a formatted table data row.
func TableRow(vals ...interface{}) {
	fmt.Printf("  ")
	for _, v := range vals {
		fmt.Printf("%-20v", v)
	}
	fmt.Println()
}
