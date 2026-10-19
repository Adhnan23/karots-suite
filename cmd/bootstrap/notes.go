package main

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// changesSince lists the fix/feat commit titles from `since` to HEAD as
// owner-readable lines. Commits scoped to a plugin this shop doesn't run are
// left out. Empty when since is unknown (first release, or history rewritten).
func changesSince(since string, shopPlugins, allPlugins []string) string {
	if since == "" {
		return ""
	}
	out, err := exec.Command("git", "log", "--no-merges", "--format=%s", since+"..HEAD").Output()
	if err != nil {
		return ""
	}
	return friendlyNotes(strings.Split(strings.TrimSpace(string(out)), "\n"), shopPlugins, allPlugins)
}

// friendlyNotes turns "fix(backup): restore works on installed shops — it set …"
// into "• Restore works on installed shops". Only fix/feat commits are kept;
// docs, tests, refactors and chores mean nothing to a shop owner.
func friendlyNotes(subjects, shopPlugins, allPlugins []string) string {
	const maxLines = 15
	var lines []string
	for _, s := range subjects {
		head, text, ok := strings.Cut(s, ": ")
		if !ok {
			continue
		}
		typ, scope, _ := strings.Cut(strings.TrimSuffix(head, "!"), "(")
		scope = strings.TrimSuffix(scope, ")")
		if typ != "fix" && typ != "feat" {
			continue
		}
		if slices.Contains(allPlugins, scope) && !slices.Contains(shopPlugins, scope) {
			continue
		}
		for _, sep := range []string{" — ", "; "} {
			text, _, _ = strings.Cut(text, sep)
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		lines = append(lines, "• "+strings.ToUpper(text[:1])+text[1:])
	}
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("…and %d more fixes", len(lines)-maxLines))
	}
	return strings.Join(lines, "\n")
}
