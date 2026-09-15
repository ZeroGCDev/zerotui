package widget

import (
	"strings"
	"unicode/utf8"
)

// languageProfile describes presentation-level lexical rules. These lexers are
// intentionally lightweight: they never build an AST and only emit tokens for
// the requested line range, keeping CodeEditor suitable for large documents.
type languageProfile struct {
	lineComment string
	blockOpen   string
	blockClose  string
	keywords    map[string]SyntaxKind
	constants   map[string]bool
	types       map[string]bool
	strings     []byte
	markdown    bool
	config      bool
	shell       bool
}

var syntaxProfiles = map[string]languageProfile{
	"python":     profile("#", "", "", pythonKeywords, nil, pythonTypes, false, false, false),
	"py":         profile("#", "", "", pythonKeywords, nil, pythonTypes, false, false, false),
	"json":       profile("", "", "", jsonKeywords, nil, nil, false, true, false),
	"yaml":       profile("#", "", "", yamlKeywords, nil, nil, false, true, false),
	"yml":        profile("#", "", "", yamlKeywords, nil, nil, false, true, false),
	"toml":       profile("#", "", "", tomlKeywords, nil, nil, false, true, false),
	"rust":       profile("//", "/*", "*/", rustKeywords, rustConstants, rustTypes, false, false, false),
	"java":       profile("//", "/*", "*/", javaKeywords, javaConstants, javaTypes, false, false, false),
	"c":          profile("//", "/*", "*/", cKeywords, cConstants, cTypes, false, false, false),
	"h":          profile("//", "/*", "*/", cKeywords, cConstants, cTypes, false, false, false),
	"cpp":        profile("//", "/*", "*/", cppKeywords, cppConstants, cppTypes, false, false, false),
	"cc":         profile("//", "/*", "*/", cppKeywords, cppConstants, cppTypes, false, false, false),
	"cxx":        profile("//", "/*", "*/", cppKeywords, cppConstants, cppTypes, false, false, false),
	"hpp":        profile("//", "/*", "*/", cppKeywords, cppConstants, cppTypes, false, false, false),
	"javascript": profile("//", "/*", "*/", jsKeywords, jsConstants, jsTypes, false, false, false),
	"js":         profile("//", "/*", "*/", jsKeywords, jsConstants, jsTypes, false, false, false),
	"jsx":        profile("//", "/*", "*/", jsKeywords, jsConstants, jsTypes, false, false, false),
	"bash":       profile("#", "", "", shellKeywords, shellConstants, nil, false, false, true),
	"sh":         profile("#", "", "", shellKeywords, shellConstants, nil, false, false, true),
	"markdown":   profile("", "", "", nil, nil, nil, true, false, false),
	"md":         profile("", "", "", nil, nil, nil, true, false, false),
}

func profile(line, open, close string, kw map[string]SyntaxKind, constants, types map[string]bool, markdown, config, shell bool) languageProfile {
	return languageProfile{lineComment: line, blockOpen: open, blockClose: close, keywords: kw, constants: constants, types: types, strings: []byte{'"', '\'', '`'}, markdown: markdown, config: config, shell: shell}
}

func languageProfileFor(language string) (languageProfile, bool) {
	p, ok := syntaxProfiles[strings.ToLower(strings.TrimSpace(language))]
	return p, ok
}

func highlightLanguageRange(lines []string, startLine, endLine int, p languageProfile) []SyntaxToken {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if endLine < startLine {
		endLine = startLine
	}
	totalBytes := 0
	for _, line := range lines[startLine:endLine] {
		totalBytes += len(line)
	}
	capHint := totalBytes/5 + 32
	if capHint < 32 {
		capHint = 32
	}
	out := make([]SyntaxToken, 0, capHint)
	for li := startLine; li < endLine; li++ {
		if p.markdown {
			highlightMarkdownLine(&out, li, lines[li])
			continue
		}
		highlightLanguageLine(&out, li, lines[li], p)
	}
	return out
}

