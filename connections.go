package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type entryKind string

const (
	category entryKind = "category"
	host     entryKind = "ssh"
	local    entryKind = "local"
)

func (a *localTerm) kindLabel(k entryKind) string {
	switch k {
	case category:
		return a.tr("分组", "Group")
	case host:
		return a.tr("远程主机", "SSH host")
	default:
		return a.tr("本地工作目录", "Local directory")
	}
}

type entry struct {
	ID             string    `json:"id"`
	Parent         string    `json:"parent,omitempty"`
	Kind           entryKind `json:"kind"`
	Name           string    `json:"name"`
	Host           string    `json:"host,omitempty"`
	User           string    `json:"user,omitempty"`
	Port           string    `json:"port,omitempty"`
	Path           string    `json:"path,omitempty"`
	StartupCommand string    `json:"startupCommand,omitempty"`
	InProjects     bool      `json:"inProjects,omitempty"`
	ProjectTitle   string    `json:"projectTitle,omitempty"`
	ProjectIcon    string    `json:"projectIcon,omitempty"`
	ProjectImage   string    `json:"projectImage,omitempty"`
	projectBitmap  *ui.Bitmap
}

var (
	sshHost = regexp.MustCompile(`^[A-Za-z0-9_.:][A-Za-z0-9_.:%-]*$`)
	sshUser = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@-]*$`)
)

func (e entry) validated() (entry, error) {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" || strings.ContainsAny(e.Name, "\r\n\x00") {
		return e, errors.New("请输入单行名称")
	}
	e.ProjectTitle = strings.TrimSpace(e.ProjectTitle)
	e.ProjectIcon = strings.TrimSpace(e.ProjectIcon)
	if strings.ContainsFunc(e.ProjectTitle, unicode.IsControl) {
		return e, errors.New("请输入单行项目页签文字")
	}
	if strings.ContainsFunc(e.ProjectIcon, unicode.IsControl) || utf8.RuneCountInString(e.ProjectIcon) > 12 {
		return e, errors.New("请输入简短的项目图标，例如一个 Emoji")
	}
	if e.Kind == category || strings.TrimSpace(e.StartupCommand) == "" {
		e.StartupCommand = ""
	} else if strings.ContainsAny(e.StartupCommand, "\r\n\x00") {
		return e, errors.New("请输入单行启动命令")
	}
	switch e.Kind {
	case category:
		e.Host, e.User, e.Port, e.Path = "", "", "", ""
	case local:
		if err := checkDirectory(e.Path); err != nil {
			return e, err
		}
		e.Host, e.User, e.Port = "", "", ""
	case host:
		e.Path = ""
		e.Host = strings.TrimSpace(e.Host)
		if strings.HasPrefix(e.Host, "[") && strings.HasSuffix(e.Host, "]") && strings.Contains(e.Host, ":") {
			e.Host = e.Host[1 : len(e.Host)-1]
		}
		e.User, e.Port = strings.TrimSpace(e.User), strings.TrimSpace(e.Port)
		if !sshHost.MatchString(e.Host) {
			return e, errors.New("请输入 SSH 别名、主机地址或 IP，用户名和端口请填在单独的字段")
		}
		if e.User != "" && !sshUser.MatchString(e.User) {
			return e, errors.New("SSH 用户名包含无效字符")
		}
		if e.Port != "" {
			port, err := strconv.ParseUint(e.Port, 10, 16)
			if err != nil || port == 0 {
				return e, errors.New("端口必须是 1 到 65535 之间的整数")
			}
			e.Port = strconv.FormatUint(port, 10)
		}
	default:
		return e, errors.New("未知的条目类型")
	}
	return e, nil
}

func checkDirectory(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("请选择目录或输入绝对路径")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("无法访问目录 %q：%w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q 不是目录", path)
	}
	return nil
}

func (e entry) sshCommand() []string {
	args := []string{"/usr/bin/ssh", "-tt"}
	if e.User != "" {
		args = append(args, "-l", e.User)
	}
	if e.Port != "" {
		args = append(args, "-p", e.Port)
	}
	args = append(args, "--", e.Host)
	if e.StartupCommand != "" {
		args = append(args, e.StartupCommand)
	}
	return args
}

func loadEntries(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	byID := make(map[string]entry, len(entries))
	for i, e := range entries {
		if e.ID == "" || byID[e.ID].ID != "" || (e.Kind != category && e.Kind != host && e.Kind != local) {
			return nil, errors.New("连接树中存在无效条目")
		}
		entries[i].projectBitmap = storedProjectIcon(e.ProjectImage)
		byID[e.ID] = e
	}
	for _, e := range entries {
		for parent, depth := e.Parent, 0; parent != ""; depth++ {
			group := byID[parent]
			if group.Kind != category || depth >= len(entries) {
				return nil, errors.New("连接树中存在无效的分组关系")
			}
			parent = group.Parent
		}
	}
	return entries, nil
}

func saveEntries(path string, entries []entry) error {
	return saveJSON(path, entries)
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func removeEntry(entries []entry, id string) []entry {
	children := make(map[string][]string)
	for _, e := range entries {
		children[e.Parent] = append(children[e.Parent], e.ID)
	}
	removed := make(map[string]bool)
	var visit func(string)
	visit = func(id string) {
		removed[id] = true
		for _, child := range children[id] {
			visit(child)
		}
	}
	visit(id)
	return slices.DeleteFunc(slices.Clone(entries), func(e entry) bool { return removed[e.ID] })
}

type connectionDrag string

type connectionDropPosition uint8

const (
	moveBefore connectionDropPosition = iota
	moveAfter
	moveInside
)

type connectionDrop struct {
	source   connectionDrag
	target   string
	position connectionDropPosition
}

func entryIndex(entries []entry, id string) int {
	return slices.IndexFunc(entries, func(e entry) bool { return e.ID == id })
}

// destination gives the insertion index before removing the source.
func (d connectionDrop) destination(entries []entry) (index int, parent string, ok bool) {
	source := string(d.source)
	if source == d.target || entryIndex(entries, source) < 0 {
		return 0, "", false
	}
	index = len(entries)
	if d.target != "" {
		target := entryIndex(entries, d.target)
		if target < 0 {
			return 0, "", false
		}
		e := entries[target]
		switch d.position {
		case moveBefore:
			index, parent = target, e.Parent
		case moveAfter:
			index, parent = target+1, e.Parent
		case moveInside:
			if e.Kind != category {
				return 0, "", false
			}
			parent = e.ID
		}
	}
	for ancestor := parent; ancestor != ""; {
		if ancestor == source {
			return 0, "", false
		}
		ancestor = entries[entryIndex(entries, ancestor)].Parent
	}
	return index, parent, true
}
