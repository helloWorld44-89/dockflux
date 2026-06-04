package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/helloWorld44-89/dockflux/internal/lockfile"
	"github.com/pterm/pterm"
)

// Success prints a green success message.
func Success(format string, a ...any) {
	pterm.Success.Printfln(format, a...)
}

// Info prints a blue info message.
func Info(format string, a ...any) {
	pterm.Info.Printfln(format, a...)
}

// Warn prints a yellow warning message.
func Warn(format string, a ...any) {
	pterm.Warning.Printfln(format, a...)
}

// Error prints a red error message.
func Error(format string, a ...any) {
	pterm.Error.Printfln(format, a...)
}

// Spinner starts a spinner with the given text and returns a stop function.
func Spinner(text string) func(success bool, msg string) {
	sp, _ := pterm.DefaultSpinner.Start(text)
	return func(success bool, msg string) {
		if success {
			sp.Success(msg)
		} else {
			sp.Fail(msg)
		}
	}
}

// StatusTable renders the lockfile state as a table.
func StatusTable(lf *lockfile.LockFile) {
	if len(lf.Stacks) == 0 {
		Info("No stacks deployed yet.")
		return
	}

	rows := [][]string{
		{"STACK", "HOST", "COMMIT", "DEPLOYED", "STATUS"},
	}

	for stackName, state := range lf.Stacks {
		for hostName, entry := range state {
			age := formatAge(entry.DeployedAt)
			status := colorStatus(entry.Status)
			rows = append(rows, []string{stackName, hostName, entry.Commit, age, status})
		}
	}

	_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
}

// HostsTable renders the inventory host list as a table.
func HostsTable(hosts [][]string) {
	header := []string{"NAME", "TYPE", "HOST", "USER", "GROUPS"}
	rows := append([][]string{header}, hosts...)
	_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
}

// DiffTable renders the diff between lockfile state and current HEAD.
func DiffTable(rows []DiffRow) {
	if len(rows) == 0 {
		Success("Everything is up to date.")
		return
	}

	tableRows := [][]string{{"STACK", "HOST", "DEPLOYED COMMIT", "REPO HEAD", "STATE"}}
	for _, r := range rows {
		tableRows = append(tableRows, []string{r.Stack, r.Host, r.DeployedCommit, r.HeadCommit, r.State})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(tableRows).Render()
}

type DiffRow struct {
	Stack          string
	Host           string
	DeployedCommit string
	HeadCommit     string
	State          string // "stale" | "undeployed"
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func colorStatus(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return pterm.Green(status)
	case "stopped":
		return pterm.Yellow(status)
	case "failed":
		return pterm.Red(status)
	default:
		return status
	}
}
