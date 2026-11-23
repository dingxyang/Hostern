package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo/ui"
)

func (a *localTerm) view(c *ui.Context) {
	a.applyTheme(c)
	if a.openOnStart {
		a.openOnStart = false
		if !a.preferences.SkipStartupTerminal {
			a.openLocal(c)
		}
	}
	for _, message := range a.terminalErrors {
		c.Toast(message)
	}
	a.terminalErrors = nil
	a.handleShortcuts(c)
	ui.Column(c).Fill().Children(func() {
		if a.settingsOpen {
			a.settingsToolbar(c)
		} else {
			a.windowToolbar(c)
		}
		ui.Box(c).Height(1).FillWidth().Shrink(0).Background(c.Theme().Border)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			if a.settingsOpen {
				a.settingsView(c)
			} else if a.projectView {
				a.projects(c)
			} else if a.sidebarHidden {
				a.terminals(c)
			} else {
				ui.Split(c, &a.sidebar, func() { a.connections(c) }, func() { a.terminals(c) }).Fill()
			}
		})
	})
	if a.editor != nil && a.editor.open {
		a.entryForm(c, a.editor)
	}
	if a.confirm {
		a.confirmDelete(c)
	}
	if picker := a.tabColorPicker; picker != nil && picker.open {
		a.tabColorView(c, picker)
	}
	if form := a.tabTitle; form != nil && form.open {
		a.tabTitleView(c, form)
	}
	if form := a.quickCommandForm; form != nil && form.open {
		a.quickCommandView(c, form)
	}
}

func (a *localTerm) windowToolbar(c *ui.Context) {
	bar := c.TitleBar()
	label := a.tr("隐藏侧栏", "Hide sidebar")
	if a.sidebarHidden || a.projectView {
		label = a.tr("显示侧栏", "Show sidebar")
	}
	const controlWidth = 30
	const projectButtonMargin = 4
	const leftPadding = 8
	controlsEnd := bar.Left + leftPadding + 2*controlWidth + 2*projectButtonMargin
	tabStart := controlsEnd
	if !a.projectView && !a.sidebarHidden {
		tabStart = max(tabStart, a.sidebar+1)
	}
	ui.Row(c).Height(max(bar.Height, 32)).Shrink(0).AlignItems(ui.Center).
		Padding(0, bar.Right+8, 0, bar.Left+leftPadding).Background(c.Theme().Background).DragWindow().Children(func() {
		button := ui.ButtonBase(c).Size(controlWidth, 26).Shrink(0).Radius(5).Label(label).Tooltip(label).OnClick(a.toggleSidebar)
		if button.Hovered() {
			button.Background(c.Theme().SurfaceHover)
		}
		button.Children(func() {
			color := c.Theme().TextMuted
			ui.Box(c).Size(18, 16).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, color, 3, 1.3)
				p.Line(r.X+6.5, r.Y+1.5, r.X+6.5, r.Y+r.H-1.5, 1.3, color)
			})
		})
		modeLabel := a.tr("项目视图", "Project view")
		theme := c.Theme()
		mode := ui.ButtonBase(c).Size(controlWidth, 26).Margin(0, projectButtonMargin).Shrink(0).Radius(5).Center().
			Label(modeLabel).Tooltip(modeLabel).Selected(a.projectView).Background(ui.Transparent).OnClick(a.toggleProjects)
		iconColor := theme.TextMuted
		if a.projectView {
			mode.Background(theme.Selection)
			iconColor = theme.Text
		} else if mode.Hovered() {
			mode.Background(theme.SurfaceHover)
		}
		mode.Children(func() {
			ui.Box(c).Size(18, 16).Draw(func(p *ui.Painter, r ui.Rect) {
				for _, offset := range []float32{1, 6.5, 12} {
					p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + offset, W: 3, H: 3}, iconColor, 0.8)
				}
				p.Stroke(ui.Rect{X: r.X + 7, Y: r.Y + 1, W: 10, H: 14}, iconColor, 2, 1.3)
			})
		})
		if tabStart > controlsEnd {
			ui.Box(c).Width(tabStart - controlsEnd).Shrink(0)
		}
		a.terminalTabs(c)
	})
}

