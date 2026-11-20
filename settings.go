package main

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type preferences struct {
	Appearance          mygo.ThemeSource             `json:"appearance"`
	UIFont              string                       `json:"interfaceFont,omitempty"`
	UIFontSize          string                       `json:"interfaceFontSize,omitempty"`
	UIBold              bool                         `json:"interfaceFontBold,omitempty"`
	TerminalBold        bool                         `json:"terminalFontBold,omitempty"`
	Language            string                       `json:"language"`
	Font                string                       `json:"terminalFont"`
	Size                string                       `json:"terminalFontSize"`
	SkipStartupTerminal bool                         `json:"skipStartupTerminal,omitempty"`
	Shortcuts           [len(shortcutActions)]string `json:"shortcuts"`
	QuickCommands       []quickCommand               `json:"quickCommands,omitempty"`
}

const (
	settingsAppearanceTab = iota
	settingsShortcutsTab
	settingsHostsTab
)

const systemUIFont = "System Default"

func (a *localTerm) loadFontOptions() {
	fonts := append(systemFontFamilies(), "Menlo", "Monaco", "SF Mono", "Courier New", a.settingsDraft.Font)
	if a.settingsDraft.UIFont != systemUIFont {
		fonts = append(fonts, a.settingsDraft.UIFont)
	}
	slices.Sort(fonts)
	a.settingsFonts = slices.Compact(fonts)
	a.settingsUIFonts = append([]string{systemUIFont}, a.settingsFonts...)
}

var defaultInterfaceSize = ui.LightTheme().FontSize
var interfaceSizes = []string{"11", "12", "13", "14", "15", "16", "18"}
var terminalSizes = []string{"11", "12", "13", "14", "15", "16", "18", "20", "24"}

func (p preferences) withDefaults() preferences {
	p.Appearance = normalizeAppearance(p.Appearance)
	if p.Language != "en" {
		p.Language = "zh-Hans"
	}
	if strings.TrimSpace(p.Font) == "" {
		p.Font = "Menlo"
	}
	if strings.TrimSpace(p.UIFont) == "" {
		p.UIFont = systemUIFont
	}
	if !slices.Contains(interfaceSizes, p.UIFontSize) {
		p.UIFontSize = strconv.Itoa(int(defaultInterfaceSize))
	}
	if !slices.Contains(terminalSizes, p.Size) {
		p.Size = "13"
	}
	for i, action := range shortcutActions {
		if !validShortcut(p.Shortcuts[i]) {
			p.Shortcuts[i] = action.defaultChord
		}
	}
	return p
}

func loadPreferences(path string) (preferences, error) {
	var p preferences
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p.withDefaults(), nil
	}
	if err != nil {
		return p.withDefaults(), err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return preferences{}.withDefaults(), err
	}
	return p.withDefaults(), nil
}

func (p preferences) terminalFont() terminal.Font {
	p = p.withDefaults()
	size, _ := strconv.Atoi(p.Size)
	return terminal.Font{Family: p.Font, Size: float32(size), Weight: fontWeight(p.TerminalBold)}
}

func (a *localTerm) openSettings() {
	a.settingsDraft = a.preferences.withDefaults()
	a.settingsSearch = ""
	a.settingsFonts, a.settingsUIFonts = nil, nil
	a.settingsOpen, a.settingsTab, a.recordingShortcut = true, settingsAppearanceTab, 0
}

func (a *localTerm) saveSettings() {
	if a.settingsPath == "" {
		a.settingsError = a.tr("无法访问设置存储目录。", "Settings storage is unavailable.")
		return
	}
	next := a.settingsDraft.withDefaults()
	for i, chord := range next.Shortcuts {
		if chord != "—" && slices.Contains(next.Shortcuts[:i], chord) {
			a.settingsTab, a.settingsSearch = settingsShortcutsTab, ""
			a.settingsError = a.tr("快捷键重复：", "Shortcut already assigned: ") + chord
			return
		}
	}
	if err := saveJSON(a.settingsPath, next); err != nil {
		a.settingsError = a.tr("无法保存设置：", "Could not save settings: ") + err.Error()
		return
	}
	appearanceChanged := normalizeAppearance(a.preferences.Appearance) != next.Appearance
	a.preferences = next
	if appearanceChanged && a.win != nil {
		mygo.Theme.SetSource(next.Appearance)
	}
	a.settingsError = ""
	a.closeSettings()
	for _, s := range a.tabs {
		if s.term != nil {
			s.term.SetFont(next.terminalFont())
		}
	}
}

func (a *localTerm) closeSettings() {
	a.settingsOpen = false
	a.focusTab = a.active != nil
}

