package main

import (
	"bytes"
	"fmt"
	"image/color"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func click(t *testing.T, view *ui.Tester, label string) {
	t.Helper()
	if err := view.Click(label); err != nil {
		t.Fatalf("%v (visible: %v)", err, view.Texts())
	}
}

func TestConnectionTreeDrag(t *testing.T) {
	app := &localTerm{
		storePath: filepath.Join(t.TempDir(), "connections.json"),
		sidebar:   270,
		expanded:  make(map[string]*bool),
		entries: []entry{
			{ID: "outer", Kind: category, Name: "Outer group"},
			{ID: "inner", Parent: "outer", Kind: category, Name: "Inner group"},
			{ID: "first", Kind: host, Name: "First host", Host: "first"},
			{ID: "second", Kind: host, Name: "Second host", Host: "second"},
		},
	}
	view := ui.NewTester(app.view, 720, 530)
	find := func(label string) ui.Rect {
		t.Helper()
		r, ok := view.Find(label)
		if !ok {
			t.Fatalf("missing %q in %v", label, view.Texts())
		}
		return r
	}
	drag := func(label string, x, y float32, cancel bool) {
		from := find(label)
		view.Press(from.X+from.W/2, from.Y+from.H/2)
		view.Move(x, y)
		if cancel {
			view.Key(0, ui.KeyEscape)
		}
		view.Release(x, y)
	}
	first, second := find("First host"), find("Second host")
	drag("Second host", first.X+8, first.Y+1, true)
	if find("First host").Y != first.Y || find("Second host").Y != second.Y {
		t.Fatal("canceling a drag changed the visible order")
	}
	drag("Second host", first.X+8, first.Y+1, false)
	if find("Second host").Y >= find("First host").Y {
		t.Fatal("dragging above a sibling did not reorder the tree")
	}

	inner := find("Inner group")
	drag("Outer group", inner.X+8, inner.Y+inner.H/2, false)
	if app.entries[entryIndex(app.entries, "outer")].Parent != "" {
		t.Fatal("a group moved into its own descendant")
	}
	drag("Second host", inner.X+8, inner.Y+inner.H/2, false)
	if app.entries[entryIndex(app.entries, "second")].Parent != "inner" {
		t.Fatal("dropping onto a group did not move the host into it")
	}

	settings := find("设置")
	if settings.X > 70 || settings.Y < 480 {
		t.Fatalf("settings button is outside the bottom left: %v", settings)
	}
	drag("Second host", 100, settings.Y-30, false)
	saved, err := loadEntries(app.storePath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved, app.entries) || saved[len(saved)-1].ID != "second" || saved[len(saved)-1].Parent != "" {
		t.Fatalf("expected the host at the end of the root: %+v", saved)
	}
}

func TestConnectionTreeUI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")
	app := &localTerm{storePath: path, sidebar: 270, expanded: make(map[string]*bool)}
	view := ui.NewTester(app.view, 1080, 650)

	click(t, view, "＋ 新建")
	view.Type("运维")
	click(t, view, "保存")
	if len(app.entries) != 1 || app.entries[0].Name != "运维" {
		t.Fatalf("category was not added: %+v", app.entries)
	}
	group := app.entries[0]
	click(t, view, "＋ 新建")
	click(t, view, "远程主机")
	view.Type("测试机")
	view.Key(0, ui.KeyTab)
	view.Type("dev")
	for range 3 {
		view.Key(0, ui.KeyTab)
	}
	view.Type("tmux attach")
	click(t, view, "保存")
	if len(app.entries) != 2 || app.entries[1].Parent != group.ID || app.entries[1].Host != "dev" {
		t.Fatalf("host was not added under category: %+v, form = %+v, visible = %v", app.entries, app.editor, view.Texts())
	}
	click(t, view, "更多操作")
	if err := view.ChooseMenuItem("编辑"); err != nil {
		t.Fatal(err)
	}
	view.Key(ui.Super, ui.KeyA)
	view.Type("内网主机")
	click(t, view, "保存")
	saved, err := loadEntries(path)
	if err != nil || len(saved) != 2 || saved[1].Name != "内网主机" || saved[1].StartupCommand != "tmux attach" || saved[1].Parent != group.ID {
		t.Fatalf("edited tree did not persist: %+v, %v", saved, err)
	}
	click(t, view, "＋ 新建")
	view.Type("子分组")
	click(t, view, "保存")
	if len(app.entries) != 3 || app.entries[2].Kind != category || app.entries[2].Parent != group.ID {
		t.Fatalf("nested category was not saved: %+v", app.entries)
	}
	click(t, view, "＋ 新建")
	click(t, view, "本地工作目录")
	view.Type("本机目录")
	view.Key(0, ui.KeyTab)
	workdir := t.TempDir()
	view.Type(workdir)
	view.Key(0, ui.KeyTab)
	view.Key(0, ui.KeyTab)
	view.Type("pwd")
	click(t, view, "放在根层")
	click(t, view, "保存")
	if len(app.entries) != 4 || app.entries[3].Parent != "" || app.entries[3].Path != workdir || app.entries[3].StartupCommand != "pwd" {
		t.Fatalf("root work directory was not saved: %+v, form = %+v", app.entries, app.editor)
	}
	deleteGroup := func() {
		t.Helper()
		if err := view.RightClick("运维"); err != nil {
			t.Fatal(err)
		}
		if err := view.ChooseMenuItem("删除"); err != nil {
			t.Fatal(err)
		}
	}
	deleteGroup()
	if !app.confirm || len(app.entries) != 4 {
		t.Fatal("nonempty category should require confirmation")
	}
	click(t, view, "取消")
	deleteGroup()
	view.Key(0, ui.KeyTab)
	view.Key(0, ui.KeyEnter)
	saved, err = loadEntries(path)
	if err != nil || len(saved) != 1 || saved[0].Kind != local {
		t.Fatalf("deleting a category should only remove its own subtree: %+v, %v", saved, err)
	}
}

