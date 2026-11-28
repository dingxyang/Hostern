package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/dingxyang/localterm/internal/terminal"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

type session struct {
	entryID  string
	name     string
	kind     entryKind
	files    *fileSession
	term     *terminal.Terminal
	pinned   bool
	tabColor ui.Color
}

type editor struct {
	fixedKind     bool
	addToProjects bool
	entry         entry
	add           bool
	open          bool
	atRoot        bool
	picking       bool
	focusName     bool
	error         string
}

type localTerm struct {
	preferences       preferences
	consoleTheme      *terminal.Theme
	settingsDraft     preferences
	settingsPath      string
	settingsError     string
	settingsOpen      bool
	settingsTab       int
	recordingShortcut int
	settingsSearch    string
	settingsFonts     []string
	settingsUIFonts   []string
	quickCommandForm  *quickCommandForm
	tabTitle          *tabTitleForm
	tabColorPicker    *tabColorPicker
	win               *mygo.Window
	update            func(func())
	storePath         string
	storeError        error
	savingEntries     bool
	entries           []entry
	connectionSearch  string
	selected          string
	expanded          map[string]*bool
	editor            *editor
	confirm           bool
	deleteID          string
	tabs              []*session
	active            *session
	focusTab          bool
	terminalErrors    []string
	sidebar           float32
	sidebarHidden     bool
	projectView       bool
	minimalView       bool
	projectID         string
	openOnStart       bool

	connectionDoublePress string
}

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.WhenReady(func() {
		app := &localTerm{sidebar: 270, expanded: make(map[string]*bool)}
		dir, err := mygo.App.Path(mygo.PathAppData)
		if err == nil {
			// MyGo normally bases the data directory on the bundle name.
			dataName := "LocalTerm"
			if mygo.App.Name() == "Hostern Dev" {
				dataName += " Dev"
			}
			dir = filepath.Join(dir, dataName)
			err = os.MkdirAll(dir, 0o755)
			if err == nil {
				mygo.App.SetPath(mygo.PathUserData, dir)
			}
		}
		if err == nil {
			app.storePath = filepath.Join(dir, "connections.json")
			app.entries, err = loadEntries(app.storePath)
			app.settingsPath = filepath.Join(dir, "settings.json")
			var settingsErr error
			app.preferences, settingsErr = loadPreferences(app.settingsPath)
			if settingsErr != nil {
				app.settingsError = settingsErr.Error()
			}
		}
		if (app.preferences.UIFont != "" && app.preferences.UIFont != systemUIFont) ||
			(app.preferences.Font != "" && app.preferences.Font != "Menlo") {
			systemFontFamilies()
		}
		if runtime.GOOS == "darwin" {
			mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
				{Label: "Hostern", Submenu: []*mygo.MenuItem{
					{Role: mygo.RoleAbout},
					updater.MenuItem(),
					mygo.Separator(),
					{Role: mygo.RoleServices},
					mygo.Separator(),
					{Role: mygo.RoleHide},
					{Role: mygo.RoleHideOthers},
					{Role: mygo.RoleUnhide},
					mygo.Separator(),
					{Role: mygo.RoleQuit},
				}},
				{Role: mygo.RoleFileMenu},
				{Role: mygo.RoleEditMenu},
				{Role: mygo.RoleViewMenu},
				{Role: mygo.RoleWindowMenu},
			}))
		}
		app.storeError = err
		app.restoreConnectionSelection()
		app.restoreViewMode()
		mygo.Theme.SetSource(normalizeAppearance(app.preferences.Appearance))
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "Hostern", Width: 1080, Height: 680,
			MinWidth: 720, MinHeight: 530,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(app.view),
		})
		app.update = app.win.Update
		app.openOnStart = true
		app.win.Invalidate()
		app.win.OnClosed(app.closeSessions)
		mygo.App.OnQuit(app.closeSessions)
		mygo.App.OnSecondInstance(func([]string, string) { app.win.Focus() })
	})
	mygo.Use(native.Plugin)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *localTerm) selectedEntry() entry {
	return a.entryByID(a.selected)
}

func (a *localTerm) restoreConnectionSelection() {
	e := a.entryByID(a.preferences.SelectedConnection)
	a.selected = e.ID
	for parent := e.Parent; parent != ""; parent = a.entryByID(parent).Parent {
		*a.expansion(parent) = true
	}
}

func (a *localTerm) selectConnection(c *ui.Context, id string) {
	a.selected = id
	if a.settingsPath == "" || a.preferences.SelectedConnection == id {
		return
	}
	next := a.preferences
	next.SelectedConnection = id
	if err := saveJSON(a.settingsPath, next); err != nil {
		c.Toast(a.tr("无法保存选中的连接：", "Could not save the selected connection: ") + err.Error())
		return
	}
	a.preferences = next
}

