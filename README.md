# PortProxy

**English** · [简体中文](README.zh-CN.md)

A Windows desktop manager for `netsh interface portproxy`, rebuilt with
[mygo](https://mygo.egoist.dev)'s native UI. It is a full reimplementation of
[zmjack/PortProxyGUI](https://github.com/zmjack/PortProxyGUI) and is fully
data-compatible with it.

Requires Go ≥ 1.27.3 (`GOTOOLCHAIN=auto` downloads it). No Node, no cgo.

## Features

- Reads and writes the actual Windows portproxy registry keys
  (`HKLM\SYSTEM\CurrentControlSet\Services\PortProxy\{type}\tcp`).
- Dark sidebar with group management (create / rename / delete / move rules
  between groups), group filtering and live search across all rule fields.
- Status/type filters, removable group chip and clear-all; live search across
  all rule fields; pagination with a remembered page size.
- Dedicated fixed "Actions" column so row edit/delete buttons always line up
  regardless of comment length; inline enable/disable switches.
- Column sorting, grouping, comments, column-width and window-size memory,
  import / export of `config.db`.
- Flush DNS cache, IP Helper service status and start, `SERVICE_CONTROL_PARAMCHANGE`.
- English and 简体中文 UI (follows the system; switch under **More → Language**).
- Single instance; closing the window quits the app (no tray or resident process).
- In-place overwrite upgrades through the NSIS installer, plus signed
  auto-updates (mygo updater, "Check for Updates" in **About** / **More**).

Because writing HKLM needs administrator rights, the app re-launches itself
elevated at startup (one UAC prompt), like the original app's manifest.

## Data compatibility

PortProxy keeps **its own** database at `%APPDATA%\PortProxy\config.db`,
outside the install folder so an overwrite upgrade never removes it. It uses
the exact same SQLite schema as the original PortProxyGUI — `rules`,
`configs`, `__history`, both unique indexes, the default config rows and all
five migration ids — so a `config.db` can be exchanged either way through
**Import** / **Export**.

There is deliberately **no automatic sync**: neither a first-run copy from the
original app's `[MyDocuments]\PortProxyGUI\config.db`, nor mirroring our edits
back into it. The two applications are independent.

## Exposing WSL 2 services to other machines (NAT mode)

WSL 2 runs behind a NAT by default: a service listening inside the Linux
distribution (for example a dev server on `:8080`) is only reachable from the
Windows host itself, not from other computers on the LAN. PortProxy makes
this a two-field edit instead of a `netsh` command line:

1. In WSL, find the current guest address: `wsl hostname -I` (for example
   `172.28.113.42`). WSL 2 assigns a new address after each Windows reboot or
   `wsl --shutdown`, so keep this rule handy for a quick edit.
2. In PortProxy create a rule:
   - **Type** `v4tov4` (or `(Auto)` — IPv4 addresses resolve to it),
   - **Listen on** `0.0.0.0` (or a specific Windows LAN address),
   - **Listen port** `8080`,
   - **Connect to** the WSL address from step 1,
   - **Connect port** `8080`.
3. Other machines can now reach `http://<windows-lan-ip>:8080`.

Two Windows-side notes:

- PortProxy does **not** configure the Windows Firewall. Allow inbound TCP on
  the listen port once, e.g.
  `netsh advfirewall firewall add rule name="PortProxy 8080" dir=in action=allow protocol=TCP localport=8080`.
- The `iphlpsvc` (IP Helper) service must be running — the status bar warns
  and offers to start it if not.

Grouping is handy here: put LAN-facing rules in a "WSL" group and keep
localhost-only experiments in another.

## Commands

| Command | What it does |
| --- | --- |
| `make dev` | Develop with live reload |
| `make build` | Package with `go tool mygo build` |
| `make test` | Run tests; `ui.Tester` drives the view without a window |
| `make lint` | golangci-lint plus `mygo vet` |
| `make format` / `make format-check` | gofmt + golines at 100 columns |

Windows installer (NSIS, in-place upgrade):

```sh
go tool mygo build -platform windows/amd64
# build/windows-amd64/PortProxy.exe
# build/windows-amd64/portproxy-setup-1.4.2-amd64.exe
```

`MYGO_LOCALE=zh-Hans go run .` forces Chinese for one run.
`SNAPSHOT=/tmp/shots go test -run TestSnapshot` renders offscreen PNGs.

## i18n

Catalogs live in `locales/` (`en.json` is the source, `zh-Hans.json` the
translation). `go tool mygo generate` regenerates the typed
`locales/catalog_gen.go` — commit it so plain `go build` / `go test` work.

## Layout

| Path | Purpose |
| --- | --- |
| `internal/data` | SQLite layer, schema/migrations, groups, page-size prefs |
| `internal/win` | Registry, IP Helper service, DNS flush, elevation, single instance |
| `locales/` | i18n catalogs + generated accessors |
| `group_dialogs.go` | New / rename group and delete confirmation dialogs |
| `installer/installer.nsh` | NSIS hook asking a running instance to quit before overwrite |
| `*.go` (root) | mygo view, theme, dialogs, app logic |