func (a *localTerm) settingsView(c *ui.Context) {
	if a.recordingShortcut == 0 && c.Shortcut(0, ui.KeyEscape) {
		a.closeSettings()
		return
	}
	pages := [...]struct {
		title, icon, keywords string
		draw                  func(*ui.Context)
	}{
		settingsAppearanceTab: {a.tr("外观", "Appearance"), "◐", "语言 启动 主题 配色 深色 浅色 字体 字号 加粗 终端 language launch theme color dark light font size bold terminal", a.settingsAppearance},
		settingsShortcutsTab:  {a.tr("快捷键", "Shortcuts"), "⌘", "按键 键盘 录入 keyboard keys binding", a.settingsShortcuts},
		settingsHostsTab:      {a.tr("SSH 主机", "SSH hosts"), "↗", "连接 远程 remote connection", a.settingsHosts},
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.consoleTheme.Background).Children(func() {
		query := strings.ToLower(strings.TrimSpace(a.settingsSearch))
		matches := func(title, keywords string) bool {
			return strings.Contains(strings.ToLower(title+" "+keywords), query)
		}
		firstMatch := -1
		ui.Column(c).Width(192).Shrink(0).Padding(16, 12).Gap(6).Background(c.Theme().Background).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Margin(0, 0, 6).Children(func() {
				ui.Text(c, a.tr("系统设置", "Settings")).FontSize(interfaceTextSize(c, 13)).Bold()
				label := a.tr("返回", "Back")
				button := ui.ButtonBase(c).Height(26).PaddingX(6).Gap(4).Center().Radius(5).
					Label(label).OnClick(a.closeSettings)
				if button.Hovered() {
					button.Background(c.Theme().SurfaceHover)
				}
				button.Children(func() {
					ui.Text(c, "←").FontSize(interfaceTextSize(c, 15)).TextColor(c.Theme().TextMuted)
					ui.Text(c, label).FontSize(interfaceTextSize(c, 12)).TextColor(c.Theme().TextMuted)
				})
			})
			ui.TextInput(c, &a.settingsSearch).Label(a.tr("搜索设置", "Search settings")).
				Placeholder(a.tr("搜索设置…", "Search settings…")).MinWidth(0).FillWidth()
			ui.Box(c).Height(4)
			for i, page := range pages {
				if matches(page.title, page.keywords) {
					firstMatch = i
					break
				}
			}
			if firstMatch >= 0 && !matches(pages[a.settingsTab].title, pages[a.settingsTab].keywords) {
				a.settingsTab, a.recordingShortcut = firstMatch, 0
			}
			for i, page := range pages {
				if !matches(page.title, page.keywords) {
					continue
				}
				selected := a.settingsTab == i
				button := ui.ButtonBase(c.Key(i)).Height(36).FillWidth().PaddingX(10).
					Justify(ui.Start).AlignItems(ui.Center).Gap(9).Radius(5).
					Role(ui.RoleTab).Label(page.title).Selected(selected).Background(ui.Transparent).
					OnClick(func() { a.settingsTab, a.recordingShortcut = i, 0 })
				labelColor := c.Theme().Text
				if selected {
					button.Background(c.Theme().Selection)
					labelColor = c.Theme().Accent
				} else if button.Hovered() {
					button.Background(c.Theme().SurfaceHover)
				}
				button.Children(func() {
					ui.Text(c, page.icon).Width(19).FontSize(interfaceTextSize(c, 15)).TextColor(c.Theme().Accent)
					ui.Text(c, page.title).SingleLine().FontSize(interfaceTextSize(c, 13)).TextColor(labelColor)
				})
			}
		})
		ui.Box(c).Width(1).Shrink(0).Background(c.Theme().Border)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Height(55).Shrink(0).PaddingX(24).AlignItems(ui.Center).Gap(12).Children(func() {
				if firstMatch >= 0 {
					page := pages[a.settingsTab]
					ui.Text(c, page.icon).FontSize(interfaceTextSize(c, 18)).TextColor(c.Theme().Accent)
					ui.Text(c, page.title).FontSize(interfaceTextSize(c, 19)).Bold()
				}
			})
			ui.Box(c).Height(1).Shrink(0).FillWidth().Background(c.Theme().Border)
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c.Key(a.settingsTab)).FillWidth().Padding(16, 24).Gap(10).Children(func() {
					if firstMatch < 0 {
						ui.Text(c, a.tr("没有匹配的设置。", "No matching settings.")).TextColor(c.Theme().TextMuted)
						return
					}
					pages[a.settingsTab].draw(c)
				})
			})
			if a.settingsError != "" {
				ui.Text(c, a.settingsError).TextColor(c.Theme().Danger).Padding(6, 24)
			}
			if a.settingsTab != settingsHostsTab {
				ui.Box(c).Height(1).Shrink(0).FillWidth().Background(c.Theme().Border)
				ui.Row(c).Height(52).Shrink(0).PaddingX(24).AlignItems(ui.Center).Justify(ui.End).Gap(8).Children(func() {
					ui.Button(c, a.tr("取消", "Cancel")).OnClick(a.closeSettings)
					ui.PrimaryButton(c, a.tr("保存", "Save")).Disabled(a.recordingShortcut != 0).OnClick(a.saveSettings)
				})
			}
		})
	})
}