func TestSSHCommand(t *testing.T) {
	remote, err := (entry{Kind: host, Name: "测试机", Host: "[::1]", User: "ops", Port: "02222"}).validated()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/ssh", "-tt", "-l", "ops", "-p", "2222", "--", "::1"}
	if got := remote.sshCommand(); !slices.Equal(got, want) {
		t.Fatalf("SSH argv = %q, want %q", got, want)
	}
	remote.StartupCommand = `printf '%s\n' "$HOME"; exit 7`
	if got := remote.sshCommand(); !slices.Equal(got, append(want, remote.StartupCommand)) {
		t.Fatalf("SSH argv did not preserve the remote shell command: %q", got)
	}
	invalid := remote
	invalid.StartupCommand = "pwd\x00"
	if _, err := invalid.validated(); err == nil {
		t.Fatal("accepted a startup command containing NUL")
	}
	for _, dest := range []string{"-F", "dev example.org", "user@dev", "dev\nother"} {
		remote.Host = dest
		if _, err := remote.validated(); err == nil {
			t.Errorf("accepted invalid destination %q", dest)
		}
	}
}

func TestLocalStartupCommand(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("set MYGO_GHOSTTY_VT to a local Ghostty dynamic library to run the PTY test")
	}
	t.Setenv("SHELL", "/bin/sh")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALTERM_STARTUP_EXPECTED_DIR", dir)
	term, err := newTerminal(entry{
		Kind: local, Name: "Startup test", Path: dir,
		StartupCommand: `test "$(pwd -P)" = "$LOCALTERM_STARTUP_EXPECTED_DIR" && printf 'startup-ok\n' && exit 7`,
	}, terminal.Font{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("startup command did not exit")
	}
	output := term.Snapshot()
	if term.ExitCode() != 7 || !bytes.Contains(output, []byte("startup-ok")) {
		t.Fatalf("startup command result = %d, output = %q", term.ExitCode(), output)
	}
}

func TestLocalTerminalTabs(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("set MYGO_GHOSTTY_VT to a local Ghostty dynamic library to run the PTY test")
	}
	t.Setenv("SHELL", "/bin/sh")
	dir := t.TempDir()
	app := &localTerm{
		sidebar: 270, expanded: make(map[string]*bool),
		entries: []entry{{ID: "work", Kind: local, Name: "工作目录", Path: dir}},
	}
	t.Cleanup(app.closeSessions)
	view := ui.NewTester(app.view, 1080, 650)

	click(t, view, "工作目录")
	click(t, view, "工作目录")
	if len(app.tabs) != 1 {
		t.Fatalf("double click opened %d tabs, want 1", len(app.tabs))
	}
	first := app.active
	view.Type("pwd; exit 7")
	view.Key(0, ui.KeyEnter)
	select {
	case <-first.term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("shell did not exit")
	}
	view.Frame()
	output := first.term.Snapshot()
	// macOS may report the physical path of a temporary directory.
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.term.ExitCode() != 7 || (!bytes.Contains(output, []byte(dir)) && !bytes.Contains(output, []byte(physical))) {
		t.Fatalf("unexpected shell result %d: %q", first.term.ExitCode(), output)
	}
	if !view.HasText("退出码 7") || len(app.tabs) != 1 {
		t.Fatal("exited shell output should stay in its tab")
	}
	time.Sleep(600 * time.Millisecond)
	click(t, view, "工作目录")
	click(t, view, "工作目录")
	if len(app.tabs) != 2 || app.active == first {
		t.Fatal("double clicking the same entry should open a new, independent shell")
	}
	second := app.active
	closeFirst, ok := view.Find("关闭 工作目录")
	if !ok {
		t.Fatal("first tab close button was not rendered")
	}
	view.ClickAt(closeFirst.X-12, closeFirst.Y+closeFirst.H/2)
	if app.active != first {
		t.Fatal("clicking the first tab did not switch to it")
	}
	click(t, view, "关闭 工作目录")
	if app.active != second || len(app.tabs) != 1 {
		t.Fatal("closing the active tab did not select the remaining tab")
	}
	click(t, view, "关闭 工作目录")
	if app.active != nil || len(app.tabs) != 0 {
		t.Fatal("closing the last tab did not restore the empty state")
	}
	select {
	case <-second.term.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("closing the tab did not end the session")
	}
}

