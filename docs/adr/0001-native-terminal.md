# 使用 MyGo 原生终端组件

LocalTerm 通过 MyGo 的原生界面和 Ghostty 终端组件绘制各个会话，由组件处理 PTY、终端输入和回滚缓冲区。远程会话在 PTY 中启动 macOS 的 OpenSSH，复用用户已有的 SSH 配置与认证流程。

## 考虑过的方案

- WebView 与浏览器终端需要额外维护 PTY 和前端之间的字节流、输入和尺寸同步。原生组件已提供这些能力。

## 影响

macOS 应用包需要包含与 MyGo 版本匹配的 Ghostty 动态库。MyGo 的构建工具负责获取和打包该库。
