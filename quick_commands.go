package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
)

type quickCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type quickCommandForm struct {
	quickCommand
	open bool
	err  string
}

func (a *localTerm) quickCommandsMenu(m *ui.Menu, s *session, selection string) {
	selection = strings.TrimSpace(selection)
	ended := false
	select {
	case <-s.term.Done():
		ended = true
	default:
	}

	m.Submenu(a.tr("快捷命令", "Quick Commands"), func(m *ui.Menu) {
		for _, cmd := range a.preferences.QuickCommands {
			if m.Item(cmd.Name).Disabled(ended || !validQuickCommand(cmd.Command)).Chosen() {
				s.term.Paste(cmd.Command)
				a.focusTab = true
			}
		}
		if len(a.preferences.QuickCommands) > 0 {
			m.Separator()
		}
		if m.Item(a.tr("将选中内容添加为快捷命令…", "Add Selection as Quick Command…")).Disabled(selection == "").Chosen() {
			a.quickCommandForm = &quickCommandForm{quickCommand: quickCommand{Command: selection}, open: true}
		}
		if m.Item(a.tr("手动添加快捷命令…", "Add Quick Command…")).Chosen() {
			a.quickCommandForm = &quickCommandForm{open: true}
		}
	})
}

func validQuickCommand(command string) bool {
	return strings.TrimSpace(command) != "" && !strings.ContainsFunc(command, unicode.IsControl)
}

func (a *localTerm) saveQuickCommand(form *quickCommandForm) {
	cmd := form.quickCommand
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" || strings.ContainsFunc(cmd.Name, unicode.IsControl) {
		form.err = a.tr("请输入单行名称。", "Enter a single-line name.")
		return
	}
	if !validQuickCommand(cmd.Command) {
		form.err = a.tr("请输入一条单行命令。", "Enter a single-line command.")
		return
	}
	if a.settingsPath == "" {
		form.err = a.tr("无法访问设置存储目录。", "Settings storage is unavailable.")
		return
	}
	next := a.preferences.withDefaults()
	next.QuickCommands = append(slices.Clone(next.QuickCommands), cmd)
	if err := saveJSON(a.settingsPath, next); err != nil {
		form.err = a.tr("无法保存快捷命令：", "Could not save quick command: ") + err.Error()
		return
	}
	a.preferences = next
	form.open = false
}

func (a *localTerm) quickCommandView(c *ui.Context, form *quickCommandForm) {
	c = c.Key(form)
	ui.Modal(c, &form.open, func() {
		ui.Column(c).Width(430).Gap(12).Children(func() {
			ui.Text(c, a.tr("添加快捷命令", "Add Quick Command")).FontSize(interfaceTextSize(c, 23)).Bold()
			ui.Text(c, a.tr("名称", "Name"))
			formInput(c, a.tr("名称", "Name"), &form.Name, a.tr("例如：查看 Git 状态", "Example: Git status")).AutoFocus()
			ui.Text(c, a.tr("命令", "Command"))
			ui.TextArea(c, &form.Command).Label(a.tr("命令", "Command")).Height(76).FillWidth()
			ui.Text(c, a.tr("选择快捷命令会将它插入终端，按回车后执行。", "Choose a quick command to insert it, then press Enter to run it.")).
				TextColor(c.Theme().TextMuted)
			if form.err != "" {
				ui.Text(c, form.err).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, a.tr("取消", "Cancel")).OnClick(func() { form.open = false })
				ui.PrimaryButton(c, a.tr("保存", "Save")).OnClick(func() { a.saveQuickCommand(form) })
			})
		})
	})
}
