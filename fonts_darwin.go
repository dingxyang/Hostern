package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebitengine/purego"
)

func systemFontFamilies() []string {
	const frameworks = "/System/Library/Frameworks/"
	ct, err := purego.Dlopen(frameworks+"CoreText.framework/CoreText", purego.RTLD_LAZY)
	if err != nil {
		return nil
	}
	defer purego.Dlclose(ct)
	cf, err := purego.Dlopen(frameworks+"CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY)
	if err != nil {
		return nil
	}
	defer purego.Dlclose(cf)

	var families func() uintptr
	var count func(uintptr) int
	var valueAt func(uintptr, int) uintptr
	var length func(uintptr) int
	var utf8String func(uintptr, *byte, int, uint32) bool
	var release func(uintptr)
	var fileURL func(uintptr, *byte, int, bool) uintptr
	var registerFont func(uintptr, uint32, uintptr) bool
	purego.RegisterLibFunc(&families, ct, "CTFontManagerCopyAvailableFontFamilyNames")
	purego.RegisterLibFunc(&count, cf, "CFArrayGetCount")
	purego.RegisterLibFunc(&valueAt, cf, "CFArrayGetValueAtIndex")
	purego.RegisterLibFunc(&length, cf, "CFStringGetLength")
	purego.RegisterLibFunc(&utf8String, cf, "CFStringGetCString")
	purego.RegisterLibFunc(&release, cf, "CFRelease")
	purego.RegisterLibFunc(&fileURL, cf, "CFURLCreateFromFileSystemRepresentation")
	purego.RegisterLibFunc(&registerFont, ct, "CTFontManagerRegisterFontsForURL")

	// Some user font files are not exposed in the current Core Text catalog.
	// Register them for this process so both the UI and terminal can use them.
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, "Library", "Fonts")
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			switch strings.ToLower(filepath.Ext(f.Name())) {
			case ".ttf", ".otf", ".ttc", ".otc":
				if f.IsDir() {
					continue
				}
				path := []byte(filepath.Join(dir, f.Name()))
				if url := fileURL(0, &path[0], len(path), false); url != 0 {
					const processScope = 1
					registerFont(url, processScope, 0)
					release(url)
				}
			}
		}
	}

	list := families()
	if list == 0 {
		return nil
	}
	defer release(list)
	n := count(list)
	names := make([]string, 0, n)
	for i := range n {
		name := valueAt(list, i)
		buf := make([]byte, length(name)*4+1)
		const utf8Encoding = 0x08000100
		if utf8String(name, &buf[0], len(buf), utf8Encoding) {
			raw, _, _ := bytes.Cut(buf, []byte{0})
			if len(raw) > 0 && raw[0] != '.' {
				names = append(names, string(raw))
			}
		}
	}
	return names
}