func TestSettingsUI(t *testing.T) {
	dir := t.TempDir()
	app := &localTerm{
		sidebar: 270, expanded: make(map[string]*bool),
		storePath:    filepath.Join(dir, "connections.json"),
		settingsPath: filepath.Join(dir, "settings.json"),
		tabs:         []*session{{name: "Font size sample"}},
	}
	view := ui.NewTester(app.view, 720, 530)
	before, ok := view.Find("Font size sample")
	if !ok {
		t.Fatal("sample tab was not rendered")
	}
	click(t, view, "设置")
	if !view.HasText("界面语言") || !view.HasText("终端字体") || view.HasText("常规") {
		t.Fatal("settings should open to the merged appearance page")
	}
	click(t, view, "English")
	click(t, view, "快捷键")
	click(t, view, "搜索设置")
	view.Type("语言")
	if !view.HasText("界面语言") || !view.HasText("终端字体") || view.HasText("快捷键") {
		t.Fatal("language search should open the merged appearance page")
	}
	pickFont := func(label, font string, x float32) {
		t.Helper()
		row, ok := view.Find(label)
		if !ok {
			t.Fatalf("missing font row %q", label)
		}
		view.ClickAt(x, row.Y+row.H/2)
		view.Key(ui.Super, ui.KeyA)
		view.Type(font)
		view.Key(0, ui.KeyDown)
		view.Key(0, ui.KeyEnter)
	}
	pickFont("界面字体", "Courier New", 465)
	pickFont("终端字体", "Monaco", 465)
	click(t, view, "界面字号")
	click(t, view, "15")
	click(t, view, "界面加粗")
	click(t, view, "终端字号")
	click(t, view, "16")
	click(t, view, "终端加粗")
	click(t, view, "保存")
	saved, err := loadPreferences(app.settingsPath)
	if err != nil || saved.Language != "en" || saved.UIFont != "Courier New" || saved.Font != "Monaco" || saved.Size != "16" || saved.UIFontSize != "15" || !saved.UIBold || !saved.TerminalBold {
		t.Fatalf("saved preferences = %+v, err = %v", saved, err)
	}
	if font := saved.terminalFont(); font.Size != 16 || font.Weight != 700 {
		t.Fatalf("terminal font = %+v", font)
	}
	if after, ok := view.Find("Font size sample"); !ok || after.W <= before.W {
		t.Fatalf("tab text did not grow after saving the interface font: before=%v after=%v", before, after)
	}
	if !view.HasText("Connections") {
		t.Fatal("sidebar did not switch to English")
	}
	click(t, view, "Settings")
	click(t, view, "SSH hosts")
	click(t, view, "Add host")
	view.Type("Dev box")
	view.Key(0, ui.KeyTab)
	view.Type("dev")
	click(t, view, "Save")
	if len(app.entries) != 1 || app.entries[0].Kind != host || app.entries[0].Host != "dev" {
		t.Fatalf("unexpected settings host: %+v", app.entries)
	}
	if !app.settingsOpen || !view.HasText("Saved hosts") {
		t.Fatal("did not return to the hosts page")
	}
	click(t, view, "Edit host")
	view.Key(ui.Super, ui.KeyA)
	view.Type("Build box")
	click(t, view, "Save")
	entries, err := loadEntries(app.storePath)
	if err != nil || len(entries) != 1 || entries[0].Name != "Build box" || entries[0].Host != "dev" {
		t.Fatalf("edited host = %+v, err = %v", entries, err)
	}
}

