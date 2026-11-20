package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (a *localTerm) tabMenu(m *ui.Menu, s *session) {
	index := slices.Index(a.tabs, s)
	if index < 0 {
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

type tabTitleForm struct {
	session *session
	title   string
	open    bool
}

func (a *localTerm) terminalMenu(services ui.Services, m *ui.Menu, s *session, selection string) {
	m.Separator()
	a.quickCommandsMenu(m, s, selection)
	if m.Item(a.tr("复制终端输出", "Copy Terminal Output")).Chosen() {
		if output := s.term.Text(); output != "" {
			services.WriteClipboard(output)
		}
	}
	m.Separator()
	if m.Item(a.tr("设置页签标题…", "Set Tab Title…")).Chosen() {
		a.tabTitle = &tabTitleForm{session: s, title: s.name, open: true}
	}
	if m.Item(a.tr("关闭当前页签", "Close Current Tab")).Chosen() {
		a.closeTab(s)
	}
}

func (a *localTerm) tabTitleView(c *ui.Context, form *tabTitleForm) {
	c = c.Key(form)
	title := a.tr("设置页签标题", "Set Tab Title")
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