func (a *localTerm) connections(c *ui.Context) {
	disabled := a.storeError != nil
	ui.Column(c).Fill().Background(c.Theme().Background).ContextMenu(func(m *ui.Menu) {
		if m.Item(a.tr("打开本地终端", "Open local terminal")).Chosen() {
			a.openLocal(c)
		}
		a.newEntryMenu(m, entry{}, disabled)
		m.Separator()
		if m.Item(a.tr("隐藏侧栏", "Hide sidebar")).Chosen() {
			a.toggleSidebar()
		}
	}).Children(func() {
		ui.Row(c).Height(44).Shrink(0).PaddingX(12).AlignItems(ui.Center).Gap(8).Children(func() {
			label := a.tr("搜索连接", "Search connections")
			ui.TextInput(c, &a.connectionSearch).Label(label).Placeholder(label).
				Grow(1).MinWidth(0).Disabled(disabled)
			iconButton(c, "＋", a.tr("＋ 新建", "＋ New")).Disabled(disabled).OnClick(a.addEntry)
		})
		if disabled {
			ui.Text(c, a.tr("无法读取连接树", "Could not load connections")).Bold().Margin(16, 14, 0)
			ui.Text(c, a.storeError.Error()).TextColor(c.Theme().Danger).Margin(6, 14, 14)
			ui.Spacer(c)
		} else {
			scroll := ui.Scroll(c).Grow(1).MinHeight(0).Padding(0, 10, 10)
			var drop connectionDrop
			scroll.Children(func() {
				children := make(map[string][]entry)
				for _, e := range a.filteredConnections() {
					children[e.Parent] = append(children[e.Parent], e)
				}
				if len(children[""]) == 0 && strings.TrimSpace(a.connectionSearch) != "" {
					ui.Text(c, a.tr("没有匹配的连接", "No matching connections")).TextColor(c.Theme().TextMuted).Margin(6)
				}
				ui.Tree(c, func() { a.treeItems(c, children, "", &drop) })
			})
			if !a.savingEntries {
				if source, ok := ui.Drop[connectionDrag](scroll); ok {
					drop = connectionDrop{source: source, position: moveInside}
				}
				if drop.source != "" {
					a.moveConnection(c, drop)
				}
			}
		}
		ui.Row(c).Height(44).Shrink(0).PaddingX(12).AlignItems(ui.Center).Children(func() {
			iconButton(c, "⚙", a.tr("设置", "Settings")).Size(30, 30).OnClick(a.openSettings)
		})
	})
}

func (a *localTerm) filteredConnections() []entry {
	query := strings.ToLower(strings.TrimSpace(a.connectionSearch))
	if query == "" {
		return a.entries
	}
	included := make(map[string]bool)
	for _, e := range a.entries {
		if strings.Contains(strings.ToLower(e.Name+" "+e.Host+" "+e.Path), query) {
			for id := e.ID; id != "" && !included[id]; id = a.entryByID(id).Parent {
				included[id] = true
			}
		}
	}
	var matches []entry
	for _, e := range a.entries {
		if included[e.ID] {
			matches = append(matches, e)
		}
	}
	return matches
}

func (a *localTerm) newEntryMenu(m *ui.Menu, target entry, disabled bool) {
	for _, item := range []struct {
		kind  entryKind
		label string
	}{
		{category, a.tr("新建分组", "New group")},
		{host, a.tr("新建连接", "New connection")},
		{local, a.tr("新建本地工作目录", "New local directory")},
	} {
		if m.Item(item.label).Disabled(disabled).Chosen() {
			a.newEntry(item.kind, target, true)
		}
	}
}