func TestStartupTerminalAndSidebarMenu(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("set MYGO_GHOSTTY_VT to a local Ghostty dynamic library to run the PTY test")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHELL", "/bin/sh")
	updates := make(chan func(), 1)
	app := &localTerm{
		sidebar: 270, openOnStart: true,
		update: func(fn func()) { updates <- fn },
	}
	t.Cleanup(app.closeSessions)
	view := ui.NewTester(app.view, 720, 530)
	finishLaunch := func() {
		t.Helper()
		select {
		case apply := <-updates:
			apply()
			view.Frame()
		case <-time.After(5 * time.Second):
			t.Fatal("terminal initialization did not complete")
		}
	}
	first := app.active
	if first == nil || first.kind != local || len(app.tabs) != 1 || first.term != nil {
		t.Fatal("startup should show one pending local terminal")
	}
	finishLaunch()
	if app.active != first || first.term == nil {
		t.Fatal("startup should attach the terminal to its pending tab")
	}
	view.RightClickAt(140, 200)
	if err := view.ChooseMenuItem("打开本地终端"); err != nil {
		t.Fatal(err)
	}
	active := app.active
	finishLaunch()
	if len(app.tabs) != 2 || active == first || app.active != active || active.term == nil {
		t.Fatal("sidebar menu should open a separate local terminal")
	}
	view.RightClickAt(140, 200)
	if err := view.ChooseMenuItem("隐藏侧栏"); err != nil {
		t.Fatal(err)
	}
	if !app.sidebarHidden || view.HasText("连接") {
		t.Fatal("sidebar should be hidden")
	}
	click(t, view, "显示侧栏")
	if app.sidebarHidden || !view.HasText("连接") || app.sidebar != 270 {
		t.Fatal("sidebar should return at its previous width")
	}
	click(t, view, "隐藏侧栏")
	if !app.sidebarHidden || view.HasText("连接") {
		t.Fatal("the toolbar button should hide the sidebar")
	}
	click(t, view, "显示侧栏")
	if len(app.tabs) != 2 || app.active != active {
		t.Fatal("toggling the sidebar should preserve the sessions and active tab")
	}
}

func TestTypedCreationMenus(t *testing.T) {
	app := &localTerm{sidebar: 270, expanded: make(map[string]*bool),
		entries: []entry{{ID: "team", Kind: category, Name: "运维分组"}},
	}
	view := ui.NewTester(app.view, 720, 530)
	for _, option := range []struct {
		label string
		kind  entryKind
	}{
		{"新建分组", category},
		{"新建连接", host},
		{"新建本地工作目录", local},
	} {
		view.RightClickAt(140, 200)
		if err := view.ChooseMenuItem(option.label); err != nil {
			t.Fatal(err)
		}
		if form := app.editor; form == nil || !form.open || !form.fixedKind || form.entry.Kind != option.kind || form.entry.Parent != "" {
			t.Fatalf("%s opened the wrong form: %+v", option.label, form)
		}
		click(t, view, "取消")
	}
	if err := view.RightClick("运维分组"); err != nil {
		t.Fatal(err)
	}
	if err := view.ChooseMenuItem("新建连接"); err != nil {
		t.Fatal(err)
	}
	if app.editor.entry.Kind != host || app.editor.entry.Parent != "team" {
		t.Fatalf("new connection has the wrong group: %+v", app.editor.entry)
	}
}

func TestBackgroundDirectorySaveAndTerminalLaunch(t *testing.T) {
	updates := make(chan func(), 2)
	app := &localTerm{
		sidebar: 270, storePath: filepath.Join(t.TempDir(), "connections.json"),
		update: func(fn func()) { updates <- fn },
	}
	finish := func() {
		t.Helper()
		select {
		case apply := <-updates:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatal("background operation did not complete")
		}
	}

	dir := t.TempDir()
	app.editor = &editor{entry: entry{Kind: local, Name: "工作目录", Path: dir}, add: true, open: true}
	view := ui.NewTester(app.view, 720, 530)
	click(t, view, "保存")
	if len(app.entries) != 0 || !app.editor.open || !view.HasText("正在保存…") {
		t.Fatal("the editor should remain open while the save is pending")
	}
	view.Key(0, ui.KeyEscape)
	if !app.editor.open {
		t.Fatal("Escape should not dismiss a save in progress")
	}
	finish()
	if len(app.entries) != 1 || app.editor.open || app.entries[0].Path != dir {
		t.Fatal("the saved directory should appear after the result is applied")
	}

	// A missing directory makes initialization fail without requiring Ghostty.
	app.launch(entry{Kind: local, Name: "不存在", Path: filepath.Join(dir, "missing")})
	view.Frame()
	if len(app.tabs) != 1 || !view.HasText("正在启动终端…") {
		t.Fatal("launch should show a tab immediately")
	}
	click(t, view, "关闭 不存在")
	finish()
	if len(app.tabs) != 0 || app.active != nil || len(app.terminalErrors) != 0 {
		t.Fatal("completion should leave a closed pending tab closed")
	}
}

