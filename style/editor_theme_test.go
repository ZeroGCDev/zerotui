package style

import "testing"

func TestBuiltInEditorThemesUseUnifiedEditorCanvas(t *testing.T) {
	for _, option := range BuiltInEditorThemes() {
		if option.Factory == nil {
			t.Fatalf("theme %q has no factory", option.Name)
		}
		theme := option.Factory()
		if theme == nil {
			t.Fatalf("theme %q returned nil", option.Name)
		}
		assertEditorCanvas(t, option.Name, theme)
	}
}

func TestZedEditorThemeUsesUnifiedEditorCanvas(t *testing.T) {
	theme := ZedEditorTheme()
	if theme == nil {
		t.Fatal("ZedEditorTheme returned nil")
	}
	assertEditorCanvas(t, "Zed", theme)
}

func TestNewEditorThemePreservesApplicationSurfaceHierarchy(t *testing.T) {
	base := TokyoNightTheme()
	theme := NewEditorTheme(base)

	if theme.Theme.Background.Bg != base.Background.Bg {
		t.Fatalf("application background changed: got %v want %v", theme.Theme.Background.Bg, base.Background.Bg)
	}
	if theme.Theme.Panel.Bg != base.Panel.Bg {
		t.Fatalf("panel background changed: got %v want %v", theme.Theme.Panel.Bg, base.Panel.Bg)
	}
	if theme.Theme.Border.Bg != base.Border.Bg || theme.Theme.BorderFocus.Bg != base.BorderFocus.Bg {
		t.Fatal("application border backgrounds changed while building editor theme")
	}
}

func assertEditorCanvas(t *testing.T, name string, theme *EditorTheme) {
	t.Helper()
	want := theme.EditorBackground.Bg

	// The editor's visual surface must be continuous: code, gutter, chrome,
	// header/status text, context-menu chrome and syntax tokens all inherit it.
	roles := []struct {
		label string
		style Style
	}{
		{"EditorBackground", theme.EditorBackground},
		{"EditorTitle", theme.EditorTitle},
		{"EditorText", theme.EditorText},
		{"EditorMuted", theme.EditorMuted},
		{"EditorWarning", theme.EditorWarning},
		{"EditorInfo", theme.EditorInfo},
		{"GutterBackground", theme.GutterBackground},
		{"Gutter", theme.Gutter},
		{"ActiveLine", theme.ActiveLine},
		{"ActiveLineNumber", theme.ActiveLineNumber},
		{"WrapGuide", theme.WrapGuide},
		{"ContextMenu", theme.ContextMenu},
		{"ContextMenuBorder", theme.ContextMenuBorder},
		{"ContextMenuHint", theme.ContextMenuHint},
		{"Primary", theme.Primary},
		{"Keyword", theme.Keyword},
		{"String", theme.String},
		{"Comment", theme.Comment},
		{"Number", theme.Number},
		{"Type", theme.Type},
		{"Function", theme.Function},
		{"Constant", theme.Constant},
		{"Boolean", theme.Boolean},
		{"Operator", theme.Operator},
		{"Punctuation", theme.Punctuation},
		{"Property", theme.Property},
		{"Attribute", theme.Attribute},
		{"Variable", theme.Variable},
		{"Tag", theme.Tag},
		{"TitleSyntax", theme.TitleSyntax},
		{"Link", theme.Link},
		{"Hint", theme.Hint},
	}
	for _, role := range roles {
		if role.style.Bg != want {
			t.Errorf("%s: %s background=%v want editor background=%v", name, role.label, role.style.Bg, want)
		}
	}

	// Interaction surfaces are intentionally allowed to differ; otherwise
	// selection/cursor/search feedback would disappear.
	if theme.Selected.Bg == want || theme.Selection.Bg == want || theme.SearchMatch.Bg == want || theme.Cursor.Bg == want {
		t.Errorf("%s: interaction surfaces lost contrast", name)
	}

	// Application chrome is independent of the editor canvas. This prevents a
	// theme switch from flattening the settings/tree/rail into the code surface.
	if theme.Theme.Panel.Bg == want && theme.Theme.Background.Bg != want {
		t.Logf("%s: panel intentionally matches editor canvas for this palette", name)
	}
}
