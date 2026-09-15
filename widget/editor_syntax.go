package widget

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SyntaxKind identifies the semantic role of a syntax token.
type SyntaxKind uint8

const (
	SyntaxText SyntaxKind = iota
	SyntaxKeyword
	SyntaxString
	SyntaxComment
	SyntaxNumber
	SyntaxType
	SyntaxPunctuation
	SyntaxFunction
	SyntaxConstant
	SyntaxBoolean
	SyntaxOperator
)

// SyntaxToken describes one highlighted range using rune-based columns.
type SyntaxToken struct {
	Line int
	Col  int
	Text string
	Kind SyntaxKind
}

// SyntaxHighlighter provides language-specific syntax presentation without
// coupling CodeEditor to a particular parser or IDE.
type SyntaxHighlighter interface {
	Highlight(documentName, language string, lines []string) []SyntaxToken
}

// RangeSyntaxHighlighter optionally supports incremental highlighting. The
// returned tokens must be limited to [startLine, endLine). Providers that
// cannot safely highlight a range should implement HighlightRange by falling
// back to Highlight and filtering the result.
type RangeSyntaxHighlighter interface {
	SyntaxHighlighter
	HighlightRange(documentName, language string, lines []string, startLine, endLine int) []SyntaxToken
}

// FoldRange describes a collapsible inclusive line range.
type FoldRange struct {
	Start int
	End   int
}

// FoldProvider discovers collapsible ranges independently of rendering.
type FoldProvider interface {
	Folds(documentName, language string, lines []string) []FoldRange
}

// DefaultSyntaxHighlighter uses the Go standard scanner for Go and a small
// language-agnostic tokenizer for other common text/code languages.
type DefaultSyntaxHighlighter struct{}

func (DefaultSyntaxHighlighter) Highlight(documentName, language string, lines []string) []SyntaxToken {
	if strings.EqualFold(language, "go") {
		return highlightGo(documentName, lines)
	}
	if profile, ok := languageProfileFor(language); ok {
		return highlightLanguageRange(lines, 0, len(lines), profile)
	}
	return highlightGeneric(lines)
}

func (h DefaultSyntaxHighlighter) HighlightRange(documentName, language string, lines []string, startLine, endLine int) []SyntaxToken {
	if strings.EqualFold(language, "go") {
		return highlightGoRange(documentName, lines, startLine, endLine)
	}
	if profile, ok := languageProfileFor(language); ok {
		return highlightLanguageRange(lines, startLine, endLine, profile)
	}
	return highlightGenericRange(lines, startLine, endLine)
}

// GoSyntaxHighlighter is an explicit reusable Go highlighter. It is useful to
// applications that want Go presentation but want to choose their own provider.
type GoSyntaxHighlighter struct{}

func (GoSyntaxHighlighter) Highlight(documentName, _ string, lines []string) []SyntaxToken {
	return highlightGo(documentName, lines)
}

func (GoSyntaxHighlighter) HighlightRange(documentName, language string, lines []string, startLine, endLine int) []SyntaxToken {
	return highlightGoRange(documentName, lines, startLine, endLine)
}

// GenericSyntaxHighlighter is the lightweight fallback for languages without a built-in profile.
type GenericSyntaxHighlighter struct{}

func (GenericSyntaxHighlighter) Highlight(_, _ string, lines []string) []SyntaxToken {
	return highlightGeneric(lines)
}

func (GenericSyntaxHighlighter) HighlightRange(_, _ string, lines []string, startLine, endLine int) []SyntaxToken {
	return highlightGenericRange(lines, startLine, endLine)
}

func isGoFunctionName(src []byte, offset int) bool {
	if offset < 0 || offset >= len(src) {
		return false
	}
	i := offset
	for i < len(src) && (src[i] == '_' || src[i] >= 'a' && src[i] <= 'z' || src[i] >= 'A' && src[i] <= 'Z' || src[i] >= '0' && src[i] <= '9') {
		i++
	}
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n') {
		i++
	}
	return i < len(src) && src[i] == '('
}

func isGoConstantName(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'
}

func isGoType(s string) bool {
	switch s {
	case "bool", "byte", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	}
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'
}

func runeColumn(s string, byteCol int) int {
	if byteCol <= 0 {
		return 0
	}
	if byteCol > len(s) {
		byteCol = len(s)
	}
	return utf8.RuneCountInString(s[:byteCol])
}

