package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type shortcutAction struct {
	zh, en, defaultChord string
}

const closeTabShortcut = 1

var shortcutActions = [...]shortcutAction{
	{"新建本地终端", "New local terminal", "⌘T"},
	{"关闭当前页签", "Close current tab", "⌘W"},
	{"切换侧栏", "Toggle sidebar", "⌘⇧B"},
	{"打开系统设置", "Open settings", "⌘,"},
}

// App shortcuts require Command so ordinary terminal input stays available.
func shortcutKey(chord string) (ui.Modifiers, ui.Key) {
	tail, ok := strings.CutPrefix(chord, "⌘")
	if !ok {
		return 0, ui.KeyUnknown
	}
	mods := ui.Super
	for _, modifier := range []struct {
		glyph string
		flag  ui.Modifiers
	}{{"⌥", ui.Alt}, {"⇧", ui.Shift}} {
		if rest, found := strings.CutPrefix(tail, modifier.glyph); found {
			tail, mods = rest, mods|modifier.flag
		}
	}
	if tail == "," {
		return mods, ui.KeyComma
	}
	if len(tail) == 1 && tail[0] >= 'A' && tail[0] <= 'Z' &&
		(mods != ui.Super || !strings.Contains("ACHMQVXZ", tail)) {
		return mods, ui.KeyA + ui.Key(tail[0]-'A')
	}
	return 0, ui.KeyUnknown
}

func validShortcut(chord string) bool {
	_, key := shortcutKey(chord)
	return chord == "—" || key != ui.KeyUnknown
}

func recordedShortcut(mods ui.Modifiers, key ui.Key) string {
	if mods&ui.Super == 0 || mods & ^(ui.Super|ui.Alt|ui.Shift) != 0 {
		return ""
	}
	chord := "⌘"
	if mods&ui.Alt != 0 {
		chord += "⌥"
	}
	if mods&ui.Shift != 0 {
		chord += "⇧"
	}
	switch {
	case key == ui.KeyComma:
		chord += ","
	case key >= ui.KeyA && key <= ui.KeyZ:
		chord += string(rune('A' + key - ui.KeyA))
	default:
		return ""
	}
	if validShortcut(chord) {
		return chord
	}
	return ""
}

func (a *localTerm) shortcutBinding(index int) (ui.Modifiers, ui.Key) {
	chord := a.preferences.Shortcuts[index]
	if !validShortcut(chord) {
		chord = shortcutActions[index].defaultChord
	}
	return shortcutKey(chord)
}

func (a *localTerm) handleShortcuts(c *ui.Context) {
	if a.settingsOpen || a.confirm || a.editor != nil && a.editor.open ||
		a.tabTitle != nil && a.tabTitle.open || a.quickCommandForm != nil && a.quickCommandForm.open {
		return
	}
	actions := [...]func(){
		func() { a.openLocal(c) },
		func() {
			if a.active != nil {
				a.closeTab(a.active)
			}
		},
		a.toggleSidebar,
		a.openSettings,
	}
	for i, run := range actions {
		mods, key := a.shortcutBinding(i)
		if key != ui.KeyUnknown && c.Shortcut(mods, key) {
			run()
			return
		}
	}
}

func (a *localTerm) settingsShortcuts(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.tr("点击键位录入，Esc 取消，Delete 清除。", "Click a binding to record. Esc cancels, Delete clears.")).
			FontSize(12).TextColor(c.Theme().TextMuted).MinWidth(0)
		ui.Spacer(c)
		ui.Button(c, a.tr("恢复默认", "Reset defaults")).OnClick(func() {
			for i, action := range shortcutActions {
				a.settingsDraft.Shortcuts[i] = action.defaultChord
			}
			a.recordingShortcut, a.settingsError = 0, ""
		})
	})
	for i, action := range shortcutActions {
		label := a.tr(action.zh, action.en)
		ui.Row(c.Key(i)).Height(34).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, label)
			ui.Spacer(c)
			recording := a.recordingShortcut == i+1
			title := a.settingsDraft.Shortcuts[i]
			if recording {
				title = a.tr("请按组合键…", "Press shortcut…")
			}
			button := ui.Button(c, title).MinWidth(112).OnClick(func() {
				a.recordingShortcut, a.settingsError = i+1, ""
			})
			if recording {
				button.Background(c.Theme().Selection)
			}
			button.HandleInput(func(ev ui.InputEvent) bool {
				if a.recordingShortcut != i+1 || ev.Kind != ui.InputKeyDown {
					return false
				}
				if ev.Mods == 0 && ev.Key == ui.KeyEscape {
					a.recordingShortcut, a.settingsError = 0, ""
					return true
				}
				chord := recordedShortcut(ev.Mods, ev.Key)
				if ev.Mods == 0 && (ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete) {
					chord = "—"
				}
				if chord == "" {
					a.settingsError = a.tr("请输入带 ⌘ 的字母或逗号组合键。", "Use Command with a letter or comma.")
					return true
				}
				for j, assigned := range a.settingsDraft.Shortcuts {
					if j != i && chord != "—" && assigned == chord {
						other := shortcutActions[j]
						a.settingsError = a.tr("快捷键已用于", "Shortcut already used by ") + a.tr(other.zh, other.en)
						return true
					}
				}
				a.settingsDraft.Shortcuts[i] = chord
				a.recordingShortcut, a.settingsError = 0, ""
				return true
			})
		})
	}
}