func iconButton(c *ui.Context, glyph, label string) ui.Element {
	button := ui.ButtonBase(c).Size(24, 24).Radius(5).Label(label).Tooltip(label)
	if button.Hovered() {
		button.Background(c.Theme().SurfaceHover)
	}
	button.Children(func() {
		ui.Text(c, glyph).FontSize(interfaceTextSize(c, 16)).TextColor(c.Theme().TextMuted)
	})
	return button
}

func (a *localTerm) treeItems(c *ui.Context, children map[string][]entry, parent string, drop *connectionDrop) {
	for _, e := range children[parent] {
		key := c.Key(e.ID)
		var row ui.Element
		var open *bool
		if e.Kind == category {
			open = a.expansion(e.ID)
			if strings.TrimSpace(a.connectionSearch) != "" {
				open = new(true)
			}
			row = ui.TreeItem(key, "", open, func() { a.treeItems(key, children, e.ID, drop) })
		} else {
			target := e.Path
			if e.Kind == host {
				target = e.Host
			}
			row = ui.TreeItem(key, "", nil, nil).Tooltip(target)
			if row.Shortcut(0, ui.KeyEnter) {
				a.launch(e)
			}
		}
		// MyGo reports a double-click on press, before it can distinguish a drag.
		if row.DoubleClicked() {
			a.connectionDoublePress = e.ID
		}
		clicked := row.Clicked()
		if a.connectionDoublePress == e.ID && (clicked || !row.Pressed()) {
			a.connectionDoublePress = ""
			if clicked {
				if open != nil {
					*open = !*open
				} else {
					a.launch(e)
				}
			}
		}
		if clicked {
			a.selectConnection(c, e.ID)
		}
		selected := e.ID == a.selected
		row.Radius(5).Selected(selected).Label(e.Name).Children(func() {
			iconColor := c.Theme().TextMuted
			if selected {
				iconColor = c.Theme().Text
			}
			connectionIcon(key, e.Kind, iconColor)
			ui.Text(key, e.Name).SingleLine().MinWidth(0)
		})
		if selected {
			row.Background(c.Theme().Selection).TextColor(c.Theme().Text)
		}
		if !a.savingEntries {
			row.Drag(connectionDrag(e.ID))
			source, dropped := ui.Drop[connectionDrag](row)
			if !dropped {
				source, _ = ui.DragOver[connectionDrag](row)
			}
			if source != "" {
				_, y, _ := row.PointerPosition()
				height := row.Bounds().H
				position := moveBefore
				switch {
				case e.Kind == category && y >= height/4 && y < 3*height/4:
					position = moveInside
				case y >= height/2:
					position = moveAfter
				}
				target := connectionDrop{source: source, target: e.ID, position: position}
				if dropped {
					*drop = target
				} else if _, _, ok := target.destination(a.entries); ok {
					row.DrawOver(func(p *ui.Painter, r ui.Rect) {
						color := c.Theme().Accent
						if position == moveInside {
							p.Stroke(r, color, 5, 1.5)
						} else {
							y := r.Y + 1
							if position == moveAfter {
								y = r.Y + r.H - 1
							}
							p.Line(r.X+2, y, r.X+r.W-2, y, 2, color)
						}
					})
				}
			}
		}
		menu := func(m *ui.Menu) {
			if e.Kind != category {
				if m.Item(a.tr("打开终端", "Open terminal")).Chosen() {
					a.launch(e)
				}
				if e.Kind == host && m.Item(a.tr("打开 SFTP", "Open SFTP")).Chosen() {
					a.launchSFTP(e)
				}
				m.Separator()
			}
			a.newEntryMenu(m, e, false)
			m.Separator()
			if m.Item(a.tr("编辑", "Edit")).Chosen() {
				a.editor = &editor{entry: e, open: true}
			}
			if m.Item(a.tr("删除", "Delete")).Disabled(a.savingEntries).Chosen() {
				a.requestDelete(c, e)
			}
		}
		row.ContextMenu(menu)
		if selected || row.Hovered() || row.Focused() {
			row.Children(func() {
				ui.Spacer(key)
				iconButton(key, "⋯", a.tr("更多操作", "More actions")).Menu(menu)
			})
		}
	}
}