func TestQuickCommandsUI(t *testing.T) {
	if os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("set MYGO_GHOSTTY_VT to test terminal selection and paste")
	}
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	term, err := terminal.New(terminal.Options{Conn: conn})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()

	s := &session{name: "本地终端", term: term}
	app := &localTerm{
		settingsPath:  filepath.Join(t.TempDir(), "settings.json"),
		tabs:          []*session{s},
		active:        s,
		sidebarHidden: true,
	}
	view := ui.NewTester(app.view, 800, 530)
	rightClick := func() {
		t.Helper()
		if err := view.RightClick("Terminal"); err != nil {
			t.Fatal(err)
		}
	}
	choose := func(path ...string) {
		t.Helper()
		if err := view.ChooseMenuItem(path...); err != nil {
			t.Fatal(err)
		}
	}

	rightClick()
	if err := view.ChooseMenuItem("快捷命令", "将选中内容添加为快捷命令…"); err == nil {
		t.Fatal("selection action was enabled without selected text")
	}
	choose("快捷命令", "手动添加快捷命令…")
	view.Type("看目录")
	view.Key(0, ui.KeyTab)
	view.Type("pwd")
	click(t, view, "保存")

	term.Feed([]byte("git status"))
	view.Frame()
	rightClick()
	choose("全选")
	rightClick()
	choose("快捷命令", "将选中内容添加为快捷命令…")
	if form := app.quickCommandForm; form == nil || form.Command != "git status" {
		t.Fatalf("command form did not receive the terminal selection: %+v", form)
	}
	view.Type("查看 Git 状态")
	click(t, view, "保存")
	saved, err := loadPreferences(app.settingsPath)
	want := []quickCommand{{Name: "看目录", Command: "pwd"}, {Name: "查看 Git 状态", Command: "git status"}}
	if err != nil || !slices.Equal(saved.QuickCommands, want) {
		t.Fatalf("saved commands = %+v, error = %v", saved.QuickCommands, err)
	}
	app.preferences = saved

	rightClick()
	choose("快捷命令", "查看 Git 状态")
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	n, err := peer.Read(buf)
	if err != nil || string(buf[:n]) != "git status" {
		t.Fatalf("terminal received %q, error = %v", buf[:n], err)
	}

	rightClick()
	wantMenu := []string{"复制", "粘贴", "全选", "-", "快捷命令", "复制终端输出", "-", "设置页签标题…", "关闭当前页签", "-", "清屏", "清除滚动历史"}
	if got := view.Menu(); !slices.Equal(got, wantMenu) {
		t.Fatalf("Chinese terminal menu = %q, want %q", got, wantMenu)
	}
	choose("复制终端输出")
	if got := strings.TrimSpace(view.Clipboard()); got != "git status" {
		t.Fatalf("copied terminal output = %q", got)
	}

	rightClick()
	choose("设置页签标题…")
	view.Key(ui.Super, ui.KeyA)
	view.Type("开发终端")
	click(t, view, "保存")
	if s.name != "开发终端" || !view.HasText("开发终端") {
		t.Fatalf("tab title was not updated: %q", s.name)
	}

	term.Feed([]byte("\r\n$ "))
	rightClick()
	choose("清屏")
	if got := strings.TrimSpace(term.Text()); got != "$" {
		t.Fatalf("clearing should keep the prompt line, got %q", got)
	}

	app.preferences.Language = "en"
	view.Frame()
	rightClick()
	wantMenu = []string{"Copy", "Paste", "Select All", "-", "Quick Commands", "Copy Terminal Output", "-", "Set Tab Title…", "Close Current Tab", "-", "Clear Screen", "Clear Scrollback"}
	if got := view.Menu(); !slices.Equal(got, wantMenu) {
		t.Fatalf("English terminal menu = %q, want %q", got, wantMenu)
	}
	choose("Close Current Tab")
	if app.active != nil || len(app.tabs) != 0 {
		t.Fatal("terminal menu did not close the current tab")
	}
}

