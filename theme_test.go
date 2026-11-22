package main

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestThemeStylePreview(t *testing.T) {
	app := &localTerm{settingsPath: filepath.Join(t.TempDir(), "settings.json"), sidebar: 270}
	view := ui.NewTester(app.view, 800, 530)

	view.Key(ui.Super, ui.KeyComma)
	click(t, view, "主题")
	click(t, view, "海蓝")
	if app.consoleTheme != &appearanceStyle("ocean").light.console {
		t.Fatal("theme was not previewed")
	}
	click(t, view, "取消")
	if app.consoleTheme != &lightPalette.console {
		t.Fatal("cancel did not restore the active theme")
	}

	view.Key(ui.Super, ui.KeyComma)
	click(t, view, "主题")
	click(t, view, "松林")
	click(t, view, "保存")
	p, err := loadPreferences(app.settingsPath)
	if err != nil || p.ThemeStyle != "forest" {
		t.Fatalf("theme was not saved: %+v, %v", p, err)
	}
	view.SetDark(true)
	if app.consoleTheme != &appearanceStyle("forest").dark.console {
		t.Fatal("saved theme did not follow the system's dark appearance")
	}
}