func highlightGoRange(_ string, lines []string, startLine, endLine int) []SyntaxToken {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if endLine <= startLine {
		return nil
	}
	// A local range is safe unless the changed span itself contains a
	// multiline delimiter. In that exceptional case retain full-document
	// scanning so lexical state remains correct across line boundaries.
	for _, line := range lines[startLine:endLine] {
		if strings.Contains(line, "/*") || strings.Contains(line, "*/") || strings.ContainsRune(line, '`') {
			return filterTokenLines(highlightGo("", lines), startLine, endLine)
		}
	}
	return highlightGoLines(lines, startLine, endLine)
}

func highlightGoLines(lines []string, startLine, endLine int) []SyntaxToken {
	totalBytes := 0
	for _, line := range lines[startLine:endLine] {
		totalBytes += len(line)
	}
	capHint := totalBytes/4 + 32
	if capHint < 32 {
		capHint = 32
	}
	tokens := make([]SyntaxToken, 0, capHint)
	for li := startLine; li < endLine; li++ {
		line := lines[li]
		for i := 0; i < len(line); {
			b := line[i]
			switch {
			case b == ' ' || b == '\t' || b == '\r':
				i++
			case b == '/' && i+1 < len(line) && line[i+1] == '/':
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, i), Text: line[i:], Kind: SyntaxComment})
				i = len(line)
			case b == '"' || b == '\'':
				start := i
				i++
				for i < len(line) {
					if line[i] == '\\' {
						i += 2
						continue
					}
					if line[i] == b {
						i++
						break
					}
					i++
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxString})
			case isGoNumberStart(line, i):
				start := i
				i = scanGoNumber(line, i)
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxNumber})
			case isGoIdentStart(line, i):
				start := i
				i = scanGoIdent(line, i)
				lit := line[start:i]
				kind := goIdentifierKind(lit)
				if kind == SyntaxText && isGoFunctionNameAt(line, i) {
					kind = SyntaxFunction
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: lit, Kind: kind})
			default:
				start := i
				i += goOperatorLen(line, i)
				kind := SyntaxPunctuation
				if isGoOperator(line[start:i]) {
					kind = SyntaxOperator
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: kind})
			}
		}
	}
	return tokens
}

func highlightGo(name string, lines []string) []SyntaxToken {
	tokens, _ := highlightGoAndFolds(name, lines, false)
	return tokens
}

// highlightGoAndFolds is a purpose-built Go lexer for presentation. The
// standard go/scanner package is excellent for parsing, but it builds a token
// file line table and allocates identifier literals for every token. Neither
// is necessary for editor coloring. Lexing the already-split document lines
// keeps cold-open memory proportional to the actual token list and lets folds
// be collected in the same pass.
func highlightGoAndFolds(_ string, lines []string, wantFolds bool) ([]SyntaxToken, []FoldRange) {
	totalBytes := 0
	for _, line := range lines {
		totalBytes += len(line)
	}
	capHint := totalBytes/4 + 64
	if capHint < 64 {
		capHint = 64
	}
	tokens := make([]SyntaxToken, 0, capHint)
	var folds []FoldRange
	var braceStack []int
	blockComment := false
	rawString := false

	for li, line := range lines {
		for i := 0; i < len(line); {
			if blockComment {
				start := i
				if j := strings.Index(line[i:], "*/"); j >= 0 {
					i += j + 2
					blockComment = false
				} else {
					i = len(line)
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxComment})
				continue
			}
			if rawString {
				start := i
				if j := strings.IndexByte(line[i:], '`'); j >= 0 {
					i += j + 1
					rawString = false
				} else {
					i = len(line)
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxString})
				continue
			}

			b := line[i]
			switch {
			case b == ' ' || b == '\t' || b == '\r':
				i++
				continue
			case b == '/' && i+1 < len(line) && line[i+1] == '/':
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, i), Text: line[i:], Kind: SyntaxComment})
				i = len(line)
				continue
			case b == '/' && i+1 < len(line) && line[i+1] == '*':
				start := i
				i += 2
				if j := strings.Index(line[i:], "*/"); j >= 0 {
					i += j + 2
				} else {
					i = len(line)
					blockComment = true
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxComment})
				continue
			case b == '`':
				start := i
				i++
				if j := strings.IndexByte(line[i:], '`'); j >= 0 {
					i += j + 1
				} else {
					i = len(line)
					rawString = true
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxString})
				continue
			case b == '"' || b == '\'':
				start := i
				i++
				for i < len(line) {
					if line[i] == '\\' {
						i += 2
						continue
					}
					if line[i] == b {
						i++
						break
					}
					i++
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxString})
				continue
			case isGoNumberStart(line, i):
				start := i
				i = scanGoNumber(line, i)
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: SyntaxNumber})
				continue
			case isGoIdentStart(line, i):
				start := i
				i = scanGoIdent(line, i)
				lit := line[start:i]
				kind := goIdentifierKind(lit)
				if kind == SyntaxText && isGoFunctionNameAt(line, i) {
					kind = SyntaxFunction
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: lit, Kind: kind})
				continue
			default:
				start := i
				i += goOperatorLen(line, i)
				kind := SyntaxPunctuation
				if isGoOperator(line[start:i]) {
					kind = SyntaxOperator
				}
				tokens = append(tokens, SyntaxToken{Line: li, Col: runeColumn(line, start), Text: line[start:i], Kind: kind})
				if wantFolds {
					switch line[start:i] {
					case "{":
						braceStack = append(braceStack, li)
					case "}":
						if n := len(braceStack); n > 0 {
							from := braceStack[n-1]
							braceStack = braceStack[:n-1]
							if li > from {
								folds = append(folds, FoldRange{Start: from, End: li})
							}
						}
					}
				}
			}
		}
	}
	return tokens, folds
}

