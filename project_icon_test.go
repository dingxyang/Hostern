package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLocalProjectIcon(t *testing.T) {
	dir := t.TempDir()
	iconPath := filepath.Join(dir, "icon.png")
	iconFile, err := os.Create(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	fill := color.RGBA{R: 210, G: 45, B: 88, A: 255}
	source := image.NewRGBA(image.Rect(0, 0, 128, 64))
	draw.Draw(source, source.Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)
	encodeErr := png.Encode(iconFile, source)
	if err := iconFile.Close(); err != nil {
		t.Fatal(err)
	}
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	encoded, _, err := readProjectIcon(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(dir, "connections.json")
	if err := saveEntries(storePath, []entry{{ID: "project", Kind: local, Name: "Project", Path: dir, ProjectImage: encoded}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(iconPath); err != nil {
		t.Fatal(err)
	}
	entries, err := loadEntries(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap := entries[0].projectBitmap; bitmap == nil {
		t.Fatal("saved project image failed to load after the original was removed")
	} else if w, h := bitmap.Size(); w != 64 || h != 32 {
		t.Fatalf("stored project image size = %dx%d, want 64x32", w, h)
	}

	app := &localTerm{projectView: true, projectID: "project", storePath: storePath, entries: entries}
	view := ui.NewTester(app.view, 800, 530)
	tile, ok := view.Find("Project")
	if !ok || view.Image().RGBAAt(int(tile.X+tile.W/2), int(tile.Y+tile.H/2)) != fill {
		t.Fatal("the project tile did not render the imported icon")
	}
	if err := view.RightClick("Project"); err != nil {
		t.Fatal(err)
	}
	if err := view.ChooseMenuItem("编辑配置"); err != nil {
		t.Fatal(err)
	}
	click(t, view, "移除本地图标")
	click(t, view, "保存")
	entries, err = loadEntries(storePath)
	if err != nil || entries[0].ProjectImage != "" || entries[0].projectBitmap != nil {
		t.Fatalf("project image removal did not persist: %v", err)
	}
}
