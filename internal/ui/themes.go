package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// theme define la paleta de la interfaz. No cambia el fondo de la terminal:
// solo los colores de texto, la barra y la selección.
type theme struct {
	ID, Name, Kind                     string
	Accent, Muted, Faint, Text, Strong lipgloss.TerminalColor
	Red, Yellow, Green, Blue, Orange   lipgloss.TerminalColor
	BarBg, SelBg, OnAccent             lipgloss.TerminalColor
}

func hex(s string) lipgloss.TerminalColor { return lipgloss.Color(s) }

var themes = []theme{
	{
		ID: "auto", Name: "Automático", Kind: "terminal",
		Accent: adaptive("30", "45"), Muted: adaptive("243", "245"), Faint: adaptive("250", "239"),
		Text: adaptive("236", "252"), Strong: adaptive("232", "255"),
		Red: adaptive("160", "203"), Yellow: adaptive("130", "214"), Green: adaptive("28", "42"),
		Blue: adaptive("26", "75"), Orange: adaptive("166", "208"),
		BarBg: adaptive("254", "235"), SelBg: adaptive("252", "237"), OnAccent: adaptive("255", "234"),
	},
	{
		ID: "dracula", Name: "Dracula", Kind: "oscuro",
		Accent: hex("#bd93f9"), Muted: hex("#8a8fa8"), Faint: hex("#44475a"), Text: hex("#e2e2dc"), Strong: hex("#f8f8f2"),
		Red: hex("#ff5555"), Yellow: hex("#f1fa8c"), Green: hex("#50fa7b"), Blue: hex("#8be9fd"), Orange: hex("#ffb86c"),
		BarBg: hex("#21222c"), SelBg: hex("#44475a"), OnAccent: hex("#282a36"),
	},
	{
		ID: "catppuccin-mocha", Name: "Catppuccin Mocha", Kind: "oscuro",
		Accent: hex("#cba6f7"), Muted: hex("#9399b2"), Faint: hex("#45475a"), Text: hex("#bac2de"), Strong: hex("#cdd6f4"),
		Red: hex("#f38ba8"), Yellow: hex("#f9e2af"), Green: hex("#a6e3a1"), Blue: hex("#89b4fa"), Orange: hex("#fab387"),
		BarBg: hex("#181825"), SelBg: hex("#313244"), OnAccent: hex("#1e1e2e"),
	},
	{
		ID: "catppuccin-macchiato", Name: "Catppuccin Macchiato", Kind: "oscuro",
		Accent: hex("#c6a0f6"), Muted: hex("#939ab7"), Faint: hex("#494d64"), Text: hex("#b8c0e0"), Strong: hex("#cad3f5"),
		Red: hex("#ed8796"), Yellow: hex("#eed49f"), Green: hex("#a6da95"), Blue: hex("#8aadf4"), Orange: hex("#f5a97f"),
		BarBg: hex("#1e2030"), SelBg: hex("#363a4f"), OnAccent: hex("#24273a"),
	},
	{
		ID: "catppuccin-latte", Name: "Catppuccin Latte", Kind: "claro",
		Accent: hex("#8839ef"), Muted: hex("#7c7f93"), Faint: hex("#bcc0cc"), Text: hex("#5c5f77"), Strong: hex("#4c4f69"),
		Red: hex("#d20f39"), Yellow: hex("#df8e1d"), Green: hex("#40a02b"), Blue: hex("#1e66f5"), Orange: hex("#fe640b"),
		BarBg: hex("#e6e9ef"), SelBg: hex("#ccd0da"), OnAccent: hex("#eff1f5"),
	},
	{
		ID: "rose-pine", Name: "Rosé Pine", Kind: "oscuro",
		Accent: hex("#c4a7e7"), Muted: hex("#908caa"), Faint: hex("#403d52"), Text: hex("#d0cde8"), Strong: hex("#e0def4"),
		Red: hex("#eb6f92"), Yellow: hex("#f6c177"), Green: hex("#9ccfd8"), Blue: hex("#31748f"), Orange: hex("#ebbcba"),
		BarBg: hex("#1f1d2e"), SelBg: hex("#26233a"), OnAccent: hex("#191724"),
	},
	{
		ID: "rose-pine-moon", Name: "Rosé Pine Moon", Kind: "oscuro",
		Accent: hex("#c4a7e7"), Muted: hex("#908caa"), Faint: hex("#44415a"), Text: hex("#d0cde8"), Strong: hex("#e0def4"),
		Red: hex("#eb6f92"), Yellow: hex("#f6c177"), Green: hex("#9ccfd8"), Blue: hex("#3e8fb0"), Orange: hex("#ea9a97"),
		BarBg: hex("#2a273f"), SelBg: hex("#393552"), OnAccent: hex("#232136"),
	},
	{
		ID: "rose-pine-dawn", Name: "Rosé Pine Dawn", Kind: "claro",
		Accent: hex("#907aa9"), Muted: hex("#797593"), Faint: hex("#cecacd"), Text: hex("#575279"), Strong: hex("#464261"),
		Red: hex("#b4637a"), Yellow: hex("#ea9d34"), Green: hex("#56949f"), Blue: hex("#286983"), Orange: hex("#d7827e"),
		BarBg: hex("#f2e9e1"), SelBg: hex("#dfdad9"), OnAccent: hex("#faf4ed"),
	},
	{
		ID: "tokyo-night", Name: "Tokyo Night", Kind: "oscuro",
		Accent: hex("#7aa2f7"), Muted: hex("#737aa2"), Faint: hex("#3b4261"), Text: hex("#a9b1d6"), Strong: hex("#c0caf5"),
		Red: hex("#f7768e"), Yellow: hex("#e0af68"), Green: hex("#9ece6a"), Blue: hex("#7dcfff"), Orange: hex("#ff9e64"),
		BarBg: hex("#16161e"), SelBg: hex("#283457"), OnAccent: hex("#1a1b26"),
	},
	{
		ID: "gruvbox", Name: "Gruvbox", Kind: "oscuro",
		Accent: hex("#fabd2f"), Muted: hex("#a89984"), Faint: hex("#504945"), Text: hex("#d5c4a1"), Strong: hex("#ebdbb2"),
		Red: hex("#fb4934"), Yellow: hex("#fe8019"), Green: hex("#b8bb26"), Blue: hex("#83a598"), Orange: hex("#d3869b"),
		BarBg: hex("#1d2021"), SelBg: hex("#3c3836"), OnAccent: hex("#282828"),
	},
	{
		ID: "nord", Name: "Nord", Kind: "oscuro",
		Accent: hex("#88c0d0"), Muted: hex("#7b88a1"), Faint: hex("#434c5e"), Text: hex("#d8dee9"), Strong: hex("#eceff4"),
		Red: hex("#bf616a"), Yellow: hex("#ebcb8b"), Green: hex("#a3be8c"), Blue: hex("#81a1c1"), Orange: hex("#d08770"),
		BarBg: hex("#2e3440"), SelBg: hex("#3b4252"), OnAccent: hex("#2e3440"),
	},
	{
		ID: "solarized-light", Name: "Solarized Light", Kind: "claro",
		Accent: hex("#268bd2"), Muted: hex("#839496"), Faint: hex("#d3cbb7"), Text: hex("#586e75"), Strong: hex("#073642"),
		Red: hex("#dc322f"), Yellow: hex("#b58900"), Green: hex("#859900"), Blue: hex("#2aa198"), Orange: hex("#cb4b16"),
		BarBg: hex("#eee8d5"), SelBg: hex("#e4dcc5"), OnAccent: hex("#fdf6e3"),
	},
}