func (a *localTerm) entryByID(id string) entry {
	for _, e := range a.entries {
		if e.ID == id {
			return e
		}
	}
	return entry{}
}

func (a *localTerm) editEntry(id string) {
	open := func() {
		if e := a.entryByID(id); e.ID != "" {
			a.editor = &editor{entry: e, open: true}
		}
	}
	// Open the modal in a window update after the native menu event.
	if a.update != nil {
		a.update(open)
		return
	}
	open()
}

// runTask does blocking work off the UI thread, then publishes the result
// on the window's thread. Standalone views have no window dispatcher.
func (a *localTerm) runTask(work func() func()) {
	if a.update == nil {
		work()()
		return
	}
	update := a.update
	go func() { update(work()) }()
}

func (a *localTerm) setEntries(entries []entry) error {
	if a.storeError != nil {
		return a.storeError
	}
	if err := saveEntries(a.storePath, entries); err != nil {
		return err
	}
	a.entries = entries
	return nil
}

func (a *localTerm) expansion(id string) *bool {
	if a.expanded[id] == nil {
		open := true
		a.expanded[id] = &open
	}
	return a.expanded[id]
}

func (a *localTerm) addEntry() {
	a.newEntry(category, a.selectedEntry(), false)
}

func (a *localTerm) newEntry(kind entryKind, target entry, fixedKind bool) {
	parent := target.Parent
	if target.Kind == category {
		parent = target.ID
	}
	a.editor = &editor{entry: entry{Kind: kind, Parent: parent}, add: true, open: true, fixedKind: fixedKind}
}

func (a *localTerm) pickDirectory(form *editor) {
	form.picking = true
	title := a.tr("选择本地工作目录", "Choose a local directory")
	// Let AppKit choose the starting location. MyGo probes DefaultPath
	// on the main thread, which can stall when it points to a slow volume.
	a.runTask(func() func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win, Title: title, Directory: true,
		})
		return func() {
			form.picking = false
			if err != nil {
				form.error = a.tr("无法打开目录选择器：", "Could not open the directory picker: ") + err.Error()
			} else if len(paths) > 0 {
				form.entry.Path, form.error = paths[0], ""
				if form.entry.Name == "" {
					form.entry.Name = filepath.Base(paths[0])
				}
			}
		}
	})
}

func (a *localTerm) saveEditor(form *editor) {
	if a.savingEntries {
		return
	}
	if a.storeError != nil {
		form.error = a.tr("无法保存连接树：", "Could not save connections: ") + a.storeError.Error()
		return
	}
	a.savingEntries, form.error = true, ""
	next, draft := slices.Clone(a.entries), form.entry
	add, atRoot, path := form.add, form.atRoot, a.storePath
	saveError := a.tr("无法保存连接树：", "Could not save connections: ")
	a.runTask(func() func() {
		e, err := draft.validated()
		if err != nil {
			return func() {
				a.savingEntries, form.error = false, err.Error()
			}
		}
		if add {
			e.ID = rand.Text()
			if atRoot {
				e.Parent = ""
			}
			next = append(next, e)
		} else {
			for i := range next {
				if next[i].ID == e.ID {
					next[i] = e
					break
				}
			}
		}
		err = saveEntries(path, next)
		return func() {
			a.savingEntries = false
			if err != nil {
				form.error = saveError + err.Error()
				return
			}
			a.entries = next
			if e.Parent != "" && add {
				*a.expansion(e.Parent) = true
			}
			a.selected, form.open = e.ID, false
			if form.addToProjects && a.projectView {
				a.openProject(e)
			}
		}
	})
}

func (a *localTerm) deleteEntry(id string) error {
	if a.savingEntries {
		return errors.New(a.tr("正在保存连接树，请稍后再试", "Connections are being saved. Try again shortly."))
	}
	next := removeEntry(a.entries, id)
	if err := a.setEntries(next); err != nil {
		return err
	}
	for id := range a.expanded {
		if a.entryByID(id).ID == "" {
			delete(a.expanded, id)
		}
	}
	a.selected = ""
	a.syncProject()
	return nil
}

func newTerminal(e entry, font terminal.Font, theme *terminal.Theme) (*terminal.Terminal, error) {
	e, err := e.validated()
	if err != nil {
		return nil, err
	}
	options := terminal.Options{Theme: theme, Dir: e.Path, Font: font}
	if e.Kind == host {
		options.Command = e.sshCommand()
	} else {
		options.StartupCommand = e.StartupCommand
	}
	return terminal.New(options)
}

func (a *localTerm) activate(s *session) {
	a.active, a.focusTab = s, s != nil
	if a.projectView && s != nil {
		a.projectID = a.sessionProject(s)
	}
}

