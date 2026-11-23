package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo/ui"
)

type fileForm struct {
	pane         *filePane
	title        string
	name         string
	item         os.FileInfo
	remove       bool
	open         bool
	conflict     func(overwrite bool)
	canOverwrite bool
}

const sftpSizeWidth = 72

func (a *localTerm) fileBrowser(c *ui.Context, s *session) {
	f := s.files
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			a.filePanel(c.Key("local"), s, &f.local, a.tr("本地", "Local"))
			ui.Box(c).Width(1).Shrink(0).Background(theme.Border)
			a.filePanel(c.Key("remote"), s, &f.remote, a.tr("远程", "Remote"))
		})
		ui.Box(c).Height(1).FillWidth().Shrink(0).Background(theme.Border)
		ui.Row(c).Height(36).Shrink(0).AlignItems(ui.Center).Gap(6).PaddingX(8).Children(func() {
			for _, p := range []*filePane{&f.local, &f.remote} {
				file := selectedFile(p)
				label := a.tr("上传 →", "Upload →")
				if p == &f.remote {
					label = a.tr("← 下载", "← Download")
				}
				ui.Button(c.Key(p), label).Disabled(!f.canTransfer(p, file)).OnClick(func() {
					a.transferFile(f, p, file)
				})
			}
			status := f.status
			if status == "" && f.client != nil {
				status = a.tr("已连接", "Connected")
			}
			ui.Text(c, status).SingleLine().Grow(1).MinWidth(0).TextColor(theme.TextMuted).Tooltip(status)
		})
	})
	if form := f.form; form != nil && form.open {
		a.fileActionDialog(c.Key(form), f, form)
	}
}

func selectedFile(p *filePane) os.FileInfo {
	for _, item := range p.items {
		if item.Name() == p.selected {
			return item
		}
	}
	return nil
}

func (f *fileSession) canTransfer(p *filePane, file os.FileInfo) bool {
	return !f.busy && f.client != nil && f.local.dir != "" && file != nil && (file.Mode().IsRegular() || p == &f.local && file.IsDir())
}

func (a *localTerm) filePanel(c *ui.Context, s *session, p *filePane, title string) {
	f := s.files
	theme := c.Theme()
	ready := p.dir != "" && !f.busy
	panel := ui.Column(c).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Clip().Background(theme.Background)
	if p.dir != "" {
		panel.ContextMenu(func(m *ui.Menu) { a.fileContextMenu(c, m, s, p, nil) })
	}
	panel.Children(func() {
		ui.Row(c).Padding(4, 8, 2, 10).Height(32).Shrink(0).Children(func() {
			ui.Text(c, title).Bold().FontSize(interfaceTextSize(c, 13)).Grow(1)
			if p.dir != "" && (p == &f.local || f.client != nil) {
				label := title + " · " + a.tr("收藏路径", "Favorite Paths")
				iconButton(c, "☆", label).Center().Shrink(0).Menu(func(m *ui.Menu) { a.sftpFavoritesMenu(c, m, s, p) })
			}
		})
		if p == &f.remote && f.client == nil {
			a.sftpConnectionView(c, s)
			return
		}
		navigate := func(dir string) { a.refreshFiles(f, p, dir, nil) }
		ui.Row(c).AlignItems(ui.Center).Gap(4).Padding(0, 8, 6).Shrink(0).Children(func() {
			ui.Button(c, "↑").Label(a.tr("上一级", "Parent folder")).Tooltip(a.tr("上一级", "Parent folder")).
				Disabled(!ready).OnClick(func() { navigate("..") })
			label := a.tr("目录路径", "Folder path")
			input := ui.TextInput(c, &p.input).Label(title + " · " + label).Placeholder(label).Grow(1).MinWidth(0).
				Disabled(!ready).OnSubmit(func() { navigate(p.input) })
			if p == &f.remote && a.focusTab {
				input.Focus()
				a.focusTab = false
			}
			ui.Button(c, a.tr("前往", "Go")).Disabled(!ready).OnClick(func() { navigate(p.input) })
			ui.Button(c, "↻").Label(a.tr("刷新", "Refresh")).Tooltip(a.tr("刷新", "Refresh")).
				Disabled(!ready).OnClick(func() { navigate(p.dir) })
		})
		ui.Row(c).Height(26).Shrink(0).PaddingX(12).Gap(8).Background(theme.Surface).Children(func() {
			ui.Text(c, a.tr("名称", "Name")).Grow(1).MinWidth(0).SingleLine().
				FontSize(interfaceTextSize(c, 11)).TextColor(theme.TextMuted)
			ui.Text(c, a.tr("大小", "Size")).Width(sftpSizeWidth).Shrink(0).TextAlign(ui.End).
				FontSize(interfaceTextSize(c, 11)).TextColor(theme.TextMuted)
		})
		ui.Box(c).Height(1).FillWidth().Shrink(0).Background(theme.Border)
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			if len(p.items) == 0 && p.dir != "" {
				ui.Text(c, a.tr("空文件夹", "Empty folder")).TextColor(theme.TextMuted).Margin(12, 12)
			}
			for _, item := range p.items {
				name := item.Name()
				key := c.Key(name)
				row := ui.ButtonBase(key).Height(29).FillWidth().PaddingX(12).Gap(8).AlignItems(ui.Center).
					Label(name).Tooltip(name).Selected(p.selected == name).Background(ui.Transparent).
					ContextMenu(func(m *ui.Menu) { a.fileContextMenu(c, m, s, p, item) }).
					OnClick(func() { p.selected = name })
				if p.selected == name {
					row.Background(theme.Selection)
				} else if row.Hovered() {
					row.Background(theme.SurfaceHover)
				}
				isDir := item.IsDir() || item.Mode()&os.ModeSymlink != 0
				if ready && isDir && (row.DoubleClicked() || row.Shortcut(0, ui.KeyEnter)) {
					navigate(name)
				}
				row.Children(func() {
					sftpItemIcon(key, item)
					ui.Text(key, name).SingleLine().Grow(1).MinWidth(0).FontSize(interfaceTextSize(c, 12))
					ui.Text(key, sftpItemSize(item)).SingleLine().Width(sftpSizeWidth).Shrink(0).TextAlign(ui.End).
						FontSize(interfaceTextSize(c, 11)).TextColor(theme.TextMuted)
				})
			}
		})
	})
}

