package main

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *localTerm) tabMenu(m *ui.Menu, s *session) {
	index := slices.Index(a.tabs, s)
	if index < 0 {
		return
	}
	if m.Item(a.tr("重命名页签…", "Rename Tab…")).Chosen() {
		a.openTabTitle(s)
		return
	}
	pinLabel := a.tr("固定页签", "Pin Tab")
	if s.pinned {
		pinLabel = a.tr("取消固定页签", "Unpin Tab")
	}
	if m.Item(pinLabel).Chosen() {
		a.tabs = slices.Delete(a.tabs, index, index+1)
		boundary := 0
		for boundary < len(a.tabs) && a.tabs[boundary].pinned {
			boundary++
		}
		s.pinned = !s.pinned
		a.tabs = slices.Insert(a.tabs, boundary, s)
		if a.active == s {
			a.focusTab = true
		}
		return
	}
	if m.Item(a.tr("设置页签颜色…", "Set Tab Color…")).Chosen() {
		a.tabColorPicker = &tabColorPicker{session: s, open: true}
		return
	}
	m.Separator()
	if a.closeSideMenuItem(m, a.tr("关闭左侧页签", "Close Tabs to the Left"), s, a.tabs[:index]) ||
		a.closeSideMenuItem(m, a.tr("关闭右侧页签", "Close Tabs to the Right"), s, a.tabs[index+1:]) {
		return
	}
	closeItem := m.Item(a.tr("关闭此页签", "Close This Tab"))
	if mods, key := a.shortcutBinding(closeTabShortcut); key != ui.KeyUnknown {
		closeItem.Shortcut(mods, key)
	}
	if closeItem.Chosen() {
		a.closeTab(s)
	}
}

func (a *localTerm) closeSideMenuItem(m *ui.Menu, label string, anchor *session, side []*session) bool {
	var targets []*session
	for _, tab := range side {
		if !tab.pinned && (!a.projectView || a.sessionProject(tab) == a.projectID) {
			targets = append(targets, tab)
		}
	}
	if !m.Item(label).Disabled(len(targets) == 0).Chosen() {
		return false
	}
	if slices.Contains(targets, a.active) {
		a.activate(anchor)
	}
	for _, tab := range targets {
		a.closeTab(tab)
	}
	return true
}

type tabColorPicker struct {
	session *session
	open    bool
}

func (a *localTerm) tabColorView(c *ui.Context, picker *tabColorPicker) {
	c = c.Key(picker)
	ui.Modal(c, &picker.open, func() {
		ui.Column(c).Width(260).Gap(4).Children(func() {
			ui.Text(c, a.tr("设置页签颜色", "Set Tab Color")).Bold().Margin(0, 0, 8)
			for _, option := range []struct {
				chinese, english string
				color            ui.Color
			}{
				{"默认", "Default", ui.Transparent},
				{"红色", "Red", ui.Hex("#D35C69")},
				{"橙色", "Orange", ui.Hex("#CF873B")},
				{"绿色", "Green", ui.Hex("#579B68")},
				{"青色", "Teal", ui.Hex("#429D9E")},
				{"蓝色", "Blue", ui.Hex("#588BD4")},
				{"紫色", "Purple", ui.Hex("#9674D1")},
			} {
				label := a.tr(option.chinese, option.english)
				selected := picker.session.tabColor == option.color
				button := ui.ButtonBase(c).FillWidth().Height(30).PaddingX(8).Gap(8).
					AlignItems(ui.Center).Label(label).Selected(selected).Background(ui.Transparent).
					OnClick(func() {
						picker.session.tabColor = option.color
						picker.open = false
					})
				if button.Hovered() {
					button.Background(c.Theme().SurfaceHover)
				}
				button.Children(func() {
					swatch := ui.Box(c).Size(14, 14).Radius(3).Background(option.color)
					if option.color == ui.Transparent {
						swatch.Border(1, c.Theme().TextMuted)
					}
					ui.Text(c, label)
					if selected {
						ui.Spacer(c)
						ui.Text(c, "✓")
					}
				})
			}
			ui.Button(c, a.tr("取消", "Cancel")).OnClick(func() { picker.open = false })
		})
	})
}

type tabTitleForm struct {
	session *session
	title   string
	open    bool
}

func (a *localTerm) openTabTitle(s *session) {
	a.tabTitle = &tabTitleForm{session: s, title: s.name, open: true}
}

func (a *localTerm) terminalMenu(c *ui.Context, m *ui.Menu, s *session, selection string) {
	m.Separator()
	a.quickCommandsMenu(m, s, selection)
	if s.kind == local && m.Item(a.tr("将当前路径添加到本地工作目录…", "Add Current Path to Local Directories…")).
		Disabled(a.savingEntries || a.storeError != nil).Chosen() {
		path := s.term.ProcessDir()
		if !filepath.IsAbs(path) {
			c.Toast(a.tr("无法获取当前工作目录", "Could not get the current working directory"))
			return
		}
		target := a.selectedEntry()
		if a.minimalView {
			target = entry{}
		}
		a.newEntry(local, target, true)
		a.editor.entry.Name = filepath.Base(path)
		a.editor.entry.Path = path
	}
	if m.Item(a.tr("复制终端输出", "Copy Terminal Output")).Chosen() {
		if output := s.term.Text(); output != "" {
			c.Services().WriteClipboard(output)
		}
	}
	m.Separator()
	if m.Item(a.tr("设置页签标题…", "Set Tab Title…")).Chosen() {
		a.openTabTitle(s)
	}
	if m.Item(a.tr("关闭当前页签", "Close Current Tab")).Chosen() {
		a.closeTab(s)
	}
}

func (a *localTerm) tabTitleView(c *ui.Context, form *tabTitleForm) {
	c = c.Key(form)
	title := a.tr("重命名页签", "Rename Tab")
	ui.Modal(c, &form.open, func() {
		ui.Column(c).Width(340).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(interfaceTextSize(c, 20)).Bold()
			formInput(c, a.tr("页签标题", "Tab title"), &form.title, a.tr("输入新标题", "Enter a new title")).AutoFocus()
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, a.tr("取消", "Cancel")).OnClick(func() { form.open = false })
				ui.PrimaryButton(c, a.tr("保存", "Save")).Disabled(strings.TrimSpace(form.title) == "").OnClick(func() {
					form.session.name = strings.TrimSpace(form.title)
					form.open = false
				})
			})
		})
	})
}
