package main

import (
	"maps"
	"path"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo/ui"
)

type sftpFavorite struct {
	Path string `json:"path"`
	File bool   `json:"file,omitempty"`
}

func sftpFavoriteGroup(s *session, p *filePane) string {
	if p == &s.files.local {
		return ""
	}
	return s.entryID
}

func (a *localTerm) sftpFavoritesMenu(c *ui.Context, m *ui.Menu, s *session, p *filePane) {
	a.sftpFavoriteItem(c, m, s, p, sftpFavorite{Path: p.dir}, a.tr("收藏当前目录", "Add Current Folder to Favorites"))
	favorites := a.preferences.SFTPFavorites[sftpFavoriteGroup(s, p)]
	if len(favorites) == 0 {
		return
	}
	m.Separator()
	for _, favorite := range favorites {
		if m.Item(favorite.Path).Disabled(s.files.busy).Chosen() {
			dir, name := favorite.Path, ""
			if favorite.File {
				dir, name = path.Dir(dir), path.Base(dir)
				if p == &s.files.local {
					dir, name = filepath.Dir(favorite.Path), filepath.Base(favorite.Path)
				}
			}
			a.refreshFileSelection(s.files, p, dir, name, nil)
		}
	}
	m.Submenu(a.tr("移除收藏", "Remove Favorite"), func(m *ui.Menu) {
		for _, favorite := range favorites {
			if m.Item(favorite.Path).Chosen() {
				a.setSFTPFavorite(c, s, p, favorite, false)
			}
		}
	})
}

func (a *localTerm) sftpFavoriteItem(c *ui.Context, m *ui.Menu, s *session, p *filePane, favorite sftpFavorite, label string) {
	favorites := a.preferences.SFTPFavorites[sftpFavoriteGroup(s, p)]
	saved := slices.ContainsFunc(favorites, func(item sftpFavorite) bool { return item.Path == favorite.Path })
	if m.Item(label).Checked(saved).Disabled(favorite.Path == "" || saved || a.settingsPath == "").Chosen() {
		a.setSFTPFavorite(c, s, p, favorite, true)
	}
}

func (a *localTerm) setSFTPFavorite(c *ui.Context, s *session, p *filePane, favorite sftpFavorite, add bool) {
	group := sftpFavoriteGroup(s, p)
	next := a.preferences
	next.SFTPFavorites = maps.Clone(next.SFTPFavorites)
	if next.SFTPFavorites == nil {
		next.SFTPFavorites = make(map[string][]sftpFavorite)
	}
	favorites := slices.Clone(next.SFTPFavorites[group])
	if add {
		favorites = append(favorites, favorite)
	} else {
		favorites = slices.DeleteFunc(favorites, func(saved sftpFavorite) bool { return saved == favorite })
	}
	if len(favorites) == 0 {
		delete(next.SFTPFavorites, group)
	} else {
		next.SFTPFavorites[group] = favorites
	}
	if err := saveJSON(a.settingsPath, next); err != nil {
		c.Toast(a.tr("无法保存收藏：", "Could not save favorites: ") + err.Error())
		return
	}
	a.preferences = next
}