func sftpItemSize(item os.FileInfo) string {
	if !item.Mode().IsRegular() {
		return ""
	}
	size := float64(item.Size())
	if size < 1024 {
		return fmt.Sprintf("%d B", item.Size())
	}
	for _, unit := range [...]string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"} {
		size /= 1024
		if size < 1024 || unit == "EiB" {
			return fmt.Sprintf("%.1f %s", size, unit)
		}
	}
	return ""
}

func sftpItemIcon(c *ui.Context, item os.FileInfo) {
	theme := c.Theme()
	if item.IsDir() {
		connectionIcon(c, category, theme.Text)
		return
	}
	ui.Box(c).Size(16, 16).Shrink(0).Role(ui.RoleNone).Draw(func(p *ui.Painter, r ui.Rect) {
		x, y := r.X, r.Y
		color := theme.TextMuted
		const stroke = 1.25
		var outline ui.Path
		outline.MoveTo(x+3, y+14).LineTo(x+3, y+2).LineTo(x+9, y+2).
			LineTo(x+13, y+6).LineTo(x+13, y+14).Close()
		p.StrokePath(&outline, stroke, color)
		var fold ui.Path
		fold.MoveTo(x+9, y+2).LineTo(x+9, y+6).LineTo(x+13, y+6)
		p.StrokePath(&fold, stroke, color)
		if item.Mode()&os.ModeSymlink != 0 {
			var arrow ui.Path
			arrow.MoveTo(x+5, y+12).LineTo(x+9, y+8).MoveTo(x+6, y+8).
				LineTo(x+9, y+8).LineTo(x+9, y+11)
			p.StrokePath(&arrow, stroke, color)
		}
	})
}

func (a *localTerm) sftpConnectionView(c *ui.Context, s *session) {
	f := s.files
	if f.connectErr != nil {
		ui.Text(c, a.tr("连接失败：", "Connection failed: ")+f.connectErr.Error()).Margin(6, 10)
	} else {
		ui.Text(c, a.tr("正在连接 SSH，请在下方完成认证。", "Connecting over SSH. Complete authentication below.")).Margin(6, 10)
	}
	if s.term != nil {
		view := terminal.View(c, s.term).Grow(1).MinHeight(0)
		if a.focusTab {
			view.Focus()
			a.focusTab = false
		}
	}
}