func isGoIdentStart(s string, i int) bool {
	r, _ := utf8.DecodeRuneInString(s[i:])
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) && i > 0
}

func scanGoIdent(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		i += n
	}
	return i
}

func isGoNumberStart(s string, i int) bool {
	return s[i] >= '0' && s[i] <= '9' || s[i] == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'
}

func scanGoNumber(s string, i int) int {
	for i < len(s) {
		b := s[i]
		if b == '+' || b == '-' {
			if i == 0 || (s[i-1] != 'e' && s[i-1] != 'E' && s[i-1] != 'p' && s[i-1] != 'P') {
				break
			}
			i++
			continue
		}
		if b == '_' || b == '.' || b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' || b == 'x' || b == 'X' || b == 'o' || b == 'O' || b == 'b' || b == 'B' || b == 'e' || b == 'E' || b == 'i' || b == 'p' || b == 'P' {
			i++
			continue
		}
		break
	}
	return i
}

func isGoFunctionNameAt(s string, i int) bool {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i < len(s) && s[i] == '('
}

func goIdentifierKind(s string) SyntaxKind {
	switch s {
	case "break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan", "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue", "for", "import", "return", "var":
		return SyntaxKeyword
	case "true", "false":
		return SyntaxBoolean
	}
	if isGoType(s) {
		return SyntaxType
	}
	if isGoConstantName(s) {
		return SyntaxConstant
	}
	return SyntaxText
}

func goOperatorLen(s string, i int) int {
	if i+3 <= len(s) && s[i:i+3] == "..." {
		return 3
	}
	if i+3 <= len(s) {
		switch s[i : i+3] {
		case "<<=", ">>=", "&^=":
			return 3
		}
	}
	if i+2 <= len(s) {
		switch s[i : i+2] {
		case "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>", "&^", "&&", "||", "++", "--", "==", "!=", "<=", ">=", ":=":
			return 2
		}
	}
	_, n := utf8.DecodeRuneInString(s[i:])
	if n == 0 {
		return 1
	}
	return n
}

func isGoOperator(s string) bool {
	switch s {
	case "+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", "&^=", "&&", "||", "==", "!=", "<", ">", "<=", ">=", "=", "!", ":=", "++", "--":
		return true
	default:
		return false
	}
}

func highlightGeneric(lines []string) []SyntaxToken {
	return highlightGenericRange(lines, 0, len(lines))
}

func filterTokenLines(tokens []SyntaxToken, startLine, endLine int) []SyntaxToken {
	if startLine < 0 {
		startLine = 0
	}
	if endLine < startLine {
		endLine = startLine
	}
	out := make([]SyntaxToken, 0)
	for _, ts := range tokens {
		if ts.Line >= startLine && ts.Line < endLine {
			out = append(out, ts)
		}
	}
	return out
}

func isGenericKeyword(word string) bool {
	switch word {
	case "if", "else", "for", "while", "return", "func", "class", "struct", "interface", "package", "import", "const", "var", "let", "type", "switch", "case", "break", "continue", "true", "false", "null", "nil", "new", "public", "private", "static":
		return true
	default:
		return false
	}
}