func connectionIcon(c *ui.Context, kind entryKind, color ui.Color) {
	ui.Box(c).Size(16, 16).Shrink(0).Role(ui.RoleNone).Draw(func(p *ui.Painter, r ui.Rect) {
		x, y := r.X, r.Y
		const stroke = 1.25
		switch kind {
		case category:
			var folder ui.Path
			folder.MoveTo(x+1.5, y+13).LineTo(x+1.5, y+3).LineTo(x+6, y+3).
				LineTo(x+8, y+5).LineTo(x+14.5, y+5).LineTo(x+14.5, y+13).Close()
			p.StrokePath(&folder, stroke, color)
		case host:
			p.Stroke(ui.Rect{X: x + 1.5, Y: y + 2.5, W: 13, H: 11}, color, 2, stroke)
			var prompt ui.Path
			prompt.MoveTo(x+4.5, y+5.5).LineTo(x+7, y+8).LineTo(x+4.5, y+10.5)
			p.StrokePath(&prompt, stroke, color)
			p.Line(x+8.5, y+10.5, x+11.5, y+10.5, stroke, color)
		case local:
			p.Stroke(ui.Rect{X: x + 1.5, Y: y + 2.5, W: 13, H: 9}, color, 2, stroke)
			p.Line(x+8, y+11.5, x+8, y+14, stroke, color)
			p.Line(x+5, y+14, x+11, y+14, stroke, color)
		}
	})
}

func (a *localTerm) moveConnection(c *ui.Context, drop connectionDrop) {
	index, parent, ok := drop.destination(a.entries)
	if !ok {
		return
	}
	source := entryIndex(a.entries, string(drop.source))
	next := slices.Clone(a.entries)
	moved := next[source]
	moved.Parent = parent
	next = slices.Delete(next, source, source+1)
	if source < index {
		index--
	}
	next = slices.Insert(next, index, moved)
	if slices.Equal(next, a.entries) {
		return
	}
	if err := a.setEntries(next); err != nil {
		c.Toast(a.tr("无法保存连接树：", "Could not save connections: ") + err.Error())
		return
	}
	if drop.position == moveInside && parent != "" {
		*a.expansion(parent) = true
	}
}

const terminalTabContentHeight = 31

func (a *localTerm) terminalTabs(c *ui.Context) {
	ui.Row(c).Height(terminalTabContentHeight).Shrink(1).MinWidth(0).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).Children(func() {
			ui.Row(c).Height(terminalTabContentHeight).AlignItems(ui.Center).Gap(3).Children(func() {
				// Tab menus can reorder or close sessions while their headers are built.
				for _, s := range slices.Clone(a.tabs) {
					if !a.projectView || a.sessionProject(s) == a.projectID {
						a.tabHeader(c.Key(s), s)
					}
				}
			})
		})
		iconButton(c, "＋", a.tr("新建终端", "New terminal")).Shrink(0).
			Menu(func(m *ui.Menu) { a.newTerminalMenu(c, m) })
	})
}

func (a *localTerm) terminals(c *ui.Context) {
	ui.Column(c).Fill().Background(a.consoleTheme.Background).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).MinWidth(0).Clip().Children(func() {
			if a.active == nil {
				return
			}
			current := a.active
			if current.files != nil {
				a.fileBrowser(c.Key(current), current)
				return
			}
			if current.term == nil {
				ui.Text(c, a.tr("正在启动终端…", "Starting terminal…")).TextColor(c.Theme().TextMuted).Margin(12)
				return
			}
			view := terminal.ViewWithMenu(c.Key(current), current.term, a.tr, func(m *ui.Menu, selection string) {
				a.terminalMenu(c, m, current, selection)
			}).Grow(1).MinHeight(0)
			if a.focusTab {
				view.Focus()
				a.focusTab = false
			}
			select {
			case <-current.term.Done():
				ui.Text(c, a.tr("会话已结束 · 退出码 ", "Session ended · exit code ")+strconv.Itoa(current.term.ExitCode())).
					FontSize(11).TextColor(c.Theme().TextMuted).Padding(6, 12).Background(c.Theme().Surface).FillWidth()
			default:
			}
		})
	})
}