func (a *localTerm) fileContextMenu(c *ui.Context, m *ui.Menu, s *session, p *filePane, item os.FileInfo) {
	f := s.files
	disabled := f.busy || p.dir == ""
	if item != nil {
		name := item.Name()
		p.selected = name
		label := a.tr("上传", "Upload")
		if p == &f.remote {
			label = a.tr("下载", "Download")
		}
		if m.Item(a.tr("打开目录", "Open Folder")).Disabled(disabled || !item.IsDir() && item.Mode()&os.ModeSymlink == 0).Chosen() {
			a.refreshFiles(f, p, name, nil)
		}
		if m.Item(label).Disabled(!f.canTransfer(p, item)).Chosen() {
			a.transferFile(f, p, item)
		}
		m.Separator()
		if m.Item(a.tr("重命名…", "Rename…")).Disabled(disabled).Chosen() {
			f.form = &fileForm{pane: p, title: a.tr("重命名", "Rename"), name: name, item: item, open: true}
		}
		if m.Item(a.tr("删除…", "Delete…")).Disabled(disabled).Chosen() {
			f.form = &fileForm{pane: p, title: a.tr("删除", "Delete"), item: item, remove: true, open: true}
		}
		m.Separator()
	}
	if m.Item(a.tr("新建文件夹…", "New Folder…")).Disabled(disabled).Chosen() {
		f.form = &fileForm{pane: p, title: a.tr("新建文件夹", "New Folder"), open: true}
	}
	favorite := sftpFavorite{Path: p.dir}
	if item != nil {
		favorite = sftpFavorite{Path: f.location(p).resolve(item.Name()), File: !item.IsDir()}
	}
	if m.Item(a.tr("复制路径", "Copy Path")).Disabled(p.dir == "").Chosen() {
		c.Services().WriteClipboard(favorite.Path)
	}
	a.sftpFavoriteItem(c, m, s, p, favorite, a.tr("收藏此路径", "Add Path to Favorites"))
	if m.Item(a.tr("刷新", "Refresh")).Disabled(disabled).Chosen() {
		a.refreshFiles(f, p, p.dir, nil)
	}
}

func (a *localTerm) fileActionDialog(c *ui.Context, f *fileSession, form *fileForm) {
	ui.Modal(c, &form.open, func() {
		ui.Column(c).Width(330).Gap(10).Children(func() {
			ui.Text(c, form.title).Bold()
			if form.conflict != nil {
				ui.Text(c, fmt.Sprintf(a.tr("远程已存在「%s」。", "“%s” already exists on the remote server."), form.name))
				ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
					ui.Button(c, a.tr("取消", "Cancel")).OnClick(func() { form.open = false })
					if form.canOverwrite {
						ui.Button(c, a.tr("覆盖", "Overwrite")).OnClick(func() { form.open = false; form.conflict(true) })
					}
					ui.PrimaryButton(c, a.tr("自动重命名", "Auto Rename")).OnClick(func() { form.open = false; form.conflict(false) })
				})
				return
			}
			if form.remove {
				message := a.tr("删除「%s」？", "Delete “%s”?")
				if form.item.IsDir() {
					message = a.tr("删除文件夹「%s」？仅可删除空文件夹。", "Delete folder “%s”? Only empty folders can be deleted.")
				}
				if form.item.IsDir() && form.pane == &f.remote {
					message = a.tr("删除文件夹「%s」及其全部内容？", "Delete folder “%s” and all of its contents?")
				}
				ui.Text(c, fmt.Sprintf(message, form.item.Name()))
			} else {
				formInput(c, a.tr("名称", "Name"), &form.name, "").AutoFocus()
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, a.tr("取消", "Cancel")).OnClick(func() { form.open = false })
				confirm := a.tr("创建", "Create")
				if form.remove {
					confirm = a.tr("确认删除", "Confirm Delete")
				} else if form.item != nil {
					confirm = a.tr("保存", "Save")
				}
				ui.PrimaryButton(c, confirm).Disabled(f.busy || !form.remove && (!validFileName(form.name) ||
					form.item != nil && form.item.Name() == form.name)).OnClick(func() {
					form.open = false
					p := form.pane
					name, item, remove := form.name, form.item, form.remove
					a.refreshFiles(f, p, p.dir, func(loc fileLocation) error {
						switch {
						case remove:
							if loc.client != nil {
								return loc.removeRemoteTree(f.ctx, item.Name())
							}
							return loc.remove(item.Name(), item.IsDir())
						case item != nil:
							return loc.rename(item.Name(), name)
						default:
							return loc.mkdir(name)
						}
					})
				})
			})
		})
	})
}

