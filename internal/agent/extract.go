package agent

import (
	"regexp"
	"strings"
)

// codeBlockRe matches a fenced code block with an optional language tag
// (```js, ```javascript, or bare ```).
var codeBlockRe = regexp.MustCompile("(?s)```[a-zA-Z0-9]*\\n?(.*?)```")

// ExtractCode returns the first fenced code block in text. If the model's
// reply contains no code block, ok is false and text is treated as its
// final natural-language answer.
func ExtractCode(text string) (code string, ok bool) {
	m := codeBlockRe.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}