func (a *localTerm) tabHeader(c *ui.Context, s *session) {
	active := a.active == s
	theme := c.Theme()
	color := theme.TextMuted
	row := ui.Row(c).Height(terminalTabContentHeight).MaxWidth(200).Shrink(0).AlignItems(ui.Center).Padding(0, 4, 0, 0).
		ContextMenu(func(m *ui.Menu) { a.tabMenu(m, s) })
	if row.Hovered() || active {
		color = theme.Text
	}
	if active && a.focusTab {
		row.ScrollIntoView()
	}
	indicator := s.tabColor
	height := float32(1)
	if active {
		height = 2
		if indicator == ui.Transparent {
			indicator = theme.Focus
		}
	}
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		const inset = 7
		p.Fill(ui.Rect{X: r.X + r.W - 1, Y: r.Y + inset, W: 1, H: r.H - 2*inset}, theme.Border, 0)
		if indicator != ui.Transparent {
			const width = 24
			p.Fill(ui.Rect{X: r.X + (r.W-width)/2, Y: r.Y + r.H - height, W: width, H: height}, indicator, 0)
		}
	})
	row.Children(func() {
		tab := ui.ButtonBase(c).MinWidth(0).Height(terminalTabContentHeight-2).Padding(0, 4, 0, 8).
			AlignItems(ui.Center).Gap(5).Role(ui.RoleTab).Selected(active).Background(ui.Transparent).FocusRing(false).Label(s.name).Tooltip(s.name).
			OnClick(func() { a.activate(s) })
		tab.Children(func() {
			if tab.FocusVisible() {
				ui.Box(c).Absolute().Top(3).Bottom(3).Left(2).Right(1).Border(1, theme.Focus)
			}
			if s.pinned {
				ui.Box(c).Size(10, 12).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
					x, y := r.X, r.Y
					p.Line(x+2, y+2, x+8, y+2, 1, color)
					p.Line(x+3.5, y+2, x+2, y+7.5, 1, color)
					p.Line(x+6.5, y+2, x+8, y+7.5, 1, color)
					p.Line(x+2, y+7.5, x+8, y+7.5, 1, color)
					p.Line(x+5, y+7.5, x+5, y+11, 1, color)
				})
			}
			label := ui.Text(c, s.name).SingleLine().MinWidth(0).FontSize(interfaceTextSize(c, 12)).TextColor(color)
			if active {
				label.Bold()
			}
		})
		closeButton := ui.ButtonBase(c).Size(22, 22).Shrink(0).Center().Background(ui.Transparent).FocusRing(false).
			Label(a.tr("关闭 ", "Close ") + s.name).Tooltip(a.tr("关闭页签", "Close tab")).
			OnClick(func() { a.closeTab(s) })
		closeColor := theme.TextMuted
		if closeButton.Hovered() {
			closeColor = theme.Text
		}
		closeButton.Children(func() {
			if closeButton.FocusVisible() {
				ui.Box(c).Absolute().Top(1).Bottom(1).Left(1).Right(1).Border(1, theme.Focus)
			}
			ui.Box(c).Size(10, 10).Draw(func(p *ui.Painter, r ui.Rect) {
				var cross ui.Path
				cross.MoveTo(r.X+2, r.Y+2).LineTo(r.X+8, r.Y+8)
				cross.MoveTo(r.X+8, r.Y+2).LineTo(r.X+2, r.Y+8)
				p.StrokePath(&cross, 1.4, closeColor)
			})
		})
	})
}

