package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/egoist/mygo/ui"
)

var agentLaunchers = [...]struct {
	name, command string
}{
	{"Codex", "codex"},
	{"Claude Code", "claude"},
}

func (a *localTerm) newTerminalMenu(c *ui.Context, m *ui.Menu) {
	project := entry{}
	if a.projectView {
		project = a.entryByID(a.projectID)
	}
	if m.Item(a.tr("本地终端", "Local Terminal")).Chosen() {
		if project.Kind == local {
			a.launch(project)
		} else {
			a.openLocal(c)
		}
	}
	if project.Kind == host && m.Item(a.tr("SSH 终端", "SSH Terminal")).Chosen() {
		a.launch(project)
	}
	if project.Kind == host && m.Item(a.tr("打开 SFTP", "Open SFTP")).Chosen() {
		a.launchSFTP(project)
	}
	m.Separator()
	for i, agent := range agentLaunchers {
		_, err := exec.LookPath(agent.command)
		if m.Item(agent.name).Disabled(err != nil).Chosen() {
			target := project
			if target.Kind != local {
				dir, err := os.UserHomeDir()
				if err != nil {
					c.Toast(a.tr("无法获取用户主目录：", "Could not find your home directory: ") + err.Error())
					return
				}
				target = entry{Kind: local, Path: dir}
			}
			target.Name = agent.name
			target.StartupCommand = strings.TrimSpace(agent.command + " " + a.preferences.AgentArguments[i])
			a.launch(target)
		}
	}
	m.Separator()
	if m.Item(a.tr("配置 Agent 启动参数…", "Configure Agent Launch Arguments…")).Chosen() {
		a.openSettings()
		a.settingsTab = settingsAgentsTab
	}
}

func (a *localTerm) settingsAgents(c *ui.Context) {
	ui.Text(c, a.tr("这些参数会追加到启动命令。留空则使用 CLI 的默认设置。", "Arguments are appended to the launch command. Leave them blank to use the CLI defaults.")).
		TextColor(c.Theme().TextMuted)
	for i, agent := range agentLaunchers {
		key := c.Key(agent.command)
		ui.Text(key, agent.name).FontSize(interfaceTextSize(c, 12)).Bold()
		label := a.tr(agent.name+" 启动参数", agent.name+" launch arguments")
		ui.TextInput(key, &a.settingsDraft.AgentArguments[i]).Label(label).
			Placeholder(a.tr("启动参数", "Launch arguments")).FillWidth()
	}
}
