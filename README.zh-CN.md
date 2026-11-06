# PortProxy

[English](README.md) · **简体中文**

用 [mygo](https://mygo.egoist.dev) 原生 UI 重写的 Windows 端口转发管理工具，
是 [zmjack/PortProxyGUI](https://github.com/zmjack/PortProxyGUI) 的完整实现，
并且与它的数据完全兼容。

需要 Go ≥ 1.27.3（`GOTOOLCHAIN=auto` 会自动下载），不需要 Node，也不需要 cgo。

## 功能

- 直接读写 Windows 的端口代理注册表
  `HKLM\SYSTEM\CurrentControlSet\Services\PortProxy\{类型}\tcp`。
- 深色侧边栏，可创建 / 重命名 / 删除分组、把规则移动到分组、按分组筛选，
  并支持对所有规则字段实时搜索。
- 按状态 / 类型筛选、可移除的分组标签、一键清除筛选；对所有规则字段实时搜索；
  支持分页并记住每页条数。
- 操作按钮放在独立的固定「操作」列，备注再长也不会导致按钮排不齐；
  行内开关直接启用 / 禁用。
- 列排序、分组、备注，记住列宽和窗口大小，导入 / 导出 `config.db`。
- 清理 DNS 缓存、检测并启动 IP Helper 服务、发送 `PARAMCHANGE` 通知。
- 中英文界面（默认跟随系统，在「更多 → 语言」里切换）。
- 单例启动；关闭窗口即退出，不驻留、没有托盘图标。
- NSIS 安装包支持覆盖安装（就地升级），并通过 mygo 更新器提供签名自动更新
  （「关于」或「更多 → 检查更新」）。

写 HKLM 需要管理员权限，因此启动时会自动以管理员身份重新启动一次
（一次 UAC 提示），和原项目的 manifest 行为一致。

## 数据兼容性

PortProxy 把**自己的**数据库放在 `%APPDATA%\PortProxy\config.db`，
不在安装目录里，覆盖安装不会丢。表结构与原项目 PortProxyGUI 完全一致：
`rules`、`configs`、`__history`、两个唯一索引、三条默认配置以及全部五个
迁移记录，因此 `config.db` 可以通过「导入 / 导出」在两个软件之间互通。

**不做自动同步**：既不会在首次启动时复制原项目
`[MyDocuments]\PortProxyGUI\config.db` 的内容，也不会把我们的修改回写过去。
两个软件各自独立。

## 让 WSL 2（NAT 模式）的端口给局域网其它电脑访问

WSL 2 默认走 NAT：Linux 子系统里监听的服务（比如 `:8080` 的开发服务器）
只能从 Windows 本机访问，局域网里的其它电脑连不上。用 PortProxy 填两个
字段就能搞定，不用敲 `netsh`：

1. 在 WSL 里查当前地址：`wsl hostname -I`（例如 `172.28.113.42`）。
   Windows 重启或 `wsl --shutdown` 后 WSL 2 会换地址，所以把这条规则留着，
   改一下「转发地址」就行。
2. 在 PortProxy 新建规则：
   - **类型** `v4tov4`（选「(自动)」也会因为 IPv4 地址解析成它），
   - **监听地址** `0.0.0.0`（或 Windows 的某个局域网地址），
   - **监听端口** `8080`，
   - **连接地址** 第 1 步拿到的 WSL 地址，
   - **连接端口** `8080`。
3. 其它电脑访问 `http://<windows局域网IP>:8080` 即可。

两个 Windows 侧的注意点：

- PortProxy **不配置 Windows 防火墙**。需要放行监听端口的入站 TCP，例如：
  `netsh advfirewall firewall add rule name="PortProxy 8080" dir=in action=allow protocol=TCP localport=8080`。
- `iphlpsvc`（IP Helper）服务必须运行；状态栏会提示并可以一键启动。

分组在这里也很好用：把对局域网开放的规则放到「WSL」分组，
只供本机测试的放另一组。

## 命令

| 命令 | 作用 |
| --- | --- |
| `make dev` | 热重载开发 |
| `make build` | 用 `go tool mygo build` 打包 |
| `make test` | 测试，界面用 `ui.Tester` 无窗口驱动 |
| `make lint` | golangci-lint 加 `mygo vet` |
| `make format` / `make format-check` | gofmt + golines，每行最多 100 列 |

生成 Windows 安装包（支持覆盖升级）：

```sh
go tool mygo build -platform windows/amd64
# build/windows-amd64/PortProxy.exe
# build/windows-amd64/portproxy-setup-1.4.2-amd64.exe
```

`MYGO_LOCALE=zh-Hans go run .` 可以单次强制中文。
`SNAPSHOT=/tmp/shots go test -run TestSnapshot` 离屏渲染界面 PNG。

## i18n

文案在 `locales/`（`en.json` 为源语言，`zh-Hans.json` 为中文）。
`go tool mygo generate` 重新生成带类型的 `locales/catalog_gen.go`，
提交它以保证直接 `go build` / `go test` 也能工作。

## 目录

| 路径 | 作用 |
| --- | --- |
| `internal/data` | SQLite 层、建表与迁移、分组、分页偏好设置 |
| `internal/win` | 注册表、IP Helper 服务、DNS、提权、单例 |
| `locales/` | i18n 文案与生成的访问函数 |
| `group_dialogs.go` | 新建 / 重命名分组与删除确认对话框 |
| `installer/installer.nsh` | NSIS 钩子：覆盖安装前通知正在运行的实例退出 |
| 根目录 `*.go` | mygo 界面、主题、对话框、业务逻辑 |