func highlightGenericRange(lines []string, startLine, endLine int) []SyntaxToken {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if endLine < startLine {
		endLine = startLine
	}
	// Generic source commonly contains many words per line, so size the first
	// allocation from bytes rather than from line count. A too-large line-based
	// estimate alone can reserve tens of megabytes before the first token exists.
	totalBytes := 0
	for _, line := range lines[startLine:endLine] {
		totalBytes += len(line)
	}
	capHint := totalBytes / 4
	if capHint < 32 {
		capHint = 32
	}
	out := make([]SyntaxToken, 0, capHint)
	for li := startLine; li < endLine; li++ {
		line := lines[li]
		runeCol, lastByte := 0, 0
		for i := 0; i < len(line); {
			if line[i] == '#' || (i+1 < len(line) && line[i:i+2] == "//") {
				runeCol += utf8.RuneCountInString(line[lastByte:i])
				out = append(out, SyntaxToken{Line: li, Col: runeCol, Text: line[i:], Kind: SyntaxComment})
				break
			}
			if line[i] == '"' || line[i] == '\'' || line[i] == '`' {
				runeCol += utf8.RuneCountInString(line[lastByte:i])
				q := line[i]
				j := i + 1
				for j < len(line) {
					if line[j] == q && line[j-1] != '\\' {
						j++
						break
					}
					j++
				}
				out = append(out, SyntaxToken{Line: li, Col: runeCol, Text: line[i:j], Kind: SyntaxString})
				i = j
				lastByte = i
				continue
			}
			if isDigit(line[i]) {
				runeCol += utf8.RuneCountInString(line[lastByte:i])
				j := i + 1
				for j < len(line) && (isDigit(line[j]) || line[j] == '.' || line[j] == 'x' || line[j] == 'X' || line[j] >= 'a' && line[j] <= 'f' || line[j] >= 'A' && line[j] <= 'F') {
					j++
				}
				out = append(out, SyntaxToken{Line: li, Col: runeCol, Text: line[i:j], Kind: SyntaxNumber})
				i = j
				lastByte = i
				continue
			}
			if isIdentByte(line[i]) {
				runeCol += utf8.RuneCountInString(line[lastByte:i])
				j := i + 1
				for j < len(line) && isIdentByte(line[j]) {
					j++
				}
				kind := SyntaxText
				if isGenericKeyword(line[i:j]) {
					kind = SyntaxKeyword
				}
				if kind != SyntaxText {
					out = append(out, SyntaxToken{Line: li, Col: runeCol, Text: line[i:j], Kind: kind})
				}
				i = j
				lastByte = i
				continue
			}
			i++
		}
	}
	return out
}

func foldGoLines(lines []string) []FoldRange {
	stack := make([]int, 0, 32)
	out := make([]FoldRange, 0, 32)
	blockComment := false
	rawString := false

	for li, line := range lines {
		for i := 0; i < len(line); {
			if blockComment {
				if j := strings.Index(line[i:], "*/"); j >= 0 {
					i += j + 2
					blockComment = false
				} else {
					i = len(line)
				}
				continue
			}
			if rawString {
				if j := strings.IndexByte(line[i:], '`'); j >= 0 {
					i += j + 1
					rawString = false
				} else {
					i = len(line)
				}
				continue
			}
			switch {
			case line[i] == '/' && i+1 < len(line) && line[i+1] == '/':
				i = len(line)
			case line[i] == '/' && i+1 < len(line) && line[i+1] == '*':
				i += 2
				if j := strings.Index(line[i:], "*/"); j >= 0 {
					i += j + 2
				} else {
					i = len(line)
					blockComment = true
				}
			case line[i] == '`':
				i++
				if j := strings.IndexByte(line[i:], '`'); j >= 0 {
					i += j + 1
				} else {
					i = len(line)
					rawString = true
				}
			case line[i] == '"' || line[i] == '\'':
				q := line[i]
				i++
				for i < len(line) {
					if line[i] == '\\' {
						i += 2
						continue
					}
					if line[i] == q {
						i++
						break
					}
					i++
				}
			case line[i] == '{':
				stack = append(stack, li)
				i++
			case line[i] == '}':
				if n := len(stack); n > 0 {
					start := stack[n-1]
					stack = stack[:n-1]
					if li > start {
						out = append(out, FoldRange{Start: start, End: li})
					}
				}
				i++
			default:
				i++
			}
		}
	}
	return out
}

// DefaultFoldProvider uses brace matching for Go and indentation-neutral
// brace matching for generic source text.
type DefaultFoldProvider struct{}

func (DefaultFoldProvider) Folds(documentName, language string, lines []string) []FoldRange {
	if strings.EqualFold(language, "go") {
		// Folding is presentation metadata; the full Go parser/scanner is
		// unnecessary here and was a major cold-open allocation source for large
		// files. Use the same lexical rules as the lightweight editor highlighter
		// and track only brace positions.
		return foldGoLines(lines)
	}
	// Keep the generic fallback conservative: without a language parser, brace
	// matching can mistake strings/comments for syntax. Go has a real scanner,
	// so only Go receives automatic folds from the default provider.
	return nil
}

// GoFoldProvider is the explicit reusable Go folding implementation.
type GoFoldProvider struct{}

func (GoFoldProvider) Folds(documentName string, _ string, lines []string) []FoldRange {
	return (DefaultFoldProvider{}).Folds(documentName, "go", lines)
}
