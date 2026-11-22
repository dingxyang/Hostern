package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pkg/sftp"
)

func TestSFTPBrowserMenus(t *testing.T) {
	localDir, remoteDir := t.TempDir(), t.TempDir()
	write := func(dir, name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(dir, name, want string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s contents = %q, error = %v, want %q", name, got, err, want)
		}
	}

	write(localDir, "upload.txt", "from local")
	write(remoteDir, "download.txt", "from remote")
	conn, peer := net.Pipe()
	t.Cleanup(func() { _ = conn.Close(); _ = peer.Close() })
	server, err := sftp.NewServer(peer)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	client, err := sftp.NewClientPipe(conn, conn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f := &fileSession{ctx: ctx, cancel: cancel, client: client}
	for pane, location := range map[*filePane]fileLocation{&f.local: {dir: localDir}, &f.remote: {dir: remoteDir, client: client}} {
		dir, items, err := location.read(".")
		if err != nil {
			t.Fatal(err)
		}
		pane.show(dir, items)
	}
	tab := &session{entryID: "server-test", name: "SFTP · Test", kind: host, files: f}
	app := &localTerm{tabs: []*session{tab}, active: tab, sidebarHidden: true,
		settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	view := ui.NewTester(app.view, 920, 560)
	left, leftOK := view.Find("upload.txt")
	right, rightOK := view.Find("download.txt")
	if !leftOK || !rightOK || left.X+left.W > right.X {
		t.Fatalf("expected local files left and remote files right: left=%+v right=%+v", left, right)
	}
	choose := func(name, action string) {
		t.Helper()
		if err := view.RightClick(name); err != nil {
			t.Fatal(err)
		}
		if err := view.ChooseMenuItem(action); err != nil {
			t.Fatalf("%s: %v; menu: %v", name, err, view.Menu())
		}
	}
	choose("upload.txt", "上传")
	check(remoteDir, "upload.txt", "from local")
	write(localDir, "upload.txt", "overwrite attempt")
	choose("upload.txt", "上传")
	check(remoteDir, "upload.txt", "from local")
	choose("download.txt", "下载")
	check(localDir, "download.txt", "from remote")
	choose("download.txt", "重命名…")
	view.Key(ui.Super, ui.KeyA)
	view.Type("renamed.txt")
	click(t, view, "保存")
	check(localDir, "renamed.txt", "from remote")
	choose("renamed.txt", "复制路径")
	if got, want := view.Clipboard(), filepath.Join(f.local.dir, "renamed.txt"); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
	choose("renamed.txt", "删除…")
	check(localDir, "renamed.txt", "from remote")
	click(t, view, "确认删除")
	if _, err := os.Stat(filepath.Join(localDir, "renamed.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file remains: %v", err)
	}
	choose("upload.txt", "新建文件夹…")
	view.Type("Test folder")
	click(t, view, "创建")
	choose("Test folder", "打开目录")
	if filepath.Base(f.local.dir) != "Test folder" || !view.HasText("空文件夹") {
		t.Fatalf("opening new folder displayed %q", f.local.dir)
	}
	write(f.local.dir, "new local.txt", "created outside the app")
	view.RightClickAt(left.X+40, left.Y+100)
	if err := view.ChooseMenuItem("刷新"); err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Find("new local.txt"); !ok {
		t.Fatal("refresh from the empty area did not update the list")
	}
	choose("new local.txt", "收藏此路径")
	click(t, view, "本地 · 收藏路径")
	if err := view.ChooseMenuItem("收藏当前目录"); err != nil {
		t.Fatal(err)
	}
	click(t, view, "远程 · 收藏路径")
	if err := view.ChooseMenuItem("收藏当前目录"); err != nil {
		t.Fatal(err)
	}
	folder := f.local.dir
	file := filepath.Join(folder, "new local.txt")
	saved, err := loadPreferences(app.settingsPath)
	if err != nil || len(saved.SFTPFavorites[""]) != 2 || len(saved.SFTPFavorites[tab.entryID]) != 1 {
		t.Fatalf("favorites did not persist by pane and connection: %+v, %v", saved.SFTPFavorites, err)
	}
	click(t, view, "上一级")
	click(t, view, "本地 · 收藏路径")
	if err := view.ChooseMenuItem(file); err != nil {
		t.Fatal(err)
	}
	if f.local.dir != folder || f.local.selected != "new local.txt" {
		t.Fatalf("favorite did not select its file: %s %s", f.local.dir, f.local.selected)
	}
	click(t, view, "本地 · 收藏路径")
	if err := view.ChooseMenuItem("移除收藏", file); err != nil {
		t.Fatal(err)
	}
	if len(app.preferences.SFTPFavorites[""]) != 1 {
		t.Fatal("the chosen favorite was not removed")
	}
}

func TestSFTPTransferDoesNotReplaceExistingFile(t *testing.T) {
	source := fileLocation{dir: t.TempDir()}
	target := fileLocation{dir: t.TempDir()}
	const name = "conflict.txt"
	if err := os.WriteFile(source.resolve(name), []byte("new file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target.resolve(name), []byte("existing file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copySFTPFile(context.Background(), source, target, name); !errors.Is(err, os.ErrExist) {
		t.Fatalf("copy error = %v, want file exists", err)
	}
	data, err := os.ReadFile(target.resolve(name))
	if err != nil || string(data) != "existing file" {
		t.Fatalf("existing file changed: %q, %v", data, err)
	}
	items, err := os.ReadDir(target.dir)
	if err != nil || len(items) != 1 {
		t.Fatalf("transfer left temporary files: %v, %v", items, err)
	}
}