func (f *fileSession) location(p *filePane) fileLocation {
	loc := fileLocation{dir: p.dir}
	if p == &f.remote {
		loc.client = f.client
	}
	return loc
}

func (a *localTerm) transferFile(f *fileSession, p *filePane, item os.FileInfo) {
	if !f.canTransfer(p, item) {
		return
	}
	destination := &f.remote
	if p == &f.remote {
		destination = &f.local
	}
	source := f.location(p)
	name := item.Name()
	if p == &f.remote {
		a.refreshFiles(f, destination, destination.dir, func(target fileLocation) error {
			return copySFTPFile(f.ctx, source, target, name)
		})
		return
	}

	upload := func(target fileLocation, destName string, overwrite bool) error {
		if !validFileName(name) {
			return fmt.Errorf("invalid file name %q", name)
		}
		if item.IsDir() {
			return copySFTPDirectory(f.ctx, source, target, name, destName)
		}
		return copySFTPFileAs(f.ctx, source, target, name, destName, overwrite)
	}
	var existing os.FileInfo
	a.updateFiles(f, destination, destination.dir, "", func(target fileLocation) error {
		var err error
		existing, err = target.stat(name)
		if os.IsNotExist(err) {
			return upload(target, name, false)
		}
		return err
	}, func() {
		if existing == nil {
			return
		}
		f.form = &fileForm{
			title: a.tr("上传名称冲突", "Upload Name Conflict"), name: name, open: true,
			canOverwrite: item.Mode().IsRegular() && existing.Mode().IsRegular(),
			conflict: func(overwrite bool) {
				a.refreshFiles(f, destination, destination.dir, func(target fileLocation) error {
					if overwrite {
						return upload(target, name, true)
					}
					ext := ""
					if !item.IsDir() {
						ext = path.Ext(name)
						if ext == name {
							ext = ""
						}
					}
					for i := 1; ; i++ {
						if err := f.ctx.Err(); err != nil {
							return err
						}
						candidate := fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext)
						_, err := target.stat(candidate)
						if os.IsNotExist(err) {
							return upload(target, candidate, false)
						}
						if err != nil {
							return err
						}
					}
				})
			},
		}
		f.status = ""
	})
}

func (a *localTerm) refreshFiles(f *fileSession, p *filePane, dir string, change func(fileLocation) error) {
	a.refreshFileSelection(f, p, dir, "", change)
}

func (a *localTerm) refreshFileSelection(f *fileSession, p *filePane, dir, selection string, change func(fileLocation) error) {
	a.updateFiles(f, p, dir, selection, change, nil)
}

func (a *localTerm) updateFiles(f *fileSession, p *filePane, dir, selection string, change func(fileLocation) error, after func()) {
	if f.busy || p.dir == "" || dir == "" || p == &f.remote && f.client == nil {
		return
	}
	loc := f.location(p)
	f.busy = true
	f.status = a.tr("正在处理…", "Working…")
	a.runTask(func() func() {
		var err error
		if change != nil {
			err = change(loc)
		}
		current, items, readErr := loc.read(dir)
		if err == nil {
			err = readErr
		}
		return func() {
			if f.ctx.Err() != nil {
				return
			}
			f.busy = false
			if readErr == nil {
				p.show(current, items)
			}
			if err != nil {
				f.status = strings.TrimSpace(err.Error())
				return
			}
			f.status = ""
			if selection != "" {
				p.selected = selection
				if selectedFile(p) == nil {
					p.selected = ""
					f.status = a.tr("找不到收藏的文件：", "Favorite file not found: ") + selection
				}
			}
			if change != nil {
				f.status = a.tr("操作完成", "Operation complete")
			}
			if after != nil {
				after()
			}
		}
	})
}
