package style

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ZeroGCDev/zerotui/color"
)

// EditorTheme is the richer theme surface used by the IDE editor. Theme is
// embedded so the same object can skin both the editor chrome and the source
// buffer. Syntax roles are deliberately explicit and exported, making a theme
// easy to modify without changing the editor widget.
type EditorTheme struct {
	Theme
	// Name is the human-readable settings label. It is empty for ad-hoc custom themes.
	Name string

	EditorBackground Style
	EditorTitle      Style
	EditorText       Style
	EditorMuted      Style
	EditorWarning    Style
	EditorInfo       Style
	GutterBackground Style
	Gutter           Style
	ActiveLine       Style
	ActiveLineNumber Style
	WrapGuide        Style
	Cursor           Style
	Selection        Style
	SearchMatch      Style

	// Context-menu roles keep editor actions readable on both dark and light
	// themes. They intentionally avoid terminal Reverse, which can turn a
	// dark editor menu into a harsh white block.
	ContextMenu         Style
	ContextMenuBorder   Style
	ContextMenuSelected Style
	ContextMenuHint     Style

	Primary     Style
	Keyword     Style
	String      Style
	Comment     Style
	Number      Style
	Type        Style
	Function    Style
	Constant    Style
	Boolean     Style
	Operator    Style
	Punctuation Style
	Property    Style
	Attribute   Style
	Variable    Style
	Tag         Style
	TitleSyntax Style
	Link        Style
	Hint        Style
}

// NewEditorTheme starts with the application theme and gives the editor a
// sensible set of source-code roles derived from it. Callers can then override
// individual fields before assigning ThemeOverride to an editor widget.
func NewEditorTheme(base *Theme) *EditorTheme {
	if base == nil {
		base = TokyoNightTheme()
	}
	t := &EditorTheme{Theme: *base}
	// Keep the application palette hierarchy intact: Background and Panel are
	// still allowed to differ for rails, trees and settings. The source editor
	// itself is a single continuous canvas, so its code, gutter, header and
	// editor-only text roles all use the exact same opaque background.
	canvasBg := base.Background.Bg
	t.EditorBackground = base.Background.WithBg(canvasBg)
	t.EditorTitle = base.Title.WithBg(canvasBg)
	t.EditorText = base.Text.WithBg(canvasBg)
	t.EditorMuted = base.TextMuted.WithBg(canvasBg)
	t.EditorWarning = base.Warning.WithBg(canvasBg)
	t.EditorInfo = base.Info.WithBg(canvasBg)
	t.GutterBackground = t.EditorBackground
	t.Gutter = base.TextMuted.WithBg(canvasBg)
	t.ActiveLine = base.Text.WithBg(canvasBg)
	t.ActiveLineNumber = base.Text.WithBg(canvasBg)
	t.WrapGuide = base.Border.WithBg(canvasBg)
	t.Cursor = base.Selected
	t.Selection = base.Selected
	t.SearchMatch = base.Selected
	t.ContextMenu = Style{Fg: base.Text.Fg, Bg: canvasBg}
	t.ContextMenuBorder = Style{Fg: base.BorderFocus.Fg, Bg: canvasBg}
	t.ContextMenuSelected = base.Selected.WithBg(base.BorderFocus.Fg)
	t.ContextMenuHint = Style{Fg: base.TextMuted.Fg, Bg: canvasBg}

	// Syntax styles are foreground roles only from a background perspective:
	// every one is explicitly rebound to the editor canvas. This is important
	// because the base Theme's Text/Title/etc. normally use the panel color.
	withCanvasBg := func(st Style) Style { return st.WithBg(canvasBg) }
	t.Primary = withCanvasBg(base.Text)
	t.Keyword = withCanvasBg(base.Title)
	t.String = withCanvasBg(base.Warning)
	t.Comment = withCanvasBg(base.TextMuted.WithAttr(Italic))
	t.Number = withCanvasBg(base.Info)
	t.Type = withCanvasBg(base.Info)
	t.Function = withCanvasBg(base.Warning)
	t.Constant = withCanvasBg(base.Info)
	t.Boolean = withCanvasBg(base.Positive)
	t.Operator = withCanvasBg(base.Title)
	t.Punctuation = withCanvasBg(base.Text)
	t.Property = withCanvasBg(base.Title)
	t.Attribute = withCanvasBg(base.Title)
	t.Variable = withCanvasBg(base.Text)
	t.Tag = withCanvasBg(base.Title)
	t.TitleSyntax = withCanvasBg(base.Warning)
	t.Link = withCanvasBg(base.Info)
	t.Hint = withCanvasBg(base.TextMuted)
	return t
}

