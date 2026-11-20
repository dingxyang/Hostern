package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

const projectIconSize = 64

func readProjectIcon(path string) (string, *ui.Bitmap, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	const maxFileSize = 8 << 20
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return "", nil, err
	}
	if len(data) > maxFileSize {
		return "", nil, errors.New("image exceeds 8 MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return "", nil, errors.New("choose a PNG or JPEG image")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		return "", nil, errors.New("image dimensions must be between 1 and 4096 pixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil, err
	}
	longest := max(config.Width, config.Height)
	bounds := image.Rect(0, 0, max(1, config.Width*projectIconSize/longest), max(1, config.Height*projectIconSize/longest))
	icon := image.NewNRGBA(bounds)
	draw.ApproxBiLinear.Scale(icon, bounds, img, img.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, icon); err != nil {
		return "", nil, err
	}
	return base64.StdEncoding.EncodeToString(encoded.Bytes()), ui.NewBitmap(icon), nil
}

func storedProjectIcon(encoded string) *ui.Bitmap {
	if encoded == "" || len(encoded) > 32<<10 {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width > projectIconSize || config.Height > projectIconSize {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return ui.NewBitmap(img)
}

func (a *localTerm) pickProjectIcon(form *editor) {
	form.picking = true
	title := a.tr("选择本地图标", "Choose a local icon")
	filter := a.tr("图片", "Images")
	errorPrefix := a.tr("无法载入项目图标：", "Could not load project icon: ")
	a.runTask(func() func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win, Title: title,
			Filters: []mygo.FileFilter{{Name: filter, Extensions: []string{"png", "jpg", "jpeg"}}},
		})
		var encoded string
		var bitmap *ui.Bitmap
		if err == nil && len(paths) > 0 {
			encoded, bitmap, err = readProjectIcon(paths[0])
		}
		return func() {
			form.picking = false
			if err != nil {
				form.error = errorPrefix + err.Error()
			} else if bitmap != nil {
				form.entry.ProjectImage, form.entry.projectBitmap, form.error = encoded, bitmap, ""
			}
		}
	})
}