func highlightLanguageLine(out *[]SyntaxToken, lineNo int, line string, p languageProfile) {
	// Keep the current rune column as we scan the line. The old implementation
	// recomputed it from the beginning of the line for every token, which made
	// syntax highlighting unnecessarily quadratic on token-dense lines.
	runeCol := 0
	for i := 0; i < len(line); {
		if p.lineComment != "" && strings.HasPrefix(line[i:], p.lineComment) {
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: line[i:], Kind: SyntaxComment})
			return
		}
		if p.blockOpen != "" && strings.HasPrefix(line[i:], p.blockOpen) {
			j := strings.Index(line[i+len(p.blockOpen):], p.blockClose)
			end := len(line)
			if j >= 0 {
				end = i + len(p.blockOpen) + j + len(p.blockClose)
			}
			text := line[i:end]
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: text, Kind: SyntaxComment})
			runeCol += utf8.RuneCountInString(text)
			i = end
			continue
		}
		if isQuote(line[i]) {
			j := scanQuoted(line, i)
			text := line[i:j]
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: text, Kind: SyntaxString})
			runeCol += utf8.RuneCountInString(text)
			i = j
			continue
		}
		if isNumberByte(line[i]) && (i == 0 || !isIdentByte(line[i-1])) {
			j := scanLanguageNumber(line, i)
			text := line[i:j]
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: text, Kind: SyntaxNumber})
			runeCol += utf8.RuneCountInString(text)
			i = j
			continue
		}
		if isIdentByte(line[i]) {
			j := i + 1
			for j < len(line) && isIdentByte(line[j]) {
				j++
			}
			word := line[i:j]
			kind := SyntaxText
			if p.keywords[word] != SyntaxText {
				kind = p.keywords[word]
			}
			if p.types[word] {
				kind = SyntaxType
			}
			if p.constants[word] {
				kind = SyntaxConstant
			}
			if (p.shell && i > 0 && line[i-1] == '$') || (j < len(line) && nextNonSpace(line, j) == '(' && !p.config) {
				kind = SyntaxFunction
			}
			if word == "true" || word == "false" || word == "null" || word == "nil" || word == "None" {
				kind = SyntaxBoolean
			}
			if kind != SyntaxText || p.config && isConfigKey(line, j) {
				*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: word, Kind: kindOrConfig(kind, p.config, line, j)})
			}
			runeCol += utf8.RuneCountInString(word)
			i = j
			continue
		}
		if isOperatorByte(line[i]) {
			j := i + 1
			if j < len(line) && strings.ContainsRune("=<>|&+-", rune(line[j])) {
				j++
			}
			text := line[i:j]
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: text, Kind: SyntaxOperator})
			runeCol += utf8.RuneCountInString(text)
			i = j
			continue
		}
		if strings.ContainsRune("{}[]();:,.", rune(line[i])) {
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeCol, Text: line[i : i+1], Kind: SyntaxPunctuation})
		}
		i++
		if line[i-1] < utf8.RuneSelf {
			runeCol++
		} else {
			_, n := utf8.DecodeRuneInString(line[i-1:])
			if n > 0 {
				i += n - 1
				runeCol++
			}
		}
	}
}

func kindOrConfig(kind SyntaxKind, config bool, line string, end int) SyntaxKind {
	if config && isConfigKey(line, end) {
		return SyntaxFunction
	}
	return kind
}

func isConfigKey(line string, end int) bool {
	for end < len(line) && (line[end] == ' ' || line[end] == '\t') {
		end++
	}
	return end < len(line) && (line[end] == ':' || line[end] == '=')
}

func highlightMarkdownLine(out *[]SyntaxToken, lineNo int, line string) {
	trim := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trim)
	if strings.HasPrefix(trim, "#") {
		*out = append(*out, SyntaxToken{Line: lineNo, Col: indent, Text: trim, Kind: SyntaxKeyword})
		return
	}
	for i := 0; i < len(line); {
		if strings.HasPrefix(line[i:], "<!--") {
			j := strings.Index(line[i+4:], "-->")
			end := len(line)
			if j >= 0 {
				end = i + 4 + j + 3
			}
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeColAt(line, i), Text: line[i:end], Kind: SyntaxComment})
			i = end
			continue
		}
		if line[i] == '`' {
			j := strings.IndexByte(line[i+1:], '`')
			end := len(line)
			if j >= 0 {
				end = i + 1 + j + 1
			}
			*out = append(*out, SyntaxToken{Line: lineNo, Col: runeColAt(line, i), Text: line[i:end], Kind: SyntaxString})
			i = end
			continue
		}
		if line[i] == '[' {
			if j := strings.Index(line[i:], "]("); j >= 0 {
				end := strings.IndexByte(line[i+j+2:], ')')
				if end >= 0 {
					end = i + j + 2 + end + 1
					*out = append(*out, SyntaxToken{Line: lineNo, Col: runeColAt(line, i), Text: line[i:end], Kind: SyntaxFunction})
					i = end
					continue
				}
			}
		}
		i++
	}
}

