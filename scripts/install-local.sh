#!/usr/bin/env bash
# 编译 Hostern 并替换本机安装的 /Applications/Hostern.app。
#
#   ./scripts/install-local.sh              # 构建后安装，并在需要时重新启动
#   ./scripts/install-local.sh --no-launch  # 只构建和替换，不启动应用
#   ./scripts/install-local.sh --skip-test  # 跳过 go test（默认会先运行测试）
#
# 只构建当前机器架构，不做代码签名、公证、DMG 或更新包：这些由发布工作流完成。
# 替换前会把原应用移动到 /tmp，替换失败时自动还原。

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

install_dir="/Applications"
target="$install_dir/Hostern.app"
launch=1
run_tests=1

usage() {
  sed -n '2,9p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --no-launch) launch=0 ;;
  --skip-test) run_tests=0 ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    echo "未知参数：$1" >&2
    usage >&2
    exit 2
    ;;
  esac
  shift
done

log() { printf '\n==> %s\n' "$*"; }

# mise 不在登录 shell 的 PATH 中，先把它所在目录加进来。
command -v mise >/dev/null 2>&1 || export PATH="$HOME/.local/bin:$PATH"

# 使用 mise.toml 固定的 Go 和 MyGo 版本。
if command -v mise >/dev/null 2>&1; then
  # 只读取激活后的 PATH，不执行 mise install：它每次都会重建用户 shim，
  # 在沙箱或受限环境中会报 "failed to create shim staging directory"，而本项目工具已安装。
  if [[ -f "$root/mise.toml" ]]; then
    eval "$(mise activate bash --shims)"
  fi
else
  log "未找到 mise，使用 PATH 中的 go 和 mygo"
fi
for tool in go mygo; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "错误：未找到 $tool。" >&2
    exit 1
  }
done

# go 的 shim 找不到发行版根目录，改用 mise 中该项目固定的版本。
go_tool="$(mise which go 2>/dev/null || true)"
if [[ -z "$go_tool" ]]; then
  go_tool="$(command -v go)"
fi

export GOROOT
GOROOT="$(dirname "$(dirname "$go_tool")")"

# go 和 mygo 调用 Go 工具链时会继承这些变量。
if ! "$go_tool" build -o /dev/null . >/dev/null 2>&1; then
  # 受限环境（如 sandbox）可能无法写入默认构建缓存，改用临时目录再试一次。
  export GOCACHE="${TMPDIR:-/tmp}/hostern-go-build-cache"
  if ! "$go_tool" build -o /dev/null . >/dev/null 2>&1; then
    echo "错误：Go 无法构建本项目，请检查上面的错误，或在可写目录中运行。" >&2
    exit 1
  fi
  log "使用临时构建缓存 $GOCACHE"
fi

go() { "$go_tool" "$@"; }

arch="$(uname -m)"
case "$arch" in
arm64 | x86_64) ;;
*)
  echo "错误：不支持的架构 $arch。" >&2
  exit 1
  ;;
esac
platform="darwin/$arch"
app_dir="$root/dist/darwin-$arch/Hostern.app"

if [[ $run_tests -eq 1 ]]; then
  log "运行 go test"
  # -short 跳过依赖 libghostty-vt 的测试；真实 PTY 测试需要 /dev/ptmx。
  # 受限终端（如沙箱）无法打开 /dev/ptmx，此时只运行不依赖 PTY 的包。
  if (open /dev/ptmx </dev/null >/dev/null 2>&1); then
    go test -short ./...
  else
    log "当前环境无法打开 /dev/ptmx，跳过需要真实 PTY 的测试"
    go test -short . ./internal/terminal ./internal/terminal/internal/vt
  fi
fi

# 构建失败时直接退出，避免继续替换旧构建。
log "构建 $platform"
# 不生成 DMG：替换本机应用只需要 .app。
if ! mygo build -skip-dmg -platform "$platform"; then
  echo "错误：mygo build 失败，未替换已安装的应用。" >&2
  exit 1
fi
[[ -d "$app_dir" ]] || {
  echo "错误：构建未生成 $app_dir，未替换已安装的应用。" >&2
  exit 1
}
# 确认构建产物属于本次构建，避免把旧产物装到本机。
[[ "$app_dir/Contents/MacOS/Hostern" -nt "$root/main.go" ]] || {
  echo "错误：$app_dir 不是本次构建生成的，未替换已安装的应用。" >&2
  exit 1
}

# 在受限终端或沙箱中可能无法访问 /Applications；此时允许 HOSTERN_INSTALL_DIR 指向其他目录。
if [[ -n "${HOSTERN_INSTALL_DIR:-}" ]]; then
  install_dir="$HOSTERN_INSTALL_DIR"
  target="$install_dir/Hostern.app"
fi
if [[ ! -w "$install_dir" ]] || { [[ -d "$target" ]] && [[ ! -w "$target" ]]; }; then
  echo "错误：没有写入 $target 的权限。可在「系统设置 > 隐私与安全性」中授予终端完全磁盘访问权限，或用 HOSTERN_INSTALL_DIR 指定可写目录。" >&2
  exit 1
fi

was_running=0
if pgrep -xq Hostern; then
  was_running=1
  log "退出正在运行的 Hostern"
  # 只退出用户启动的进程，避免它把更新写入旧版本。
  osascript -e 'quit app id "com.dingxyang.localterm"' >/dev/null 2>&1 || true
  for _ in $(seq 1 40); do
    pgrep -xq Hostern || break
    sleep 0.25
  done
  if pgrep -xq Hostern; then
    echo "错误：Hostern 未能退出，替换已中止。" >&2
    exit 1
  fi
fi

# 旧应用备份到临时目录，同一时刻的备份用一个带 PID 的目录区分。
backup_root="${TMPDIR:-/tmp}"
backup=""
if [[ -d "$target" ]]; then
  backup="$backup_root/Hostern-backup-$PPID"
  rm -rf "$backup"
  log "备份原应用到 $backup"
fi

log "替换 $target"
# ditto 保留扩展属性和符号链接，比 cp -R 更适合复制应用包。合并到已有目录，
# 这样没有 /Applications 写权限、但目录本身可写时也能原地更新。
if ! ditto "$app_dir" "$target.new.$$"; then
  rm -rf "$target.new.$$"
  echo "错误：复制新应用失败，原应用未改动。" >&2
  exit 1
fi
if [[ -e "$target" ]]; then
  mv "$target" "$backup"
fi
if ! mv "$target.new.$$" "$target"; then
  [[ -n "$backup" ]] && mv "$backup" "$target"
  rm -rf "$target.new.$$"
  echo "错误：替换失败，已还原原应用。" >&2
  exit 1
fi

# 新应用由 mygo build 以 ad hoc 方式签在本地，没有隔离属性，可以直接启动。
# 仅在替换前它原本在运行时才启动，避免凭空打开应用。
if [[ $launch -eq 1 ]] && [[ $was_running -eq 1 ]]; then
  log "启动 Hostern"
  open "$target"
fi

log "完成"
printf '已安装：%s\n' "$target"
printf '原应用：%s\n' "${backup:-无（首次安装）}"