func TestShortcutSettingsUI(t *testing.T) {
	app := &localTerm{sidebar: 270, settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	view := ui.NewTester(app.view, 720, 530)

	view.Key(ui.Super, ui.KeyComma)
	if !app.settingsOpen {
		t.Fatal("default shortcut did not open settings")
	}
	click(t, view, "仅打开主窗口")
	click(t, view, "快捷键")
	click(t, view, "⌘W")
	view.Key(0, ui.KeyEscape)
	if !app.settingsOpen || app.recordingShortcut != 0 || app.settingsDraft.Shortcuts[1] != "⌘W" {
		t.Fatal("Escape should cancel recording and leave settings open")
	}
	click(t, view, "⌘W")
	view.Key(0, ui.KeyBackspace)
	if app.settingsDraft.Shortcuts[1] != "—" {
		t.Fatal("Backspace should clear the shortcut")
	}
	click(t, view, "⌘⇧B")
	view.Key(ui.Super, ui.KeyT)
	if !view.HasText("快捷键已用于新建本地终端") {
		t.Fatal("shortcut conflict should name the action already using it")
	}
	view.Key(ui.Super|ui.Shift, ui.KeyS)
	if app.settingsDraft.Shortcuts[2] != "⌘⇧S" {
		t.Fatalf("shortcut picker did not update the draft: %v", app.settingsDraft.Shortcuts)
	}
	app.settingsDraft.Shortcuts[0] = "⌘⇧S"
	click(t, view, "保存")
	if !app.settingsOpen || !view.HasText("快捷键重复：⌘⇧S") {
		t.Fatal("duplicate shortcut was not rejected")
	}
	app.settingsDraft.Shortcuts[0] = "⌘T"
	click(t, view, "保存")

	saved, err := loadPreferences(app.settingsPath)
	if err != nil || !saved.SkipStartupTerminal || saved.Shortcuts[2] != "⌘⇧S" {
		t.Fatalf("settings were not saved: %+v, %v", saved, err)
	}
	view.Key(ui.Super|ui.Shift, ui.KeyB)
	if app.sidebarHidden {
		t.Fatal("old sidebar shortcut still triggered")
	}
	view.Key(ui.Super|ui.Shift, ui.KeyS)
	if !app.sidebarHidden {
		t.Fatal("assigned shortcut did not toggle the sidebar")
	}
}

func TestAppearanceSettingsUI(t *testing.T) {
	app := &localTerm{sidebar: 270, settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	view := ui.NewTester(app.view, 720, 530)

	pixel := func(x, y int, want ui.Color) {
		t.Helper()
		if got := view.Image().RGBAAt(x, y); got != (color.RGBA{R: want.R, G: want.G, B: want.B, A: want.A}) {
			t.Errorf("pixel (%d, %d) = %v, want %v", x, y, got, want)
		}
	}
	checkPalette := func(p *palette) {
		t.Helper()
		pixel(100, 400, p.chrome.Background)
		pixel(600, 400, p.console.Background)
	}

	checkPalette(lightPalette)
	view.SetDark(true)
	checkPalette(darkPalette)

	view.Key(ui.Super, ui.KeyComma)
	click(t, view, "浅色")
	click(t, view, "取消")
	checkPalette(darkPalette)

	view.Key(ui.Super, ui.KeyComma)
	click(t, view, "浅色")
	click(t, view, "保存")
	checkPalette(lightPalette)
	saved, err := loadPreferences(app.settingsPath)
	if err != nil || saved.Appearance != mygo.ThemeLight {
		t.Fatalf("saved appearance = %q, err = %v", saved.Appearance, err)
	}
}

func TestTabContextMenu(t *testing.T) {
	pinned := &session{name: "Pinned", entryID: "project"}
	left := &session{name: "Left", entryID: "project"}
	anchor := &session{name: "Anchor", entryID: "project"}
	right := &session{name: "Right", entryID: "project"}
	hidden := &session{name: "Other project"}
	app := &localTerm{
		entries: []entry{{ID: "project", Name: "Project", Kind: local}},
		tabs:    []*session{left, anchor, hidden, right, pinned}, active: left,
		projectView: true, projectID: "project", sidebarHidden: true,
	}
	view := ui.NewTester(app.view, 800, 530)
	choose := func(tab, action string) {
		t.Helper()
		if err := view.RightClick(tab); err != nil {
			t.Fatal(err)
		}
		if err := view.ChooseMenuItem(action); err != nil {
			t.Fatal(err)
		}
	}
	choose("Pinned", "固定页签")
	if !pinned.pinned || app.tabs[0] != pinned || app.active != left {
		t.Fatal("pinning should move the tab to the front and preserve the active tab")
	}
	choose("Anchor", "关闭左侧页签")
	if !slices.Equal(app.tabs, []*session{pinned, anchor, hidden, right}) || app.active != anchor {
		t.Fatal("closing tabs to the left should keep the pinned tab and activate the anchor")
	}
	choose("Anchor", "关闭右侧页签")
	if !slices.Equal(app.tabs, []*session{pinned, anchor, hidden}) {
		t.Fatal("closing tabs to the right should preserve tabs from other projects")
	}
	choose("Pinned", "取消固定页签")
	if pinned.pinned {
		t.Fatal("unpinning should clear the pinned state")
	}
	choose("Pinned", "关闭此页签")
	if !slices.Equal(app.tabs, []*session{anchor, hidden}) || app.active != anchor {
		t.Fatal("the context menu should close the clicked tab and preserve the active tab")
	}
	view.Key(ui.Super, ui.KeyW)
	if !slices.Equal(app.tabs, []*session{hidden}) || app.active != nil {
		t.Fatal("Command-W should close the active tab")
	}
}

func TestThemedTabStates(t *testing.T) {
	first, second := &session{name: "First"}, &session{name: "Second"}
	app := &localTerm{tabs: []*session{first, second}, active: first}
	view := ui.NewTester(func(c *ui.Context) {
		app.applyTheme(c)
		ui.Row(c).Height(36).Background(c.Theme().Background).Children(func() {
			app.tabHeader(c.Key(first), first)
			app.tabHeader(c.Key(second), second)
		})
	}, 320, 80)
	view.SetDark(true)
	countColor := func(want ui.Color) int {
		img := view.Image()
		count := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if img.RGBAAt(x, y) == (color.RGBA{R: want.R, G: want.G, B: want.B, A: want.A}) {
					count++
				}
			}
		}
		return count
	}
	if pixels := countColor(darkPalette.chrome.Focus); pixels == 0 {
		t.Error("selected tab underline was not rendered")
	}
	r, ok := view.Find("Second")
	if !ok {
		t.Fatal("inactive tab was not rendered")
	}
	view.Move(r.X+r.W/2, r.Y+r.H/2)
	if countColor(darkPalette.chrome.SurfaceHover) == 0 {
		t.Error("hovered tab background was not rendered")
	}

	before := countColor(darkPalette.chrome.Focus)
	view.Key(0, ui.KeyTab)
	if !view.Focused("First") || countColor(darkPalette.chrome.Focus) <= before {
		t.Errorf("keyboard focus ring was not rendered on the selected tab: focused=%v, before=%d, after=%d", view.Focused("First"), before, countColor(darkPalette.chrome.Focus))
	}
}

