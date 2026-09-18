package agent

import "testing"

func TestExtractCode_WithLanguageTag(t *testing.T) {
	text := "Here's the code:\n```js\nprint(1)\n```\nDone."
	code, ok := ExtractCode(text)
	if !ok {
		t.Fatal("ExtractCode() ok = false, want true")
	}
	if code != "print(1)" {
		t.Errorf("ExtractCode() = %q, want %q", code, "print(1)")
	}
}

func TestExtractCode_WithoutLanguageTag(t *testing.T) {
	text := "```\nprint(2)\n```"
	code, ok := ExtractCode(text)
	if !ok {
		t.Fatal("ExtractCode() ok = false, want true")
	}
	if code != "print(2)" {
		t.Errorf("ExtractCode() = %q, want %q", code, "print(2)")
	}
}

func TestExtractCode_Absent(t *testing.T) {
	text := "The answer is 42, no code needed."
	_, ok := ExtractCode(text)
	if ok {
		t.Fatal("ExtractCode() ok = true, want false for text with no code block")
	}
}

func TestExtractCode_MultipleBlocksReturnsFirst(t *testing.T) {
	text := "```js\nvar a = 1;\n```\nsome text\n```js\nvar b = 2;\n```"
	code, ok := ExtractCode(text)
	if !ok {
		t.Fatal("ExtractCode() ok = false, want true")
	}
	if code != "var a = 1;" {
		t.Errorf("ExtractCode() = %q, want first block %q", code, "var a = 1;")
	}
}

func TestExtractCode_MultilineBody(t *testing.T) {
	text := "```javascript\nvar rows = readCsv(\"a.csv\");\nprint(rows.length);\n```"
	code, ok := ExtractCode(text)
	if !ok {
		t.Fatal("ExtractCode() ok = false, want true")
	}
	want := "var rows = readCsv(\"a.csv\");\nprint(rows.length);"
	if code != want {
		t.Errorf("ExtractCode() = %q, want %q", code, want)
	}
}