func (a *localTerm) closeTab(s *session) {
	i := slices.Index(a.tabs, s)
	if i < 0 {
		return
	}
	if a.tabTitle != nil && a.tabTitle.session == s {
		a.tabTitle = nil
	}
	if a.tabColorPicker != nil && a.tabColorPicker.session == s {
		a.tabColorPicker = nil
	}
	a.tabs = slices.Delete(a.tabs, i, i+1)
	if a.active == s {
		if a.projectView {
			a.activate(a.projectSession(a.projectID))
		} else if len(a.tabs) == 0 {
			a.activate(nil)
		} else {
			a.activate(a.tabs[min(i, len(a.tabs)-1)])
		}
	}
	s.close()
}

func (a *localTerm) closeSessions() {
	for _, tab := range a.tabs {
		tab.close()
	}
	a.tabs, a.active = nil, nil
}

func (s *session) close() {
	if s.files != nil {
		s.files.cancel()
		defer s.files.removeSocketDir()
	}
	if s.term != nil {
		if err := s.term.Close(); err != nil {
			log.Printf("关闭终端 %q：%v", s.name, err)
		}
	}
}

func (a *localTerm) launch(e entry) {
	a.launchSession(e, nil)
}

func (a *localTerm) launchSession(e entry, files *fileSession) {
	font := a.preferences.terminalFont()
	theme := a.consoleTheme
	if theme == nil {
		theme = &paletteFor(a.preferences.Appearance, false, a.preferences.ThemeStyle).console
	}
	name := e.Name
	if files != nil {
		name = "SFTP · " + name
	}
	s := &session{entryID: e.ID, name: name, kind: e.Kind, files: files}
	a.tabs = append(a.tabs, s)
	a.activate(s)
	a.runTask(func() func() {
		var term *terminal.Terminal
		var err error
		if files == nil {
			term, err = newTerminal(e, font, theme)
		} else {
			term, err = terminal.New(terminal.Options{Theme: theme, Font: font, Command: e.sftpMasterCommand(files.socket)})
		}
		return func() {
			s.term = term
			if !slices.Contains(a.tabs, s) {
				s.close()
				return
			}
			if err != nil {
				a.closeTab(s)
				a.terminalErrors = append(a.terminalErrors,
					fmt.Sprintf(a.tr("无法打开 %s：%v", "Could not open %s: %v"), name, err))
				return
			}
			term.SetFont(a.preferences.terminalFont())
			if a.consoleTheme != nil && a.consoleTheme != theme {
				term.SetTheme(a.consoleTheme, nil)
			}
			if files != nil {
				a.connectFiles(s, e.Host)
			}
		}
	})
}

const (
	viewModeSidebar  = "sidebar"
	viewModeProjects = "projects"
	viewModeMinimal  = "minimal"
)

func (a *localTerm) restoreViewMode() {
	switch a.preferences.ViewMode {
	case viewModeProjects:
		a.projectView = true
		a.minimalView = false
		a.syncProject()
	case viewModeMinimal:
		a.minimalView = true
		a.projectView = false
	default:
		a.projectView = false
		a.minimalView = false
	}
}

func (a *localTerm) saveViewMode(mode string) {
	if a.settingsPath == "" || a.preferences.ViewMode == mode {
		return
	}
	next := a.preferences
	next.ViewMode = mode
	if err := saveJSON(a.settingsPath, next); err != nil {
		return
	}
	a.preferences = next
}

func (a *localTerm) openLocal(c *ui.Context) {
	e := a.selectedEntry()
	if !a.projectView && !a.minimalView && e.Kind == local && checkDirectory(e.Path) == nil {
		e.StartupCommand = ""
		a.launch(e)
		return
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		c.Toast(a.tr("无法获取用户主目录：", "Could not find your home directory: ") + err.Error())
		return
	}
	a.launch(entry{Kind: local, Name: a.tr("本地终端", "Local terminal"), Path: dir})
}

func (a *localTerm) toggleSidebar() {
	if a.projectView {
		a.toggleProjects()
		a.sidebarHidden = false
		return
	}
	if a.minimalView {
		a.toggleMinimal()
		return
	}
	a.sidebarHidden = !a.sidebarHidden
	a.focusTab = a.active != nil
}

func (a *localTerm) toggleMinimal() {
	a.minimalView = !a.minimalView
	if a.minimalView {
		a.projectView = false
		if a.active == nil && len(a.tabs) > 0 {
			a.activate(a.tabs[len(a.tabs)-1])
		}
		a.saveViewMode(viewModeMinimal)
	} else {
		a.sidebarHidden = false
		a.saveViewMode(viewModeSidebar)
	}
	a.focusTab = a.active != nil
}
