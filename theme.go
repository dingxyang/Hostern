package main

import (
	"strconv"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type palette struct {
	chrome  ui.Theme
	console terminal.Theme
}

var lightPalette = newPalette(false)
var darkPalette = newPalette(true)

type themeStyle struct {
	id, chinese, english string
	light, dark          *palette
}

var themeStyles = []themeStyle{
	{"", "默认", "Default", lightPalette, darkPalette},
	{"ocean", "海蓝", "Ocean",
		tintedPalette(lightPalette, "#EEF5FA", "#F8FCFF", "#1D678F", "#246F9B"),
		tintedPalette(darkPalette, "#14222E", "#192C3A", "#276B96", "#83C8F4")},
	{"forest", "松林", "Forest",
		tintedPalette(lightPalette, "#EFF5ED", "#F8FCF6", "#366D45", "#397B4D"),
		tintedPalette(darkPalette, "#17251F", "#1C3027", "#347153", "#91D7A8")},
}

func appearanceStyle(id string) themeStyle {
	for _, style := range themeStyles {
		if style.id == id {
			return style
		}
	}
	return themeStyles[0]
}

func tintedPalette(base *palette, background, workspace, accent, focus string) *palette {
	p := *base
	chrome := &p.chrome
	chrome.Background = ui.Hex(background)
	chrome.Surface = ui.Hex(workspace)
	chrome.Accent, chrome.Focus = ui.Hex(accent), ui.Hex(focus)
	chrome.Border = chrome.Background.Mix(chrome.Text, 0.15)
	chrome.SurfaceHover = chrome.Surface.Mix(chrome.Text, 0.08)
	chrome.SurfacePressed = chrome.Surface.Mix(chrome.Text, 0.13)
	chrome.AccentHover = chrome.Accent.Mix(ui.Hex("#000000"), 0.1)
	chrome.AccentPressed = chrome.Accent.Mix(ui.Hex("#000000"), 0.2)
	chrome.Selection = chrome.Background.Mix(chrome.Focus, 0.18)
	p.console.Background = chrome.Surface
	p.console.Cursor = chrome.Focus
	p.console.Selection = p.console.Background.Mix(chrome.Focus, 0.28)
	return &p
}

func normalizeAppearance(source mygo.ThemeSource) mygo.ThemeSource {
	switch source {
	case mygo.ThemeLight, mygo.ThemeDark:
		return source
	default:
		return mygo.ThemeSystem
	}
}

func paletteFor(source mygo.ThemeSource, systemDark bool, id string) *palette {
	style := appearanceStyle(id)
	if source == mygo.ThemeDark || (normalizeAppearance(source) == mygo.ThemeSystem && systemDark) {
		return style.dark
	}
	return style.light
}

func newPalette(dark bool) *palette {
	chrome, console := ui.LightTheme(), terminal.LightTheme()
	workspace := ui.Hex("#FCFCFD")
	chrome.Background = ui.Hex("#F4F4F7")
	chrome.Border = ui.Hex("#E4E4EA")
	chrome.Text, chrome.TextMuted = ui.Hex("#2A2A33"), ui.Hex("#646471")
	chrome.Surface, chrome.SurfaceHover, chrome.SurfacePressed = ui.Hex("#FFFFFF"), ui.Hex("#ECECF2"), ui.Hex("#E3E3EB")
	chrome.Accent, chrome.Focus = ui.Hex("#6D5BD0"), ui.Hex("#6D5BD0")
	chrome.Danger = ui.Hex("#B52B4C")
	ansi := [16]string{
		"#2A2A33", "#B82349", "#317321", "#885607", "#2A5BD7", "#7C3BD6", "#086A83", "#646471",
		"#575764", "#C73352", "#387A2A", "#945B17", "#3865CD", "#8645CD", "#0E7188", "#464652",
	}
	if dark {
		chrome, console = ui.DarkTheme(), terminal.DarkTheme()
		workspace = ui.Hex("#18181D")
		chrome.Background = ui.Hex("#131317")
		chrome.Border = ui.Hex("#2D2D36")
		chrome.Text, chrome.TextMuted = ui.Hex("#D6D6DE"), ui.Hex("#9595A2")
		chrome.Surface, chrome.SurfaceHover, chrome.SurfacePressed = ui.Hex("#202027"), ui.Hex("#272730"), ui.Hex("#2E2E38")
		chrome.Accent, chrome.Focus = ui.Hex("#7260E3"), ui.Hex("#A397FF")
		chrome.Danger = ui.Hex("#F0718B")
		ansi = [16]string{
			"#2A2A33", "#F0718B", "#A9D48F", "#E8C482", "#8FB4FF", "#B99CF7", "#7DCFE0", "#D6D6DE",
			"#777783", "#FF8FA5", "#BFE6A5", "#F5D699", "#A9C6FF", "#CDB5FA", "#9EE0EE", "#FFFFFF",
		}
	}
	chrome.AccentText = ui.Hex("#FFFFFF")
	chrome.AccentHover = chrome.Accent.Mix(ui.Hex("#000000"), 0.1)
	chrome.AccentPressed = chrome.Accent.Mix(ui.Hex("#000000"), 0.2)
	chrome.Selection = chrome.Background.Mix(chrome.Focus, 0.18)
	console.Background, console.Foreground = workspace, chrome.Text
	console.Cursor = chrome.Focus
	console.Selection = workspace.Mix(chrome.Focus, 0.28)
	for i, color := range ansi {
		console.Palette[i] = ui.Hex(color)
	}
	return &palette{chrome: *chrome, console: *console}
}

func fontWeight(bold bool) int {
	if bold {
		return 700
	}
	return 400
}

func interfaceTextSize(c *ui.Context, size float32) float32 {
	return c.Theme().FontSize * size / defaultInterfaceSize
}

func (a *localTerm) applyTheme(c *ui.Context) {
	appearance := a.preferences
	if a.settingsOpen {
		appearance = a.settingsDraft
	}
	p := paletteFor(appearance.Appearance, c.Theme().Dark, appearance.ThemeStyle)
	theme := p.chrome
	if appearance.UIFont != "" && appearance.UIFont != systemUIFont {
		theme.Font = appearance.UIFont
	}
	if size, _ := strconv.Atoi(appearance.UIFontSize); size > 0 {
		theme.FontSize = float32(size)
	}
	c.SetTheme(&theme)
	c.Root().FontWeight(fontWeight(appearance.UIBold))
	if a.consoleTheme != &p.console {
		a.consoleTheme = &p.console
		for _, s := range a.tabs {
			if s.term != nil {
				s.term.SetTheme(a.consoleTheme, nil)
			}
		}
	}
}