func runeColAt(s string, bytePos int) int { return utf8.RuneCountInString(s[:bytePos]) }
func isQuote(b byte) bool                 { return b == '\'' || b == '"' || b == '`' }
func scanQuoted(s string, i int) int {
	q := s[i]
	i++
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if i < len(s) && s[i] == q {
			return i + 1
		}
		i++
	}
	return len(s)
}
func scanLanguageNumber(s string, i int) int {
	for i < len(s) {
		b := s[i]
		if isIdentByte(b) || b == '.' || b == '+' || b == '-' {
			i++
			continue
		}
		break
	}
	return i
}
func isNumberByte(b byte) bool   { return b >= '0' && b <= '9' || b == '.' }
func isOperatorByte(b byte) bool { return strings.ContainsRune("+-*/%=!<>|&^~?", rune(b)) }
func nextNonSpace(s string, i int) byte {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i < len(s) {
		return s[i]
	}
	return 0
}

func kw(words string) map[string]SyntaxKind {
	m := map[string]SyntaxKind{}
	for _, w := range strings.Fields(words) {
		m[w] = SyntaxKeyword
	}
	return m
}
func types(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}
func consts(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var pythonKeywords = kw("and as assert async await break case class continue def del elif else except finally for from global if import in is lambda match nonlocal not or pass raise return try while with yield")
var pythonTypes = types("bool bytes complex dict float int list object set str tuple")
var jsonKeywords = kw("true false null")
var yamlKeywords = kw("true false null yes no on off")
var tomlKeywords = kw("true false")
var rustKeywords = kw("as async await break const continue crate dyn else enum extern false fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while async await")
var rustConstants = consts("None Some Ok Err")
var rustTypes = types("bool char str i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64")
var javaKeywords = kw("abstract assert boolean break byte case catch char class const continue default do double else enum extends final finally float for goto if implements import instanceof int interface long native new package private protected public return short static strictfp super switch synchronized this throw throws transient try void volatile while true false null")
var javaConstants = consts("System Math Integer Long String")
var javaTypes = types("boolean byte char double float int long short void")
var cKeywords = kw("auto break case char const continue default do double else enum extern float for goto if inline int long register restrict return short signed sizeof static struct switch typedef union unsigned void volatile while _Bool _Complex _Atomic")
var cConstants = consts("NULL EXIT_SUCCESS EXIT_FAILURE")
var cTypes = types("char double float int long short signed unsigned void size_t uint8_t uint16_t uint32_t uint64_t int8_t int16_t int32_t int64_t")
var cppKeywords = kw("alignas alignof and and_eq asm auto bitand bitor bool break case catch char class compl concept const consteval constexpr constinit const_cast continue co_await co_return co_yield decltype default delete do double dynamic_cast else enum explicit export extern false float for friend goto if inline int long mutable namespace new noexcept not nullptr operator or private protected public register reinterpret_cast requires return short signed sizeof static static_assert static_cast struct switch template this thread_local throw true try typedef typeid typename union unsigned using virtual void volatile wchar_t while xor")
var cppConstants = consts("NULL nullptr std true false")
var cppTypes = types("bool char double float int long short signed unsigned void wchar_t size_t uint8_t uint16_t uint32_t uint64_t int8_t int16_t int32_t int64_t")
var jsKeywords = kw("as async await break case catch class const continue debugger default delete do else export extends false finally for from function get if import in instanceof let new null of return set static super switch this throw true try typeof undefined var void while with yield")
var jsConstants = consts("JSON Math console Promise Array Object String Number Boolean RegExp Date Error")
var jsTypes = types("string number boolean bigint symbol object any unknown never void")
var shellKeywords = kw("if then else elif fi for while in do done case esac function select time coproc until return local export readonly declare typeset let")
var shellConstants = consts("HOME PATH PWD USER SHELL TERM IFS")