var currentTheme = themes[0].ID

// ConfigDir es donde se guarda la preferencia de tema; lo fija main.
var ConfigDir string

func themeByID(id string) (theme, bool) {
	for _, t := range themes {
		if t.ID == id {
			return t, true
		}
	}
	return theme{}, false
}

func applyTheme(t theme) {
	currentTheme = t.ID
	colorAccent, colorMuted, colorFaint, colorText, colorStrong = t.Accent, t.Muted, t.Faint, t.Text, t.Strong
	colorRed, colorYellow, colorGreen, colorBlue, colorOrange = t.Red, t.Yellow, t.Green, t.Blue, t.Orange
	colorBarBg, colorSelBg, colorOnAcc = t.BarBg, t.SelBg, t.OnAccent
	buildStyles()
}

func themeFile() string {
	if ConfigDir == "" {
		return ""
	}
	return filepath.Join(ConfigDir, "theme")
}

// loadTheme aplica el tema guardado, si existe y sigue siendo válido.
func loadTheme() {
	path := themeFile()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if t, ok := themeByID(strings.TrimSpace(string(raw))); ok {
		applyTheme(t)
	}
}

func saveTheme(id string) error {
	path := themeFile()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(id+"\n"), 0o644)
}

// openThemePicker lista los temas con vista previa al mover el cursor;
// esc restaura el que estaba.
func (a *App) openThemePicker() {
	previous := currentTheme
	items := make([]pickItem, len(themes))
	start := 0
	for i, t := range themes {
		items[i] = pickItem{Kind: "theme", Value: t.ID, Label: t.Name, Detail: t.Kind}
		if t.ID == previous {
			start = i
		}
	}
	preview := func(a *App, item pickItem) {
		if t, ok := themeByID(item.Value); ok {
			applyTheme(t)
			a.restyle()
			a.reloadAll()
		}
	}
	a.picker = newPicker("Tema de colores", items, func(a *App, item pickItem) {
		preview(a, item)
		if err := saveTheme(item.Value); err != nil {
			a.errf("No se pudo guardar el tema: %v", err)
			return
		}
		a.toastf("Tema: %s", item.Label)
	})
	a.picker.cursor = start
	a.picker.onMove = preview
	a.picker.onCancel = func(a *App) {
		if t, ok := themeByID(previous); ok {
			applyTheme(t)
			a.restyle()
			a.reloadAll()
		}
	}
	a.picker.layout(a.width, a.height-2)
}