func formInput(c *ui.Context, label string, value *string, hint string) ui.Element {
	ui.Text(c, label).FontSize(interfaceTextSize(c, 12)).Bold()
	return ui.TextInput(c, value).Label(label).Placeholder(hint).FillWidth()
}

func (a *localTerm) entryForm(c *ui.Context, form *editor) {
	open := form.open
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(390).MinHeight(0).Shrink(1).Gap(12).Disabled(a.savingEntries).Children(func() {
			title := a.tr("新建条目", "New connection")
			switch {
			case form.add && form.fixedKind:
				title = a.tr("新建", "New ") + a.kindLabel(form.entry.Kind)
			case !form.add:
				title = a.tr("编辑", "Edit ") + a.kindLabel(form.entry.Kind)
			}
			ui.Text(c, title).FontSize(interfaceTextSize(c, 20)).Bold().Margin(0, 0, 4)
			ui.Scroll(c).MinHeight(0).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Gap(12).Children(func() {
					if form.add && !form.fixedKind {
						ui.Row(c).Gap(6).Children(func() {
							for _, kind := range []entryKind{category, host, local} {
								b := ui.Button(c.Key(kind), a.kindLabel(kind)).OnClick(func() {
									form.entry.Kind, form.error, form.focusName = kind, "", true
								})
								if form.entry.Kind == kind {
									b.Selected(true).Background(c.Theme().Selection).TextColor(c.Theme().Text)
								}
							}
						})
					}
					if form.add && form.entry.Parent != "" {
						ui.Checkbox(c, &form.atRoot, a.tr("放在根层", "Add at top level"))
						if !form.atRoot {
							ui.Text(c, a.tr("添加到 ", "Add to ")+a.entryByID(form.entry.Parent).Name).
								TextColor(c.Theme().TextMuted).SingleLine()
						}
					}
					name := formInput(c, a.tr("名称", "Name"), &form.entry.Name, a.tr("显示在连接树中", "Shown in the sidebar")).AutoFocus()
					if form.focusName {
						name.Focus()
						form.focusName = false
					}
					switch form.entry.Kind {
					case host:
						formInput(c, a.tr("SSH 别名或主机地址", "SSH alias or host address"), &form.entry.Host, a.tr("例如 dev 或 server.example.com", "e.g. dev or server.example.com"))
						formInput(c, a.tr("用户名（可选）", "Username (optional)"), &form.entry.User, a.tr("留空时使用 SSH 配置", "Uses SSH config if blank"))
						formInput(c, a.tr("端口（可选）", "Port (optional)"), &form.entry.Port, a.tr("留空时使用 SSH 配置", "Uses SSH config if blank"))
					case local:
						ui.Text(c, a.tr("工作目录", "Working directory")).FontSize(interfaceTextSize(c, 12)).Bold()
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.TextInput(c, &form.entry.Path).Label(a.tr("工作目录", "Working directory")).Placeholder("/Users/…").Grow(1).MinWidth(0)
							browse := a.tr("选择…", "Browse…")
							if form.picking {
								browse = a.tr("正在打开…", "Opening…")
							}
							ui.Button(c, browse).Disabled(form.picking || a.savingEntries).OnClick(func() { a.pickDirectory(form) })
						})
					}
					if form.entry.Kind != category {
						hint := a.tr("在本地工作目录中执行", "Run in the local working directory")
						if form.entry.Kind == host {
							hint = a.tr("连接后在远程主机上执行", "Run on the remote host after connecting")
						}
						formInput(c, a.tr("启动命令（可选）", "Startup command (optional)"), &form.entry.StartupCommand, hint)
					}
					if form.entry.Kind == host {
						label := a.tr("SSH 私钥（可选）", "SSH private key (optional)")
						ui.Text(c, label).FontSize(interfaceTextSize(c, 12)).Bold()
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.TextInput(c, &form.entry.IdentityFile).Label(label).Grow(1).MinWidth(0).
								Placeholder(a.tr("留空时使用 SSH 默认认证", "Uses default SSH authentication if blank"))
							ui.Button(c, a.tr("从 ~/.ssh 选择", "Choose from ~/.ssh")).Menu(func(m *ui.Menu) {
								if m.Item(a.tr("使用默认认证", "Use default authentication")).Checked(form.entry.IdentityFile == "").Chosen() {
									form.entry.IdentityFile = ""
									return
								}
								names := sshIdentityNames()
								if len(names) == 0 {
									m.Item(a.tr("未找到 SSH 私钥", "No SSH private keys found")).Disabled(true)
								}
								for _, name := range names {
									path := "~/.ssh/" + name
									if m.Item(name).Checked(form.entry.IdentityFile == path).Chosen() {
										form.entry.IdentityFile = path
									}
								}
							})
						})
					}
					if a.projectView && form.entry.isProject() {
						ui.Row(c).Gap(8).Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								formInput(c, a.tr("项目页签文字", "Project tab title"), &form.entry.ProjectTitle, a.tr("默认使用配置名称", "Defaults to configuration name"))
							})
							ui.Column(c).Width(100).Shrink(0).Children(func() {
								formInput(c, a.tr("项目图标", "Project icon"), &form.entry.ProjectIcon, a.tr("例如 📁", "e.g. 📁"))
							})
						})
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							if bitmap := form.entry.projectBitmap; bitmap != nil {
								ui.Image(c, bitmap).Size(24, 24).Fit(ui.Contain)
							}
							ui.Button(c, a.tr("选择本地图标…", "Choose local icon…")).Disabled(form.picking).
								OnClick(func() { a.pickProjectIcon(form) })
							if form.entry.ProjectImage != "" {
								ui.Button(c, a.tr("移除本地图标", "Remove local icon")).Disabled(form.picking).OnClick(func() {
									form.entry.ProjectImage, form.entry.projectBitmap, form.error = "", nil, ""
								})
							}
						})
					}
					if form.error != "" {
						ui.Text(c, form.error).TextColor(c.Theme().Danger)
					}
				})
			})
			ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0).Children(func() {
				ui.Button(c, a.tr("取消", "Cancel")).Disabled(a.savingEntries).OnClick(func() { form.open = false })
				save := a.tr("保存", "Save")
				if a.savingEntries {
					save = a.tr("正在保存…", "Saving…")
				}
				ui.PrimaryButton(c, save).Disabled(form.picking || a.savingEntries).OnClick(func() { a.saveEditor(form) })
			})
		})
	})
	if !open && !a.savingEntries {
		form.open = false
	}
}

