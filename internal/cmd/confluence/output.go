package confluence

import (
	"fmt"
	"os"

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

func pageLink(host string, page confluence.Page) string {
	if host == "" || page.ID == "" {
		return ""
	}
	return fmt.Sprintf(" -> https://%s/wiki/pages/viewpage.action?pageId=%s", host, page.ID)
}
