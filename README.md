# Hostern

Hostern 是一个 macOS 终端管理工具。它用连接树管理 SSH 目标和本地工作目录，在同一个窗口中打开多个独立的终端页签。

## 启动

需要 macOS 13 或更新版本及已安装的 mise。项目的 `mise.toml` 固定了 Go 1.27.1 和 MyGo CLI 0.4.5。在仓库根目录运行：

```sh
mise trust
mise install
mise run dev
```

首次运行时，MyGo 会获取并打包匹配当前机器架构的 Ghostty 动态库。运行 `mise run build` 可生成 macOS 应用包。

## 使用

- 启动时会恢复侧栏上次选中的条目。在连接视图中，首个终端以及通过页签栏「＋」、快捷键或侧栏菜单新建的本地终端，都会从选中的本地工作目录启动。没有选中可访问的本地工作目录时，使用用户主目录。
- 点击窗口顶部的侧栏图标可显示或隐藏连接树。右键点击侧栏空白处可新建分组、连接或本地工作目录，也可打开本地终端或隐藏侧栏。
- 点击连接树顶部的「＋」添加分组、远程主机或本地工作目录。选中分组后，新条目默认添加到该分组，勾选「放在根层」可改为顶层。
- 右键点击连接树中的条目，或点击该行的「⋯」，即可编辑或删除该条目。对分组使用新建操作时，条目默认加入该分组。
- 点击连接树顶部的齿轮打开设置。在「外观」中可选择跟随系统、浅色或深色，界面和终端可分别设置字体、字号和加粗。保存后立即更新界面和已打开的终端。
- 双击远程主机打开 SSH 终端。可填写 `~/.ssh/config` 中的别名或主机地址，用户名和端口可选。密码、私钥口令和主机指纹提示在终端中处理。
- 在远程主机的右键菜单或 SSH 项目的「＋」菜单中选择「打开 SFTP」。左栏从本地用户主目录开始，右栏在 SSH 认证后打开远程默认目录。双击文件夹即可进入，也可输入路径跳转。选中文件后可上传或下载，遇到同名文件会提示。右键菜单支持传输、新建文件夹、重命名、删除文件或空文件夹、复制路径和刷新。SFTP 会话会跳过 SSH 启动命令。
- SFTP 两栏顶部的「☆」可收藏当前目录、跳转或移除收藏。也可右键收藏文件或文件夹。打开文件收藏会进入其所在目录并选中文件。远程收藏按连接保存，重启后仍可使用。
- 双击本地工作目录，从该目录启动系统登录 shell。每次打开都会创建独立页签，进程退出后保留终端输出。
- 编辑远程主机或本地工作目录时，可以填写「启动命令」。远程主机在 SSH 连接后执行命令，本地工作目录在指定目录中用本机 shell 执行命令。命令结束后，页签保留输出和退出码。留空时打开交互式终端。
- 连接树保存在应用的本地数据目录中，macOS 正式版为 `~/Library/Application Support/LocalTerm/connections.json`。

## 验证

```sh
mise run test
mise run vet
```

可将 `MYGO_GHOSTTY_VT` 指向本机兼容的 Ghostty 动态库，再运行测试以覆盖真实 PTY、工作目录和终端页签生命周期。

## 本地编译替换

改完代码后，可用脚本编译并替换已安装的 `/Applications/Hostern.app`：

```sh
./scripts/install-local.sh
```

脚本默认先运行 `go test`（跳过会下载 Ghostty 动态库的用例），只构建当前机器架构，不生成 DMG、不做公证，也不签名更新包。替换前它会退出正在运行的 Hostern，把原应用备份到临时目录，替换完成后重新启动。选项：`--no-launch` 不重启应用；`--skip-test` 跳过测试。

脚本会自动使用 `mise.toml` 固定的 Go 和 MyGo。需要写入 `/Applications` 时，请从有相应权限的终端运行；也可用 `HOSTERN_INSTALL_DIR` 指定其他安装目录。

对于已用脚本安装 ad hoc 签名的本地版本，程序内的更新器无法自动更新，请重新运行脚本，或从 Release 下载新版本覆盖安装。

## 发布和更新

首次发布前，将与 `mygo.json` 中公钥配对的签名私钥保存为仓库的 GitHub Actions Secret `MYGO_UPDATER_PRIVATE_KEY`。后续发布需要继续使用同一把密钥，已有用户的应用会用内置公钥验证更新。

提交发布改动后，为该提交创建并推送版本标签，例如：

```sh
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 从标签读取版本，通过测试和静态检查后，为 Apple Silicon 和 Intel 构建 DMG 和已签名的更新包，直接上传到同一条 GitHub Release。两种架构的文件全部上传成功后，工作流会自动公开该 Release。

从 Release 安装的版本会在后台检查更新，也可以从 macOS 的 Hostern 菜单手动检查。早于更新功能的构建需要先从 Release 手动安装一次。

## 致谢

- 侧栏中集中管理多个工作目录的设计，借鉴了 Mayoi 的项目页签。 [Mayoi](https://github.com/sorrycc/Mayoi)
- 应用界面基于 MyGo 的原生 UI，终端组件在 MyGo v0.4.5 的终端插件基础上修改。 [MyGo](https://github.com/egoist/mygo) · [插件许可证](internal/terminal/LICENSE)
- 终端组件使用 Ghostty 的 `libghostty-vt` 解析终端控制序列并维护终端状态。 [Ghostty](https://github.com/ghostty-org/ghostty)
