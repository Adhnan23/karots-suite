package aicatalog

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// decodeReply reads a chatbot's reply: the outermost {…}, then, if that isn't valid
// JSON, the same text with stray quotes and line breaks in its strings repaired.
func decodeReply(raw string, out any) error {
	s, err := jsonObject(raw)
	if err != nil {
		return err
	}
	if json.Unmarshal([]byte(s), out) == nil {
		return nil
	}
	if json.Unmarshal([]byte(repairQuotes(s)), out) == nil {
		return nil
	}
	clean := cleanMarkdown(s)
	if json.Unmarshal([]byte(clean), out) == nil || json.Unmarshal([]byte(repairQuotes(clean)), out) == nil {
		return nil
	}
	return errNoAnswer
}

var markdownEscape = regexp.MustCompile(`\\([_\[\]*#~.!(){}+>|`+"`"+`-])`)

// cleanMarkdown undoes what a chat page does to JSON when its answer is selected and
// copied by hand: HTML spaces (&#x20;), a backslash ending each line, \_ and \[.
func cleanMarkdown(s string) string {
	s = html.UnescapeString(s)
	s = strings.NewReplacer("\\\r\n", "\n", "\\\n", "\n", "\u00a0", " ").Replace(s)
	return markdownEscape.ReplaceAllString(s, "$1")
}

// repairQuotes escapes what chatbots leave raw inside strings: an inch mark
// (12.7mm (1/2"); …) and a line break. A " ends a string only when what follows can
// follow a string in JSON. Valid JSON comes back unchanged.
func repairQuotes(s string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case !inString:
			inString = c == '"'
			b.WriteByte(c)
		case c == '\\' && i+1 < len(s):
			b.WriteByte(c)
			i++
			b.WriteByte(s[i])
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
		case c == '"' && !endsString(s[i+1:]):
			b.WriteString(`\"`)
		default:
			inString = c != '"'
			b.WriteByte(c)
		}
	}
	return b.String()
}

// endsString says whether the text after a " can follow a finished JSON string.
func endsString(rest string) bool {
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" {
		return true
	}
	switch rest[0] {
	case '}', ']', ':':
		return true
	case ',':
		next := strings.TrimLeft(rest[1:], " \t\r\n")
		if next == "" {
			return true
		}
		if strings.ContainsRune(`"{[-0123456789`, rune(next[0])) {
			return true
		}
		return strings.HasPrefix(next, "true") || strings.HasPrefix(next, "false") || strings.HasPrefix(next, "null")
	}
	return false
}