func (t *EditorTheme) Clone() *EditorTheme {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

// WithEditorBackground returns a copy whose editor chrome and source canvas
// share the exact same opaque background. Intentional state backgrounds such
// as Selected and SearchMatch are preserved.
func (t *EditorTheme) WithEditorBackground() *EditorTheme {
	if t == nil {
		return nil
	}
	c := *t
	bg := c.EditorBackground.Bg
	roles := []*Style{
		&c.EditorBackground, &c.EditorTitle, &c.EditorText, &c.EditorMuted,
		&c.EditorWarning, &c.EditorInfo, &c.GutterBackground, &c.Gutter,
		&c.ActiveLine, &c.ActiveLineNumber, &c.WrapGuide,
		&c.ContextMenu, &c.ContextMenuBorder, &c.ContextMenuHint,
		&c.Primary, &c.Keyword, &c.String, &c.Comment, &c.Number, &c.Type,
		&c.Function, &c.Constant, &c.Boolean, &c.Operator, &c.Punctuation,
		&c.Property, &c.Attribute, &c.Variable, &c.Tag, &c.TitleSyntax, &c.Link,
		&c.Hint,
	}
	for _, st := range roles {
		st.Bg = bg
	}
	// Selected/search/cursor surfaces are deliberate interaction states and
	// remain independent of the editor canvas. The application Background and
	// Panel roles are also intentionally untouched.
	return &c
}

var zedDefaultOnce sync.Once
var zedDefaultTheme *EditorTheme

func defaultZedEditorTheme() *EditorTheme {
	zedDefaultOnce.Do(func() {
		t, err := LoadZedEditorTheme([]byte(defaultZedThemeJSON))
		if err != nil {
			t = NewEditorTheme(TokyoNightTheme())
		}
		t.Name = "Zed Dark"
		zedDefaultTheme = t
	})
	return zedDefaultTheme
}

// ZedTheme returns a fresh UI palette for the built-in Zed-style theme. The
// JSON is parsed once; callers still receive an independent value they can
// mutate without affecting other components.
func ZedTheme() *Theme {
	return defaultZedEditorTheme().Theme.Clone()
}

// ZedEditorTheme returns a fresh editor interpretation of the built-in
// Zed-style palette. Parsing is cached because this factory is also used by
// theme previews and settings lookups.
func ZedEditorTheme() *EditorTheme {
	return defaultZedEditorTheme().Clone()
}

// LoadZedEditorTheme parses the compact theme format used by Zed: the first
// entry in "themes" is read and its "style" role map is translated into the
// zerotui Theme plus editor syntax roles. Alpha colors are composited over the
// relevant base color because terminal cells use opaque RGB.
func LoadZedEditorTheme(data []byte) (*EditorTheme, error) {
	var file struct {
		Themes []struct {
			Style map[string]json.RawMessage `json:"style"`
		} `json:"themes"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if len(file.Themes) == 0 {
		return nil, fmt.Errorf("zed theme contains no themes")
	}
	m := file.Themes[0].Style
	get := func(key string, fallback color.Color) color.Color {
		raw, ok := m[key]
		if !ok {
			return fallback
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fallback
		}
		if c, ok := parseThemeColor(value, fallback); ok {
			return c
		}
		return fallback
	}
	var syntax map[string]struct {
		Color     string `json:"color"`
		FontStyle string `json:"font_style"`
	}
	if raw, ok := m["syntax"]; ok {
		_ = json.Unmarshal(raw, &syntax)
	}
	getSyntax := func(key string, fallback color.Color) color.Color {
		if role, ok := syntax[key]; ok && role.Color != "" {
			if c, ok := parseThemeColor(role.Color, fallback); ok {
				return c
			}
		}
		return fallback
	}
	syntaxStyle := func(key string, fallback Style) Style {
		st := fallback
		if role, ok := syntax[key]; ok {
			if role.Color != "" {
				if c, ok := parseThemeColor(role.Color, fallback.Fg); ok {
					st.Fg = c
				}
			}
			switch strings.ToLower(role.FontStyle) {
			case "italic":
				st.Attr |= Italic
			case "bold":
				st.Attr |= Bold
			case "underline":
				st.Attr |= Underline
			}
		}
		return st
	}

	background := get("background", color.RGB(21, 20, 27))
	panel := get("panel.background", background)
	text := get("text", color.RGB(237, 236, 238))
	muted := get("text.muted", color.RGB(109, 109, 109))
	border := get("border", color.RGB(61, 55, 94))
	accent := get("text.accent", color.RGB(162, 119, 255))
	info := get("info", color.RGB(255, 202, 133))
	success := get("success", color.RGB(97, 255, 202))
	errorColor := get("error", color.RGB(255, 103, 103))
	warning := get("warning", color.RGB(255, 202, 133))
	selectedBg := get("element.selected", color.RGB(61, 55, 94))
	selectedFg := text

	base := &Theme{
		Background:  Style{Fg: text, Bg: background},
		Panel:       Style{Fg: text, Bg: panel},
		Border:      Style{Fg: border, Bg: panel},
		BorderFocus: Style{Fg: accent, Bg: panel},
		Text:        Style{Fg: text, Bg: panel},
		TextMuted:   Style{Fg: muted, Bg: panel},
		Title:       Style{Fg: accent, Bg: panel, Attr: Bold},
		Positive:    Style{Fg: success, Bg: panel, Attr: Bold},
		Negative:    Style{Fg: errorColor, Bg: panel, Attr: Bold},
		Warning:     Style{Fg: warning, Bg: panel, Attr: Bold},
		Info:        Style{Fg: info, Bg: panel},
		Selected:    Style{Fg: selectedFg, Bg: selectedBg},
		TrackFull:   Style{Fg: success, Bg: panel},
		TrackEmpty:  Style{Fg: border, Bg: panel},
	}

	t := NewEditorTheme(base)
	t.EditorBackground = Style{Fg: text, Bg: get("editor.background", background)}
	// The editor canvas is the visual source of truth. The surrounding editor
	// chrome uses this same background even when a theme defines a separate
	// generic panel color.
	t.EditorTitle = Style{Fg: accent, Bg: t.EditorBackground.Bg, Attr: Bold}
	t.EditorText = Style{Fg: text, Bg: t.EditorBackground.Bg}
	t.EditorMuted = Style{Fg: muted, Bg: t.EditorBackground.Bg}
	t.EditorWarning = Style{Fg: warning, Bg: t.EditorBackground.Bg}
	t.EditorInfo = Style{Fg: info, Bg: t.EditorBackground.Bg}
	t.GutterBackground = Style{Fg: text, Bg: t.EditorBackground.Bg}
	t.Gutter = Style{Fg: get("editor.line_number", muted), Bg: t.GutterBackground.Bg}
	// Do not paint a full-width opaque active-line stripe. It creates a second
	// background surface and makes the editor look like nested rectangles.
	// Cursor/selection/active line number still provide focus feedback.
	t.ActiveLine = Style{Fg: text, Bg: t.EditorBackground.Bg}
	t.ActiveLineNumber = Style{Fg: get("editor.active_line_number", text), Bg: t.EditorBackground.Bg}
	t.WrapGuide = Style{Fg: get("editor.wrap_guide", border), Bg: t.EditorBackground.Bg}
	t.Cursor = Style{Fg: get("text", text), Bg: get("border.selected", accent), Attr: Bold}
	t.Selection = Style{Fg: text, Bg: get("search.match_background", selectedBg)}
	t.SearchMatch = Style{Fg: text, Bg: get("search.match_background", selectedBg)}
	t.ContextMenu = Style{Fg: text, Bg: panel}
	t.ContextMenuBorder = Style{Fg: accent, Bg: panel}
	t.ContextMenuSelected = Style{Fg: selectedFg, Bg: selectedBg, Attr: Bold}
	t.ContextMenuHint = Style{Fg: muted, Bg: panel}

	t.Primary = syntaxStyle("primary", Style{Fg: getSyntax("primary", text), Bg: t.EditorBackground.Bg})
	t.Keyword = syntaxStyle("keyword", Style{Fg: getSyntax("keyword", accent), Bg: t.EditorBackground.Bg})
	t.String = syntaxStyle("string", Style{Fg: getSyntax("string", success), Bg: t.EditorBackground.Bg})
	t.Comment = syntaxStyle("comment", Style{Fg: getSyntax("comment", muted), Bg: t.EditorBackground.Bg, Attr: Italic})
	t.Number = syntaxStyle("number", Style{Fg: getSyntax("number", success), Bg: t.EditorBackground.Bg})
	t.Type = syntaxStyle("type", Style{Fg: getSyntax("type", accent), Bg: t.EditorBackground.Bg})
	t.Function = syntaxStyle("function", Style{Fg: getSyntax("function", warning), Bg: t.EditorBackground.Bg})
	t.Constant = syntaxStyle("constant", Style{Fg: getSyntax("constant", getSyntax("number", success)), Bg: t.EditorBackground.Bg})
	t.Boolean = syntaxStyle("boolean", Style{Fg: getSyntax("boolean", success), Bg: t.EditorBackground.Bg})
	t.Operator = syntaxStyle("operator", Style{Fg: getSyntax("operator", accent), Bg: t.EditorBackground.Bg})
	t.Punctuation = syntaxStyle("punctuation", Style{Fg: getSyntax("punctuation", text), Bg: t.EditorBackground.Bg})
	t.Property = syntaxStyle("property", Style{Fg: getSyntax("property", getSyntax("attribute", accent)), Bg: t.EditorBackground.Bg})
	t.Attribute = syntaxStyle("attribute", Style{Fg: getSyntax("attribute", accent), Bg: t.EditorBackground.Bg})
	t.Variable = syntaxStyle("variable", Style{Fg: getSyntax("variable", text), Bg: t.EditorBackground.Bg})
	t.Tag = syntaxStyle("tag", Style{Fg: getSyntax("tag", accent), Bg: t.EditorBackground.Bg})
	t.TitleSyntax = syntaxStyle("title", Style{Fg: getSyntax("title", warning), Bg: t.EditorBackground.Bg})
	t.Link = syntaxStyle("link_text", Style{Fg: getSyntax("link_text", get("link_text.hover", success)), Bg: t.EditorBackground.Bg})
	t.Hint = syntaxStyle("hint", Style{Fg: getSyntax("hint", get("hint", color.RGB(130, 130, 130))), Bg: t.EditorBackground.Bg})

	// Keep the source canvas visually continuous. Theme-specific foregrounds
	// remain intact, while all non-interaction backgrounds resolve to the exact
	// editor background.
	t = t.WithEditorBackground()
	return t, nil
}

// EditorThemeOption describes one selectable built-in editor appearance.
// Factories return fresh theme values so changing one editor never mutates a
// palette shared by another editor instance.
type EditorThemeOption struct {
	Name    string
	Factory func() *EditorTheme
}

// BuiltInEditorThemes returns the themes exposed by the IDE settings panel.
// The list deliberately includes dark, colorful and light choices rather than
// treating the editor as permanently tied to the original Zed palette.
func BuiltInEditorThemes() []EditorThemeOption {
	return []EditorThemeOption{
		{Name: "Zed Dark", Factory: ZedEditorTheme},
		{Name: "Tokyo Night", Factory: func() *EditorTheme { return namedEditorTheme("Tokyo Night", TokyoNightTheme()) }},
		{Name: "Catppuccin Mocha", Factory: func() *EditorTheme { return namedEditorTheme("Catppuccin Mocha", CatppuccinMochaTheme()) }},
		{Name: "Dracula", Factory: func() *EditorTheme { return namedEditorTheme("Dracula", DraculaTheme()) }},
		{Name: "Nord", Factory: func() *EditorTheme { return namedEditorTheme("Nord", NordTheme()) }},
		{Name: "Rose Pine", Factory: func() *EditorTheme { return namedEditorTheme("Rose Pine", RosePineTheme()) }},
		{Name: "Cyberpunk", Factory: func() *EditorTheme { return namedEditorTheme("Cyberpunk", CyberpunkTheme()) }},
		{Name: "Synthwave", Factory: func() *EditorTheme { return namedEditorTheme("Synthwave", SynthwaveTheme()) }},
		{Name: "Vaporwave", Factory: func() *EditorTheme { return namedEditorTheme("Vaporwave", VaporwaveTheme()) }},
		{Name: "Matcha Latte", Factory: func() *EditorTheme { return namedEditorTheme("Matcha Latte", MatchaLatteTheme()) }},
		{Name: "Mocha Espresso", Factory: func() *EditorTheme { return namedEditorTheme("Mocha Espresso", MochaEspressoTheme()) }},
		{Name: "Deep Abyss", Factory: func() *EditorTheme { return namedEditorTheme("Deep Abyss", DeepAbyssTheme()) }},
		{Name: "Autumn", Factory: func() *EditorTheme { return namedEditorTheme("Autumn", AutumnTheme()) }},
		{Name: "Solarized Light", Factory: func() *EditorTheme { return namedEditorTheme("Solarized Light", SolarizedLightTheme()) }},
	}
}

func namedEditorTheme(name string, base *Theme) *EditorTheme {
	t := NewEditorTheme(base)
	t.Name = name
	return t
}

// EditorThemeByName returns a fresh theme for a registered name.
func EditorThemeByName(name string) (*EditorTheme, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "zed dark":
		return ZedEditorTheme(), true
	case "tokyo night":
		return namedEditorTheme("Tokyo Night", TokyoNightTheme()), true
	case "catppuccin mocha":
		return namedEditorTheme("Catppuccin Mocha", CatppuccinMochaTheme()), true
	case "dracula":
		return namedEditorTheme("Dracula", DraculaTheme()), true
	case "nord":
		return namedEditorTheme("Nord", NordTheme()), true
	case "rose pine":
		return namedEditorTheme("Rose Pine", RosePineTheme()), true
	case "cyberpunk":
		return namedEditorTheme("Cyberpunk", CyberpunkTheme()), true
	case "synthwave":
		return namedEditorTheme("Synthwave", SynthwaveTheme()), true
	case "vaporwave":
		return namedEditorTheme("Vaporwave", VaporwaveTheme()), true
	case "matcha latte":
		return namedEditorTheme("Matcha Latte", MatchaLatteTheme()), true
	case "mocha espresso":
		return namedEditorTheme("Mocha Espresso", MochaEspressoTheme()), true
	case "deep abyss":
		return namedEditorTheme("Deep Abyss", DeepAbyssTheme()), true
	case "autumn":
		return namedEditorTheme("Autumn", AutumnTheme()), true
	case "solarized light":
		return namedEditorTheme("Solarized Light", SolarizedLightTheme()), true
	default:
		return nil, false
	}
}

// LoadZedEditorThemeFile loads a Zed theme file from disk.
func LoadZedEditorThemeFile(path string) (*EditorTheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadZedEditorTheme(data)
}

// LoadZedTheme is a short alias for LoadZedEditorTheme for callers that want
// to import a Zed JSON theme without caring about the implementation name.
func LoadZedTheme(data []byte) (*EditorTheme, error) { return LoadZedEditorTheme(data) }

func LoadZedEditorThemeFileFromBytes(data []byte) (*EditorTheme, error) {
	return LoadZedEditorTheme(data)
}

func parseThemeColor(s string, under color.Color) (color.Color, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if len(s) != 6 && len(s) != 8 {
		return color.Default, false
	}
	var v uint64
	if _, err := fmt.Sscanf(s, "%x", &v); err != nil {
		return color.Default, false
	}
	var r, g, b uint8
	var a uint8 = 255
	if len(s) == 8 {
		a = uint8(v)
		v >>= 8
	}
	r, g, b = uint8(v>>16), uint8(v>>8), uint8(v)
	if a == 255 {
		return color.RGB(r, g, b), true
	}
	ur, ug, ub := under.Components()
	blend := func(fg, bg uint8) uint8 {
		return uint8((int(fg)*int(a) + int(bg)*(255-int(a))) / 255)
	}
	return color.RGB(blend(r, ur), blend(g, ug), blend(b, ub)), true
}

// This is intentionally kept small and mirrors the user-provided theme file.
// It makes ZedEditorTheme available without requiring applications to ship a
// second runtime asset.
const defaultZedThemeJSON = `{
  "themes": [{
    "name": "Zerotui Zed Dark",
    "style": {
      "background": "#15141b",
      "panel.background": "#15141c",
      "editor.background": "#15141b",
      "editor.gutter.background": "#15141b",
      "editor.active_line.background": "#a394f033",
      "editor.active_line_number": "#edecee",
      "editor.line_number": "#a394f033",
      "editor.wrap_guide": "#4d4d4d",
      "text": "#edecee",
      "text.accent": "#a277ff",
      "text.muted": "#6d6d6d",
      "border": "#3d375e7f",
      "border.selected": "#a277ff",
      "element.selected": "#3d375e7f",
      "success": "#61ffca",
      "error": "#ff6767",
      "warning": "#ffca85",
      "info": "#ffca85",
      "search.match_background": "#3d375e7f"
    },
    "syntax": {
      "attribute": {"color": "#f694ff"},
      "boolean": {"color": "#61ffca"},
      "comment": {"color": "#6d6d6d", "font_style": "italic"},
      "constant": {"color": "#82e2ff"},
      "function": {"color": "#ffca85"},
      "keyword": {"color": "#a277ff"},
      "number": {"color": "#61ffca"},
      "operator": {"color": "#a277ff"},
      "property": {"color": "#f694ff"},
      "punctuation": {"color": "#edecee"},
      "string": {"color": "#61ffca"},
      "tag": {"color": "#a277ff"},
      "type": {"color": "#a277ff"},
      "variable": {"color": "#edecee"},
      "title": {"color": "#ffca85"},
      "link_text": {"color": "#49c29a"},
      "hint": {"color": "#82e2ff"}
    }
  }]
}`