func TestProjectViewUI(t *testing.T) {
	general := &session{name: "General shell"}
	first := &session{entryID: "first", name: "First shell"}
	second := &session{entryID: "second", name: "Second shell"}
	remote := &session{entryID: "remote", name: "Remote shell"}
	app := &localTerm{
		sidebar: 270, expanded: make(map[string]*bool),
		storePath: filepath.Join(t.TempDir(), "connections.json"),
		entries: []entry{
			{ID: "first", Kind: local, Name: "First directory"},
			{ID: "second", Kind: local, Name: "Second directory"},
			{ID: "remote", Kind: host, Name: "Server", Host: "example.test"},
		},
		tabs: []*session{general, first, second, remote}, active: first,
	}
	view := ui.NewTester(app.view, 800, 530)
	visible := func(label string, want bool) {
		t.Helper()
		if _, found := view.Find(label); found != want {
			t.Errorf("%q visible = %v, want %v", label, found, want)
		}
	}
	click(t, view, "项目视图")
	visible("连接", false)
	visible("First directory", true)
	visible("First shell", true)
	visible("Second shell", false)
	visible("Server", false)

	railTab, railFound := view.Find("终端")
	shellTab, shellFound := view.Find("First shell")
	if !railFound || !shellFound || railTab.Y < 32 || railTab.Y > 44 || shellTab.Y < 32 || shellTab.Y > 40 {
		t.Fatalf("project rail and terminal tabs must start below the header: rail=%+v, terminal=%+v", railTab, shellTab)
	}
	img := view.Image()
	border := lightPalette.chrome.Border
	wantBorder := color.RGBA{R: border.R, G: border.G, B: border.B, A: border.A}
	for _, divider := range []struct {
		name string
		x, y int
	}{
		{"window header", 400, 32},
		{"project rail", 54, 500},
		{"terminal tabs", 400, 69},
	} {
		if got := img.RGBAAt(divider.x, divider.y); got != wantBorder {
			t.Errorf("%s divider has color %v, want %v", divider.name, got, wantBorder)
		}
	}
	console := lightPalette.console.Background
	if got := img.RGBAAt(400, 500); got != (color.RGBA{R: console.R, G: console.G, B: console.B, A: console.A}) {
		t.Errorf("terminal background must extend to the bottom of the project view, got %v", got)
	}

	checkTileColor := func(label string, want ui.Color) {
		t.Helper()
		r, ok := view.Find(label)
		if !ok {
			t.Fatalf("missing project tab %q", label)
		}
		got := view.Image().RGBAAt(int(r.X+8), int(r.Y+8))
		if got != (color.RGBA{R: want.R, G: want.G, B: want.B, A: want.A}) {
			t.Errorf("%q background = %v, want %v", label, got, want)
		}
	}
	checkTileColor("First directory", lightPalette.chrome.Selection)
	if r, ok := view.Find("Second directory"); ok {
		view.Move(r.X+r.W/2, r.Y+r.H/2)
	}
	checkTileColor("Second directory", lightPalette.chrome.SurfaceHover)
	for i := 0; i < 10 && !view.Focused("Second directory"); i++ {
		view.Key(0, ui.KeyTab)
	}
	if !view.Focused("Second directory") {
		t.Fatal("project tab is not reachable by keyboard")
	}
	view.Key(0, ui.KeyEnter)
	visible("First shell", false)
	visible("Second shell", true)
	if app.active != second {
		t.Fatal("switching directories did not restore the existing session")
	}
	click(t, view, "关闭 Second shell")
	if app.active != nil || app.projectID != "second" || len(app.tabs) != 3 {
		t.Fatal("closing a project's last tab changed projects or closed another session")
	}
	click(t, view, "添加项目")
	if err := view.ChooseMenuItem("添加本地工作目录…"); err != nil {
		t.Fatal(err)
	}
	if app.editor == nil || app.editor.entry.Kind != local || !app.editor.addToProjects {
		t.Fatal("adding a local directory did not open the project form")
	}
	click(t, view, "取消")
	click(t, view, "添加项目")
	if err := view.ChooseMenuItem("新建连接配置…"); err != nil {
		t.Fatal(err)
	}
	if app.editor == nil || app.editor.entry.Kind != host || !app.editor.entry.InProjects {
		t.Fatal("creating an SSH configuration did not open the project form")
	}
	click(t, view, "取消")
	click(t, view, "添加项目")
	if err := view.ChooseMenuItem("添加已有连接配置", "Server · example.test"); err != nil {
		t.Fatal(err)
	}
	visible("Server", true)
	visible("Remote shell", true)
	if app.active != remote {
		t.Fatal("adding an existing configuration did not restore its session")
	}
	entries, err := loadEntries(app.storePath)
	if err != nil || len(entries) != 3 || !entries[2].InProjects {
		t.Fatalf("joining an existing configuration was not saved: %+v, %v", entries, err)
	}
	if err := view.RightClick("Server"); err != nil {
		t.Fatal(err)
	}
	if err := view.ChooseMenuItem("编辑配置"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7 && !view.Focused("项目页签文字"); i++ {
		view.Key(0, ui.KeyTab)
	}
	if !view.Focused("项目页签文字") {
		t.Fatal("project title field is not reachable by keyboard")
	}
	view.Type("Deploy")
	view.Key(0, ui.KeyTab)
	if !view.Focused("项目图标") {
		t.Fatal("project icon field does not follow the title field")
	}
	view.Type("🚀")
	click(t, view, "保存")
	visible("Deploy", true)
	visible("🚀", true)
	visible("Remote shell", true)
	entries, err = loadEntries(app.storePath)
	if err != nil || len(entries) != 3 || entries[2].Name != "Server" || entries[2].ProjectTitle != "Deploy" || entries[2].ProjectIcon != "🚀" {
		t.Fatalf("project appearance was not saved separately from the connection name: %+v, %v", entries, err)
	}
	click(t, view, "项目视图")
	visible("连接", true)
	visible("General shell", true)
	visible("First shell", true)
	visible("Remote shell", true)
}

func TestProjectRailScrollKeepsSettingsVisible(t *testing.T) {
	app := &localTerm{projectView: true}
	for i := range 25 {
		name := fmt.Sprintf("Project %02d", i)
		app.entries = append(app.entries, entry{ID: name, Kind: local, Name: name})
	}
	view := ui.NewTester(app.view, 800, 530)
	settings, ok := view.Find("设置")
	if !ok || settings.X >= 54 || settings.Y+settings.H != 520 {
		t.Fatalf("settings should be in the project footer with bottom padding, got %+v", settings)
	}
	view.Scroll(26, 100, 0, 2000)
	if after, ok := view.Find("设置"); !ok || after != settings {
		t.Fatalf("settings moved while scrolling: before=%+v, after=%+v", settings, after)
	}
	add, ok := view.Find("添加项目")
	if !ok || add.Y < 32 || add.Y+add.H >= settings.Y {
		t.Fatalf("the add button should scroll into view above settings, got %+v", add)
	}
	click(t, view, "设置")
	if !app.settingsOpen {
		t.Fatal("settings did not open from the project footer")
	}
	title, titleFound := view.Find("系统设置")
	back, backFound := view.Find("返回")
	if !titleFound || !backFound || back.X < title.X+title.W || back.X+back.W > 192 {
		t.Fatalf("back button must sit next to the settings title: title=%+v, back=%+v", title, back)
	}
	click(t, view, "返回")
	if app.settingsOpen || !app.projectView {
		t.Fatal("back did not return to the project view")
	}
}
