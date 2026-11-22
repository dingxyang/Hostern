package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (e entry) isProject() bool {
	return e.Kind == local && !e.HiddenInProjects || e.Kind == host && e.InProjects
}

func (a *localTerm) sessionProject(s *session) string {
	if a.entryByID(s.entryID).isProject() {
		return s.entryID
	}
	return ""
}

func (a *localTerm) projectSession(id string) *session {
	for i := len(a.tabs) - 1; i >= 0; i-- {
		if a.sessionProject(a.tabs[i]) == id {
			return a.tabs[i]
		}
	}
	return nil
}

func (a *localTerm) toggleProjects() {
	a.projectView = !a.projectView
	a.syncProject()
	if !a.projectView && a.active == nil && len(a.tabs) > 0 {
		a.activate(a.tabs[len(a.tabs)-1])
	}
	a.focusTab = a.active != nil
}

func (a *localTerm) syncProject() {
	if !a.projectView {
		return
	}
	if a.active != nil {
		a.projectID = a.sessionProject(a.active)
	} else if !a.entryByID(a.projectID).isProject() {
		a.projectID = ""
		a.activate(a.projectSession(""))
	}
}

func (a *localTerm) openProject(e entry) {
	if a.projectID == e.ID && a.active != nil {
		a.activate(a.active)
		return
	}
	a.projectID = e.ID
	if tab := a.projectSession(e.ID); tab != nil {
		a.activate(tab)
	} else if e.ID != "" {
		a.launch(e)
	} else {
		a.activate(nil)
	}
}

func (a *localTerm) setProjectVisible(c *ui.Context, e entry, visible bool) {
	if a.savingEntries {
		return
	}
	i := entryIndex(a.entries, e.ID)
	if i < 0 {
		return
	}
	next := slices.Clone(a.entries)
	switch next[i].Kind {
	case local:
		next[i].HiddenInProjects = !visible
	case host:
		next[i].InProjects = visible
	default:
		return
	}
	if err := a.setEntries(next); err != nil {
		c.Toast(a.tr("无法保存连接树：", "Could not save connections: ") + err.Error())
		return
	}
	if visible {
		a.openProject(next[i])
	} else {
		a.syncProject()
	}
}

func (a *localTerm) projects(c *ui.Context) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(54).Shrink(0).Background(c.Theme().Background).Children(func() {
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Padding(6, 0, 8).Gap(4).AlignItems(ui.Center).Children(func() {
					a.projectTab(c.Key("general"), entry{Name: a.tr("终端", "Terminals"), Kind: host})
					for _, e := range a.entries {
						if e.isProject() {
							a.projectTab(c.Key(struct{ ID string }{e.ID}), e)
						}
					}
					label := a.tr("添加项目", "Add project")
					iconButton(c.Key("add"), "＋", label).Size(36, 34).Margin(4, 0, 0).
						Border(1, c.Theme().Border).Menu(func(m *ui.Menu) { a.addProjectMenu(m, c) })
				})
			})
			ui.Column(c).Shrink(0).Padding(0, 8, 10).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
				iconButton(c.Key("settings"), "⚙", a.tr("设置", "Settings")).Size(36, 32).OnClick(a.openSettings)
			})
		})
		ui.Box(c).Width(1).Shrink(0).Background(c.Theme().Border)
		ui.Box(c).Grow(1).MinWidth(0).Children(func() { a.terminals(c) })
	})
}

func (a *localTerm) projectTab(c *ui.Context, e entry) {
	selected := a.projectID == e.ID
	title := e.ProjectTitle
	if title == "" {
		title = e.Name
	}
	label := title
	target := e.Path
	if e.Kind == host {
		target = e.Host
	}
	if target != "" {
		label += " · " + target
	}
	theme := c.Theme()
	tile := ui.ButtonBase(c).Size(36, 36).Center().Radius(7).Role(ui.RoleTab).
		Label(title).Tooltip(label).Selected(selected).Background(theme.Surface).
		OnClick(func() { a.openProject(e) })
	color := theme.TextMuted
	if selected {
		tile.Background(theme.Selection)
		color = theme.Text
	} else if tile.Hovered() {
		tile.Background(theme.SurfaceHover)
	}
	tile.Children(func() {
		if selected {
			ui.Box(c).Absolute().Left(0).Top(8).Bottom(8).Width(2).Background(theme.Focus)
		}
		if e.projectBitmap != nil {
			ui.Image(c, e.projectBitmap).Size(24, 24).Fit(ui.Contain)
		} else if e.ProjectIcon != "" {
			ui.Text(c, e.ProjectIcon).SingleLine().MaxWidth(32).FontSize(interfaceTextSize(c, 16)).TextColor(color)
		} else if e.ID != "" {
			initials := []rune(strings.TrimSpace(title))
			ui.Text(c, string(initials[:min(2, len(initials))])).FontSize(interfaceTextSize(c, 12)).Bold().TextColor(color)
		} else {
			connectionIcon(c, host, color)
		}
	})
	if e.ID != "" {
		tile.ContextMenu(func(m *ui.Menu) {
			if m.Item(a.tr("编辑配置", "Edit configuration")).Chosen() {
				a.editor = &editor{entry: e, open: true}
			}
			if m.Item(a.tr("关闭项目页签", "Close project tab")).Disabled(a.savingEntries).Chosen() {
				a.setProjectVisible(c, e, false)
			}
		})
	}
}

func (a *localTerm) addProjectMenu(m *ui.Menu, c *ui.Context) {
	disabled := a.storeError != nil || a.savingEntries
	for _, item := range []struct {
		kind  entryKind
		label string
	}{
		{local, a.tr("添加本地工作目录…", "Add local directory…")},
		{host, a.tr("新建连接配置…", "New connection configuration…")},
	} {
		if m.Item(item.label).Disabled(disabled).Chosen() {
			a.newEntry(item.kind, entry{}, true)
			a.editor.addToProjects = true
			a.editor.entry.InProjects = item.kind == host
		}
	}
	available := func(e entry) bool { return e.Kind == host && !e.InProjects }
	m.Submenu(a.tr("添加已有连接配置", "Add existing connection"), func(sub *ui.Menu) {
		for _, e := range a.entries {
			if available(e) && sub.Item(e.Name+" · "+e.Host).Chosen() {
				a.setProjectVisible(c, e, true)
			}
		}
	}).Disabled(disabled || !slices.ContainsFunc(a.entries, available))
	hiddenDirectory := func(e entry) bool { return e.Kind == local && e.HiddenInProjects }
	m.Submenu(a.tr("恢复本地工作目录", "Restore local directory"), func(sub *ui.Menu) {
		for _, e := range a.entries {
			if hiddenDirectory(e) && sub.Item(e.Name+" · "+e.Path).Chosen() {
				a.setProjectVisible(c, e, true)
			}
		}
	}).Disabled(disabled || !slices.ContainsFunc(a.entries, hiddenDirectory))
}