func (a *localTerm) settingsAppearance(c *ui.Context) {
	ui.Row(c).MinHeight(42).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, a.tr("界面语言", "Interface language"))
		ui.Spacer(c)
		for _, language := range []struct{ code, label string }{
			{"zh-Hans", "简体中文"}, {"en", "English"},
		} {
			b := ui.Button(c.Key(language.code), language.label).Selected(a.settingsDraft.Language == language.code).
				OnClick(func() { a.settingsDraft.Language = language.code })
			if a.settingsDraft.Language == language.code {
				b.Background(c.Theme().Selection).TextColor(c.Theme().Text)
			}
		}
	})
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	ui.Row(c).MinHeight(42).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, a.tr("应用启动", "On launch"))
			ui.Text(c, a.tr("启动后可手动新建终端", "You can open a terminal after launch")).
				FontSize(12).TextColor(c.Theme().TextMuted)
		})
		ui.Checkbox(c, &a.settingsDraft.SkipStartupTerminal, a.tr("仅打开主窗口", "Open only the main window"))
	})
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	ui.Row(c).MinHeight(46).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, a.tr("主题", "Theme"))
		ui.Spacer(c)
		for _, option := range []struct {
			source mygo.ThemeSource
			label  string
		}{
			{mygo.ThemeSystem, a.tr("跟随系统", "System")},
			{mygo.ThemeLight, a.tr("浅色", "Light")},
			{mygo.ThemeDark, a.tr("深色", "Dark")},
		} {
			selected := a.settingsDraft.Appearance == option.source
			button := ui.Button(c.Key(option.source), option.label).Selected(selected).
				OnClick(func() { a.settingsDraft.Appearance = option.source })
			if selected {
				button.Background(c.Theme().Selection).TextColor(c.Theme().Text)
			}
		}
	})
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	if a.settingsFonts == nil {
		a.loadFontOptions()
	}
	ui.Row(c).MinHeight(46).AlignItems(ui.Center).Gap(10).Children(func() {
		label := a.tr("界面字体", "Interface font")
		ui.Text(c, label).Grow(1).MinWidth(0)
		ui.Combobox(c.Key("interface-font"), &a.settingsDraft.UIFont, a.settingsUIFonts).Label(label).Width(168)
		ui.Select(c.Key("interface-size"), &a.settingsDraft.UIFontSize, interfaceSizes).
			Label(a.tr("界面字号", "Interface font size")).Width(68).MinWidth(0)
		ui.Checkbox(c, &a.settingsDraft.UIBold, a.tr("加粗", "Bold")).Label(a.tr("界面加粗", "Bold interface text"))
	})
	ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
	ui.Row(c).MinHeight(46).AlignItems(ui.Center).Gap(10).Children(func() {
		label := a.tr("终端字体", "Terminal font")
		ui.Text(c, label).Grow(1).MinWidth(0)
		ui.Combobox(c.Key("terminal-font"), &a.settingsDraft.Font, a.settingsFonts).Label(label).Width(168)
		ui.Select(c.Key("terminal-size"), &a.settingsDraft.Size, terminalSizes).
			Label(a.tr("终端字号", "Terminal font size")).Width(68).MinWidth(0)
		ui.Checkbox(c, &a.settingsDraft.TerminalBold, a.tr("加粗", "Bold")).Label(a.tr("终端加粗", "Bold terminal text"))
	})
	font := a.settingsDraft.terminalFont()
	ui.Text(c, a.tr("$ ls -la   终端预览", "$ ls -la   Terminal preview")).Font(font.Family).FontSize(font.Size).FontWeight(font.Weight).
		TextColor(a.consoleTheme.Foreground).Padding(8, 12).Background(a.consoleTheme.Background).Radius(5).FillWidth()
}

func (a *localTerm) settingsHosts(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, a.tr("已保存的主机", "Saved hosts")).FontSize(interfaceTextSize(c, 15)).Bold()
		ui.Spacer(c)
		ui.PrimaryButton(c, a.tr("新增主机", "Add host")).Disabled(a.storeError != nil).OnClick(func() {
			a.settingsOpen = false
			a.editor = &editor{entry: entry{Kind: host}, add: true, open: true, fixedKind: true, fromSettings: true}
		})
	})
	ui.Column(c).FillWidth().Children(func() {
		hasHosts := false
		for _, e := range a.entries {
			if e.Kind != host {
				continue
			}
			hasHosts = true
			key := c.Key(e.ID)
			ui.Row(key).Padding(8, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(key).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(key, e.Name).SingleLine().Bold()
					ui.Text(key, e.Host).SingleLine().TextColor(c.Theme().TextMuted)
				})
				ui.Button(key, a.tr("编辑主机", "Edit host")).Tooltip(e.Name).OnClick(func() {
					a.settingsOpen = false
					a.editor = &editor{entry: e, open: true, fromSettings: true}
				})
			})
		}
		if !hasHosts {
			ui.Text(c, a.tr("还没有 SSH 主机。", "No SSH hosts yet.")).TextColor(c.Theme().TextMuted)
		}
	})
}

func (a *localTerm) tr(chinese, english string) string {
	if a.preferences.Language == "en" {
		return english
	}
	return chinese
}