// formTheme adapta los formularios huh a la paleta activa.
func formTheme() *huh.Theme {
	t := huh.ThemeBase()
	f := &t.Focused
	f.Base = f.Base.BorderForeground(colorAccent)
	f.Title = f.Title.Foreground(colorAccent).Bold(true)
	f.Description = f.Description.Foreground(colorMuted)
	f.ErrorIndicator = f.ErrorIndicator.Foreground(colorRed)
	f.ErrorMessage = f.ErrorMessage.Foreground(colorRed)
	f.SelectSelector = f.SelectSelector.Foreground(colorAccent)
	f.Option = f.Option.Foreground(colorText)
	f.SelectedOption = f.SelectedOption.Foreground(colorStrong)
	f.NextIndicator = f.NextIndicator.Foreground(colorMuted)
	f.PrevIndicator = f.PrevIndicator.Foreground(colorMuted)
	f.TextInput.Cursor = f.TextInput.Cursor.Foreground(colorAccent)
	f.TextInput.Prompt = f.TextInput.Prompt.Foreground(colorAccent)
	f.TextInput.Placeholder = f.TextInput.Placeholder.Foreground(colorFaint)
	f.TextInput.Text = f.TextInput.Text.Foreground(colorStrong)
	f.FocusedButton = lipgloss.NewStyle().Padding(0, 2).MarginRight(1).Bold(true).Foreground(colorOnAcc).Background(colorAccent)
	f.BlurredButton = lipgloss.NewStyle().Padding(0, 2).MarginRight(1).Foreground(colorText).Background(colorSelBg)
	f.MultiSelectSelector = f.MultiSelectSelector.Foreground(colorAccent)
	f.SelectedPrefix = lipgloss.NewStyle().Foreground(colorAccent).SetString("[•] ")
	f.UnselectedPrefix = lipgloss.NewStyle().Foreground(colorMuted).SetString("[ ] ")
	f.TextInput.Placeholder = f.TextInput.Placeholder.Foreground(colorMuted)
	f.Description = f.Description.Foreground(colorMuted)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Title = t.Blurred.Title.Foreground(colorMuted).Bold(false)
	t.Blurred.SelectSelector = t.Blurred.SelectSelector.Foreground(colorFaint)
	t.Blurred.TextInput.Prompt = t.Blurred.TextInput.Prompt.Foreground(colorFaint)
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(colorText)
	t.Blurred.NextIndicator, t.Blurred.PrevIndicator = lipgloss.NewStyle(), lipgloss.NewStyle()

	t.Group.Title = t.Group.Title.Foreground(colorStrong).Bold(true)
	t.Help.ShortKey = t.Help.ShortKey.Foreground(colorStrong).Bold(true)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(colorMuted)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(colorFaint)
	return t
}
