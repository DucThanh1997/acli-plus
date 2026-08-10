package jira

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	jiradomain "acli-plus/internal/domain/jira"
)

// format selects how a read command prints its result. The default is an
// aligned table; --json emits the API payload for scripting, and --csv the
// same columns as the table for spreadsheets.
type format struct {
	json bool
	csv  bool
}

// register adds the output flags to a command. csvToo is set on the listing
// commands, where a spreadsheet dump makes sense.
func (f *format) register(cmd *cobra.Command, csvToo bool) {
	cmd.Flags().BoolVar(&f.json, "json", false, "print raw JSON instead of a table")
	if csvToo {
		cmd.Flags().BoolVar(&f.csv, "csv", false, "print CSV instead of a table")
	}
}

// rows is a rendered result set: one header plus the data rows, which the
// table and CSV writers share.
type rows struct {
	header []string
	data   [][]string
}

func newRows(header ...string) *rows { return &rows{header: header} }

func (r *rows) add(cells ...string) { r.data = append(r.data, cells) }

// render writes the result set in the selected format. jsonValue is what --json
// prints; it is passed separately because the JSON form is the untouched API
// payload rather than the table's columns.
func (r *rows) render(f format, jsonValue any) error {
	switch {
	case f.json:
		return printJSON(jsonValue)
	case f.csv:
		return r.writeCSV()
	default:
		r.writeTable()
		return nil
	}
}

func (r *rows) writeTable() {
	if len(r.data) == 0 {
		fmt.Fprintln(os.Stderr, "no results")
		return
	}
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(r.header, "\t"))
	for _, row := range r.data {
		fmt.Fprintln(writer, strings.Join(clip(row), "\t"))
	}
	writer.Flush()
}

func (r *rows) writeCSV() error {
	writer := csv.NewWriter(os.Stdout)
	if err := writer.Write(r.header); err != nil {
		return err
	}
	if err := writer.WriteAll(r.data); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

// maxCell keeps one long summary from pushing every other column off screen.
const maxCell = 80

func clip(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		cell = strings.ReplaceAll(cell, "\n", " ")
		if len(cell) > maxCell {
			cell = cell[:maxCell-1] + "…"
		}
		out[i] = cell
	}
	return out
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// printResult reports a write command's outcome on stdout, with warnings and
// the dry-run marker on stderr so piped output stays clean.
func printResult(res app.JiraResult, host string) {
	for _, warning := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	prefix := ""
	if res.DryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf("%s%s%s\n", prefix, res.Detail, resultLinks(res, host))
}

// resultLinks appends browse links for a single-item result; a bulk result
// already lists its keys in the detail line.
func resultLinks(res app.JiraResult, host string) string {
	if res.DryRun || len(res.Keys) != 1 || host == "" {
		return ""
	}
	link := jiradomain.BrowseURL(host, res.Keys[0])
	if link == "" {
		return ""
	}
	return " -> " + link
}

// rawOrItem prints the untouched API payload when it is available, so --json
// exposes custom fields the typed struct does not model.
func rawOrItem(item jiradomain.WorkItem) any {
	if len(item.Raw) > 0 {
		return item.Raw
	}
	return item
}

func rawOrItems(items []jiradomain.WorkItem) any {
	raws := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		if len(item.Raw) == 0 {
			return items
		}
		raws = append(raws, item.Raw)
	}
	return raws
}

func shortDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func dateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return ""
}

func itoa(value int) string { return strconv.Itoa(value) }

// humanSize renders a byte count the way a file listing would.
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}