func (a *localTerm) requestDelete(c *ui.Context, e entry) {
	for _, item := range a.entries {
		if item.Parent == e.ID {
			a.deleteID, a.confirm = e.ID, true
			return
		}
	}
	if err := a.deleteEntry(e.ID); err != nil {
		c.Toast(a.tr("无法删除条目：", "Could not delete connection: ") + err.Error())
	}
}

func (a *localTerm) confirmDelete(c *ui.Context) {
	ui.Modal(c, &a.confirm, func() {
		ui.Column(c).Width(320).Gap(12).Children(func() {
			ui.Text(c, a.tr("删除分组", "Delete group")).FontSize(interfaceTextSize(c, 17)).Bold()
			ui.Text(c, a.tr("删除「", "Delete “")+a.entryByID(a.deleteID).Name+a.tr("」及其包含的所有条目？", "” and all its connections?"))
			ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0).Children(func() {
				ui.Button(c, a.tr("取消", "Cancel")).AutoFocus().OnClick(func() { a.confirm = false })
				ui.Button(c, a.tr("删除", "Delete")).TextColor(c.Theme().Danger).OnClick(func() {
					a.confirm = false
					if err := a.deleteEntry(a.deleteID); err != nil {
						c.Toast(a.tr("无法删除分组：", "Could not delete group: ") + err.Error())
					}
				})
			})
		})
	})
}
