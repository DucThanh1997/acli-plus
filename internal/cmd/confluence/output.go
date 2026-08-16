package confluence

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"acli-plus/internal/app"
	confluence "acli-plus/internal/domain/confluence"
)

// printResult reports one page write. Warnings go to stderr so stdout stays a
// clean result line for callers that pipe it.
func printResult(res app.Result, host string) {
	for _, warning := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	prefix := ""
	if res.DryRun {
		prefix = "[dry-run] "
	}
	switch res.Action {
	case app.ActionCreate:
		fmt.Printf("%screated %q%s\n", prefix, res.Page.Title, pageLink(host, res.Page))
	case app.ActionUpdate:
		fmt.Printf("%supdated %q%s\n", prefix, res.Page.Title, pageLink(host, res.Page))
	case app.ActionDelete:
		fmt.Printf("%sdeleted (moved to trash) %q\n", prefix, res.Page.Title)
	case app.ActionAborted:
		fmt.Println("aborted; no changes made")
	}
}

// printPage renders the detail view: a header block of one-line fields, then
// the stored body. The body is Confluence storage format (XHTML) as the API
// returned it — there is no reverse renderer yet, so it is shown verbatim
// rather than flattened the way the Jira side flattens ADF.
func printPage(page confluence.Page, host string) {
	version := ""
	if page.Version.Number > 0 {
		version = strconv.Itoa(page.Version.Number)
		if page.Version.Message != "" {
			version += " (" + page.Version.Message + ")"
		}
	}
	pairs := [][2]string{
		{"Id", page.ID},
		{"Title", page.Title},
		{"Space id", page.SpaceID},
		{"Parent id", page.ParentID},
		{"Version", version},
		{"URL", strings.TrimPrefix(pageLink(host, page), " -> ")},
	}
	for _, pair := range pairs {
		if pair[1] != "" {
			fmt.Printf("%-11s %s\n", pair[0]+":", pair[1])
		}
	}
	if page.Body != "" {
		fmt.Printf("\n%s\n", page.Body)
	}
}

// printJSON emits the page as indented JSON for scripts to consume.
func printJSON(value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func pageLink(host string, page confluence.Page) string {
	if host == "" || page.ID == "" {
		return ""
	}
	return fmt.Sprintf(" -> https://%s/wiki/pages/viewpage.action?pageId=%s", host, page.ID)
}
