# 本地开发与实例维护

## 工具和构建

在工作树根目录运行命令。Go 版本由 `go.mod` 的 `go 1.26.0` 指定；前端使用 npm 和 `web/package-lock.json`，Node 范围由 `web/package.json` 的 engines 指定（`^22.22.2 || ^24.15.0 || >=26`），CI 沿用 Node 22 的最新修订。安装所用 Node 必须满足锁定依赖的 engines 要求。新工作树先安装前端依赖；运行 `make test` 前也安装桌面测试依赖：

```sh
npm --prefix web ci
npm --prefix desktop ci
make build
```

`make build` 先执行 `tsc --noEmit && vite build`，生成 `internal/web/dist/app`，再构建根目录二进制 `inftask`。`internal/web/dist/.gitkeep` 受 Git 跟踪，`//go:embed all:dist` 没构建前端时也有匹配，所以 `go vet ./...`、`go build ./...` 和 `go test ./...` 不依赖前端构建；这样构建的服务只有 API 和 MCP 可用，页面返回 404。Vite 只清空 `dist/app`，构建输出不进入 Git。前端独立检查入口为 `npm --prefix web run typecheck`、`npm --prefix web run lint`、`npm --prefix web run test`；`make test` 统一执行构建、Hooks lint、组件测试和 Go 测试。不要修改生成的 dist 或删除 `.gitkeep`，也不要把二进制或 node_modules 提交。

## macOS 云端客户端

`desktop/` 是连接正式云端的 Electron 客户端，不是本地 Go 服务；开发启动、arm64 DMG、会话与签名边界见 [桌面客户端](../desktop/README.md)。它独立构建，不依赖网页/Go 构建，也不打开业务 SQLite 或监听端口。正式实例只读登录页检查不代表真实 UC 身份登录完成；自动行为验证使用随机 loopback HTTPS fixture 和临时 profile，不提交业务数据或真实凭据。

## 隔离运行

```sh
go run ./cmd/inftask -data "$(mktemp -d)/dev.db"
```

默认地址 `127.0.0.1:8787`；打开该地址查看页面，`GET /api/health` 返回 `{"status":"ok"}`。新库没有 workspace，页面会提示先建一个；workspace 的页面在 `/<slug>/`，REST 在 `/api/w/<slug>/`，MCP 在 `/w/<slug>/mcp`：

```sh
curl -X POST http://127.0.0.1:8787/api/workspaces -H 'Content-Type: application/json' -d '{"slug":"default","name":"Default"}'
```

新 workspace 没有 team，建票前先建一个：页面侧边栏 Teams 旁的 “+”，或 `/w/<slug>/mcp` 的 `create_team`，比如 `{"key":"AL","name":"InfTask"}`；[造数脚本](VALIDATION.md#ui-检查)会自己建 workspace `default` 和 team `AL`。服务日志会打印监听地址。数据库及 backups 均在这次临时目录内，Ctrl-C 停止开发服务；仅在确认不再需要时清理自己创建的临时目录。不要用维护者的 8700 实例验证开发写入。

多个 agent 并行开发时 8787 可能已被占用。启动前先检查端口，有输出就表示被占用：

```sh
lsof -nP -iTCP:8787 -sTCP:LISTEN
```

被占用时换一个同样检查过、没有输出的空闲端口，用 `-addr` 指定，并用这个地址访问页面和 API：

```sh
go run ./cmd/inftask -data "$(mktemp -d)/dev.db" -addr 127.0.0.1:8791
```

永远不用 8700，也不要为了开发终止别人或维护者实例的进程。

前端热更新另开终端运行 `npm --prefix web run dev`，访问 Vite 打印的地址。只有 `/api` 和 `/auth` 代理（写在 `web/vite.config.ts`），目标默认是 `http://127.0.0.1:8787`；开发服务换了端口时用环境变量 `INFTASK_API_URL` 指向它，例如 `INFTASK_API_URL=http://127.0.0.1:8791 npm --prefix web run dev`；不要假设 Vite 提供 `/w/<slug>/mcp`。默认开发无需 GitHub token；仅测试 GitHub 集成时才配置对应参数，行为边界见 [安全](rules/SECURITY.md)。

页面看起来仍是旧版时，区分 Vite 与 Go 嵌入页面：后者需重新构建并重启自己启动的开发进程。数据库升级/恢复失败的处理见 [可靠性](rules/RELIABILITY.md)。

## MCP 工具没加载时

正式实例使用 `https://harnessagent.tokensinfinity.com/w/default/mcp` 和 UC OAuth。Claude Code、Codex 均通过用户级 `inftask-cloud` 接入；仓库不再重复注册旧的本地 `inftask` 服务。工具未加载时先检查用户级配置与登录状态，再重新打开会话。不要把请求退回已停用的 8700 或直接修改保留的本机数据库。

只有持有有效 UC access token 的客户端才能经 HTTPS 发 HTTP JSON-RPC `tools/call`。请求需要 `Authorization: Bearer <token>`、`Content-Type: application/json` 和 `Accept: application/json, text/event-stream`；凭据由已认证客户端管理，不从聊天、日志或仓库文件获取。具体接入契约见下方 [远程 MCP](#远程-mcp)。

本地开发的 MCP 在自己的临时服务 `/w/<slug>/mcp` 上；`params.name` 是工具名，`params.arguments` 是参数，`tools/list` 可列出工具。成功时结果在 `result.structuredContent`，工具失败时 `result.isError` 为 `true`。开发环境不得使用正式数据或凭据。

## 仓库内 Agent 接票

本仓库的 `.codex/hooks.json` 与 `.claude/settings.json` 在 `delegate_issue` 执行前调用 `scripts/delegate-hook.mjs`。需要 Node 22+；`scripts/run-delegate-hook` 保留原 PATH 优先级，再追加已有的常见工具目录，不读取 shell profile、不安装工具。Codex 先信任仓库配置层，再按客户端提示信任该 hook，不在真实配置中绕过信任。工具调用仍须通过宿主的 MCP 审批；hook 的 allow 不替代 Codex 的日常审批策略。

委派前先指定人类 assignee；Service 对每次调用（包括重复登记）检查这一前置条件，未分配时返回 `assign a human assignee before delegating this issue`。清空负责人保留已有来源历史，重新分配后可继续委派。临时 host smoke 的测试票也必须先通过本地 `bootstrap-access` 初始化测试成员，再指定该成员为负责人；不使用维护者身份或真实库造数。

只适配 MCP 名称 `inftask`、`inftask-cloud`、`inftask_cloud` 与 Claude 插件的 `plugin_inftask_inftask` 工具前缀；自定义其他名称不会注入。这不是插件分发：`plugins/inftask/` 和 marketplace 保持原样。仓库主/子 Agent 按 [ticket skill](../.agents/skills/ticket/SKILL.md#agent-participation-repository-dogfooding) 只提供票号；hook 整体替换 `client_context`，不依赖 status、Git remote 或是否为 root agent。

模型端新增工具 schema 需重连 MCP 后加载。生产实例尚未部署该工具时不能宣称接票记录成功；既有任务流程仍可使用。开发和 host smoke 指向隔离 loopback MCP，不向正式实例写入测试委派。hook 不读取 MCP token 或 transcript，不自行联网；元数据随客户端已有认证连接发往当前 InfTask workspace。

记录字段为 `agent`（codex/claude）、`session_id`、可选 `agent_id`/`agent_type`、`hostname`；hostname 是运行 hook 的机器名，不主动查 IP。子 Agent 的 session ID 可能是父会话 ID，具体子 Agent 以宿主提供的 `agent_id` 区分。缺 session 或非法字段会明确拒绝，不生成假 ID。会话/Agent ID 只是来源，不证明进程仍存活或具备独立授权。

## 从 Linear 快照导入

导出脚本 `scripts/linear/export.py` 和 `inftask import-linear` 的用法、输出文件、key 位置、映射和写入前检查见 [README](../README.md#import-from-linear)。快照含团队内部数据，放在仓库外（维护者的快照在 `~/Library/Application Support/inftask/linear-export/<日期>/`，目录权限 700），不进提交、fixture、ticket 或 PR；评论和 PR 里只写数量和结论。导出脚本调用 Linear API，只在需要新快照时运行。

导入前先在副本上试：`sqlite3 <数据库> ".backup '<临时目录>/copy.db'"`，对副本运行导入并核对数量，用完删除副本。

导入维护者实例时，在已合并 `import-linear` 的主检出执行。目标 team 要先存在，且 `next_number` 大于快照里的最大编号。

```sh
git switch main && git pull
make build
launchctl bootout gui/$(id -u)/io.github.dengnaichen.inftask
db="$HOME/Library/Application Support/inftask/inftask.db"
sqlite3 "$db" ".backup '$HOME/Library/Application Support/inftask/backups/before-linear-import.db'"
./inftask import-linear -data "$db" -workspace default -team <team key> -from <快照目录>
install -m 755 inftask ~/.local/bin/inftask
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.dengnaichen.inftask.plist
```

依次是停实例、备份、导入、装新二进制并启动。导入像服务启动一样先执行未完成的迁移（迁移前副本照常写在 `backups/`），然后在一个事务里写入；非 0 退出时什么都没写，按报错处理后重跑，已占用的编号会列在报错里。`bootstrap` 紧跟 `bootout` 时可能报 `Bootstrap failed: 5: Input/output error`，等几秒重试。导入成功后要撤销：停实例，删除数据库的 `-wal`、`-shm`，用 `before-linear-import.db` 覆盖数据库，再启动；此后的写入会丢失。

## 合并后更新正式实例

正式实例在云端。合并后在主检出的 `main` 上，先运行 `deploy/release.sh --dry-run <ssh-host>` 核对将执行的命令，再运行 `deploy/release.sh <ssh-host>` 发布；`<ssh-host>` 是下一段所说的正式主机别名。脚本先备份，再只重建 app，保留现有 Compose project、数据目录和 Caddy，验证失败时不自动回滚；成功后更新服务器的 `deployment.json` 和 `/opt/inftask/RUNBOOK.txt`。服务器布局、镜像标签、备份和回滚见 [deploy/README](../deploy/README.md)。本机 launchd 已停用，不能再用 `launchctl kickstart` 更新正式实例。本机库和 plist 仅供有明确授权的回滚使用。

正式实例所在的主机就是 `harnessagent.tokensinfinity.com` 解析到的机器，连接前先用 `dig +short harnessagent.tokensinfinity.com` 核对地址。维护者本机的 SSH 别名 `harness` 指向另一台服务器，那台机器上没有 `/opt/inftask`，不得用它部署或运维 InfTask。建议在自己的 `~/.ssh/config` 里给正式主机单独建一个别名，不要复用 `harness`。登录用户和密钥由维护者提供，不写进仓库。

分支与工作树清理遵循 [git-workflow](../.agents/skills/git-workflow/SKILL.md)。本地开发仍使用隔离临时库。

## 云端团队部署

云端是一个进程、一份 SQLite。InfOne UC 负责确认人的身份，InfTask 自己保存工作区成员和 Admin/Member 角色，不复用 HarnessKit workspace membership。UC 登录成功不自动加入已有 workspace；通过 Settings → Members 中管理员生成的邀请链接加入。成员被停用后，即使 UC token 或网页登录仍有效，后续工作区请求也返回 403。

### 初始化已有工作区的管理员

升级先停止业务写入、备份数据库，再用新二进制显式指定已有工作区与已核实的 UC `sub`。`sub` 可由对应用户已登录的 `/auth/session` 响应核实，不能用 email 或 display name 替代：

```sh
./inftask bootstrap-access -data <已有数据库路径> -workspace default -subject <已核实的UC-sub>
```

命令会先执行迁移，并且只允许给没有任何成员的已有工作区添加一个 Admin；工作区不存在、已初始化或缺参数均失败，不覆盖成员。每个已有工作区分别初始化。升级后尚未初始化的工作区对云端用户不可访问；不要以“首个登录用户”推断管理员。新建工作区在创建事务内授予创建者 Admin。

完成初始化再启动服务，由该管理员打开 Settings → Members 生成邀请并手动分享；每次生成新链接都会让旧链接失效。受邀者登录 UC 后点击 Accept invitation 成为 Member，被停用者须由管理员恢复。此命令是部署步骤，不由常规开发测试或发布脚本自动选择身份执行。

### 服务端配置

| 环境变量 | 含义 |
| --- | --- |
| `INFTASK_PUBLIC_URL` | 外部 HTTPS origin，不带路径，回调固定为该地址加 `/auth/callback` |
| `INFONE_UC_BASE_URL` | UC 的 HTTPS origin；authorize/token/userinfo 使用 `/user_center/v1/oauth2/` 下的入口 |
| `INFONE_UC_CLIENT_ID` | 暂时使用现有 HarnessKit 的 UC client ID |
| `INFONE_UC_CLIENT_SECRET` | 对应 secret，只注入服务端环境，不写入源码或前端 |
| `INFONE_SHARE_DIR` | 内含 `infone.pub` 的目录；文件必须是单个 PKIX `PUBLIC KEY` PEM，支持 RSA/RS256 或 P-256/ES256 |
| `INFONE_UC_JWT_ISSUER` | 可选；配置后严格检查 `iss`。当前 UC token 可以没有 `iss`，信任根是配置的公钥 |
| `GITHUB_TOKEN` | 已配置 repository 的 PR 读取凭据 |
| `GITHUB_WEBHOOK_SECRET` | webhook HMAC secret，和 UC client secret 不同；未配置时不挂载 webhook 路由 |

UC 地址与外部地址只允许 HTTPS；本地假 UC 验证允许 loopback HTTP。所有 UC 配置都未设置时保留本地无认证模式，但只能监听 loopback；任何部分配置都必须补齐，不能静默降级成无认证。云端反向代理必须保留外部 Host，避免破坏已有同源写入检查。

在 UC 对当前 client 登记精确的 `https://<域名>/auth/callback`。网页跳转 UC 后由 Go 后端换 token、验证签名、`client_id`、`type=access`、过期时间和 userinfo subject，随后发放 InfTask 的 opaque HttpOnly cookie。共享 client 意味着两个产品无法仅凭 UC token 区分用户最初在哪个产品登录；独立 client 可后续替换配置。

### 容器与 HTTPS

前提：Docker Engine、Compose 插件、解析到部署主机的域名、开放 80/443、UC 配置和可读的验证公钥。不要在维护者实例上试部署。将变量放入仓库外受保护的环境文件（权限 `0600`），Compose 另需 `INFTASK_DOMAIN` 和 `GITHUB_REPOS=<slug>=<owner>/<name>[,<slug>=<owner>/<name>...]`（例如 `GITHUB_REPOS=default=owner/one,default=owner/two`，仓库的 PR 只关联该 workspace 的票）；`INFONE_SHARE_DIR` 是宿主的公钥目录。环境文件不要提交。

```sh
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml config --quiet
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml build
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml up -d
```

Caddy 自动申请证书并反向代理到容器内部 8787；app 不向宿主发布 8787。`/api/health` 公开且只返回健康状态；`/webhooks/github` 通过 GitHub 签名认证；其他业务入口需要 UC 身份或网页会话。镜像以 UID 10001 运行，数据库和 `backups` 都在 `/data` 命名卷中。每日备份仍是同一磁盘上的一致性快照，不能抵御卷丢失，应将已完成的日期备份另行复制到异地存储。

更新前先保留卷和可恢复的日期备份，再重新构建并 `up -d`；不得运行删除卷的命令。恢复步骤：停止 app，保留现有数据库和 WAL/SHM 供调查，将选定一致性备份恢复为 `/data/inftask.db`，清理该库对应的旧 WAL/SHM，确保 UID 10001 可读写，再启动。备份恢复遵循 [可靠性](rules/RELIABILITY.md)。旧版程序回滚前需核对 schema 兼容性，不直接对已升级数据库运行旧二进制。

网页登录会话最长 30 天，浏览器只持有随机 HttpOnly cookie。服务端按 UC access token 的签名 `exp` 判断何时需要刷新；过期后的下一次请求用 refresh token 向 UC 续期，轮换后的 refresh token 写回服务端，身份必须保持同一 subject。UC 拒绝续期时撤销会话并要求重新登录；网络故障、限流和 UC 5xx 返回 503，保留会话供后续请求恢复，不在后台循环保活。

网页会话加密保存在 `-data` 路径加 `.sessions` 的旁路文件中（默认 `/data/inftask.db.sessions`），权限 `0600`，原子替换。密钥由 UC client secret、client ID 和外部 URL 作专用派生；正常部署重启保留登录，注销持久撤销。更换上述配置或文件损坏会使加载失败；维护者需在停服时保留旧文件后移走，启动后重新登录，不能静默忽略损坏。升级前的纯内存会话需重新登录一次。该文件不是 SQLite 业务备份，不应把旧会话文件随旧业务库回滚，否则可能恢复已注销的会话。保持单进程单实例，不能让两个服务同时写同一文件。

待完成登录和 MCP 授权码仍只在内存里，重启需重走未完成的登录。已发给 MCP 的 UC token 按自身有效期使用，由宿主刷新。UC 禁用账号不会立即撤销尚未到期的离线 access token；下一次刷新由 UC 决定是否允许。

### 远程 MCP

每个 workspace 的 MCP 是 `/w/<slug>/mcp`，不接受网页 cookie，只接受有效 UC access token。未认证响应带 `WWW-Authenticate` 指向 `/.well-known/oauth-protected-resource/w/<slug>/mcp`；对应授权服务器 metadata 是 `/.well-known/oauth-authorization-server`，授权和换 token 的 `/mcp/oauth/authorize`、`/mcp/oauth/token` 所有 workspace 共用。

MCP host 配置服务 URL `https://<域名>/w/<slug>/mcp`、public client ID `inftask`（与后端使用的 UC client ID 不同），使用 Authorization Code + S256 PKCE；本版不提供动态 client 注册。授权请求必须带 `resource=https://<域名>/w/<slug>/mcp`，换 token 时的 `resource` 必须与授权时相同，callback 只接受带端口的 `http://127.0.0.1`、`http://[::1]` 或 `http://localhost` URL，不接受 query 或 fragment。后端将其转到 UC 的固定网页回调。公网托管 MCP host 的 HTTPS callback 不在本版范围。

客户端配置（2026-10-11 用 Claude Code 2.1.296 和 Codex 0.162.1 对 `https://harnessagent.tokensinfinity.com` 实测，见 AL-37）。两者都用预注册的 public client ID `inftask`，第一次连接时在浏览器完成 UC 登录：

- Claude Code：`claude mcp add --transport http --scope user --client-id inftask <名称> https://<域名>/w/<slug>/mcp`。已经在运行的会话不会重读 user 级 MCP 配置，先重启会话（`claude --continue` 可接着原对话），再在 `/mcp` 里选该服务并 Authenticate。
- Codex：`codex mcp add <名称> --url https://<域名>/w/<slug>/mcp --oauth-client-id inftask`，再 `codex mcp login <名称>`。不要加 `--oauth-resource`：Codex 会自动带上 `resource`，再配一次会让授权请求出现两个 `resource`，后端按重复字段返回 `{"error":"invalid_request"}`。
- 也可以安装仓库自带的 agent 插件，它带好了同样的 MCP 配置，见 [README 的 Agent plugin 一节](../README.md#agent-plugin)。

后端的 `/mcp/oauth/token` 接受 form-urlencoded：code grant 必须有 `grant_type`、`client_id`、`resource`、`code`、`redirect_uri`、`code_verifier`；refresh grant 必须有 `grant_type`、`client_id`、`resource`、`refresh_token`。重复、额外或空字段被拒绝。MCP 后端只在短期内存 grant 中保存 UC token，完成 PKCE 校验后将 access/refresh token 交给 host；secret 不交给 host。返回的 `expires_in` 取已验签 access token 的实际剩余时间，避免授权码等待期间的寿命偏差。上游网络故障、429/5xx 返回 `503 temporarily_unavailable`，不会误报成必须重新登录的 `invalid_grant`；UC 永久拒绝刷新仍返回 `invalid_grant`。失效 bearer 的 401 challenge 带 `error="invalid_token"`，供宿主识别刷新。直接持有合法 access token 的客户端也可用 `Authorization: Bearer ...` 访问 REST/MCP。

### 验证边界

`go test ./internal/ucauth ./internal/httpapi ./internal/app ./cmd/inftask` 覆盖假 UC、签名、回调绑定、PKCE、注销、入口保护和 webhook 验签；不证明真实 UC 公钥/client/回调匹配。上线验收还需真实 UC 浏览器登录与退出、远程 MCP host 的 tool call 和 refresh、GitHub Delivery 成功，以及临时卷上的重启保留与备份恢复。不得以构建成功代替这些检查。

### 本机数据迁入公网实例（AL-34）

正式迁移以维护者指定的数据源为准，以下步骤用于切换与恢复。迁移前获得操作授权；先验证两端远程 MCP，再冻结两边写入。云端成为唯一正式写入入口后，本机库仅作回滚保留，不允许两边继续独立写入。

1. **准备与演练。** 核实本机 TOK 导入已经实际完成，不能用 AL-29 的副本验收代替。记录当前版本、数据库路径、launchd plist、服务器 Compose project/文件/卷名，以及各表计数和逐票状态。对运行中的库用 SQLite `.backup` 创建隔离副本，在空闲 loopback 端口用副本运行新版本，绑定两个仓库并注入真实 `GITHUB_TOKEN`，不启用 linkback。首次同步会读取历史 PR；等待两个仓库的 `github_sync` 游标建立且无同步失败，再比较同步前后 TOK 状态，列出 identifier、原状态、新状态供维护者确认。演练不用 8700、不访问正式库写入。检查 `PRAGMA integrity_check` 为 `ok`、`PRAGMA foreign_key_check` 无输出。
2. **准备凭据。** Fine-grained PAT 限定两个仓库的 Pull requests 读取权限和 Metadata 读取权限。另生成随机 webhook secret；两者只保存在仓库外 `0600` 环境文件。部署变量改为 `GITHUB_REPOS`，旧 `GITHUB_REPO` 不再被 Compose 读取：

   ```dotenv
   GITHUB_REPOS=default=DengNaichen/inftask,default=DengNaichen/harnesskit-shared-memory
   ```

   `GITHUB_TOKEN`、`GITHUB_WEBHOOK_SECRET` 另填真实值，不提交。保持 `-github-linkback` 关闭。现有服务器部署先读 `/opt/inftask/RUNBOOK.txt` 和 `deployment.json`，在原 Compose project 与数据卷上更新 app 参数及环境；不要直接启动仓库的完整 Compose 去争用现有 80/443 或意外创建新数据卷。
3. **冻结写入并备份。** 在约定维护窗口暂停所有 agent 和人工写入。用 `launchctl bootout gui/$(id -u)/io.github.dengnaichen.inftask` 停本机服务，保留 plist。对本机库执行 `sqlite3 <本机数据库> ".backup '<唯一备份路径>'"`，保留这份切换前基线。停止服务器 app（保留代理），对服务器原库也做一致性备份，并保留原库及对应 WAL/SHM、版本和环境文件。核对两份备份的完整性及校验和后再替换；不得删除数据卷。
4. **安装数据与版本。** 将本机一致性备份传至服务器受保护的暂存路径，核对传输前后校验和。在 app 停止时把备份安装为已确认数据卷内的 `/data/inftask.db`，移走该路径原有的 `inftask.db-wal` 和 `inftask.db-shm`（旧文件已保留），设置数据库及数据目录归 UID 10001 可读写。使用支持本机 schema 的新镜像和准备好的环境，先运行现有部署的 Compose `config --quiet`，再启动 app。不要输出带真实凭据的完整 Compose config。
5. **验证数据与同步。** 冻结期间的本机快照是比较基线：公网 AL/TOK 票数、评论、关系、链接计数必须一致，抽查正文和评论作者；状态变化必须与演练确认结果一致。检查健康、真实 UC 网页登录、数据库完整性、两个仓库的同步游标和错误日志。若失败，停止公网 app，按下面的回滚步骤处理，不切 agent 配置。
6. **创建 webhook。** 两个 GitHub 仓库分别进入 Settings → Webhooks → Add webhook。Payload URL 为 `https://harnessagent.tokensinfinity.com/webhooks/github`，Content type 为 `application/json`，Secret 与服务端 `GITHUB_WEBHOOK_SECRET` 完全一致，启用 SSL verification，选择 Let me select individual events 后只勾 Pull requests。启用后初始 `ping` 返回 204；实际已配置仓库的 `pull_request` delivery 才应返回 202。在 Recent Deliveries 检查该事件和签名验证结果，并核对随后发生同步。轮询每分钟补漏，webhook 不代替读取凭据。
7. **远程 MCP 与最终切换。** 先在 Claude Code 和 Codex 各完成真实 UC OAuth 登录、tool call 与 refresh。任一失败就停止切换。全部验收通过后统一使用用户级 `inftask-cloud` 的 `https://harnessagent.tokensinfinity.com/w/default/mcp`，清空仓库 `.mcp.json` 的重复条目，在 `.codex/config.toml` 禁用旧 `inftask`，并更新 `AGENTS.md` 路由，再切 go-backend 的配置与开票流程（在其仓库另行处理）。确认所有写入者都指向公网后，本机 launchd 保持卸载，保留 plist、原库及备份。

**回滚：** 先冻结写入、停公网 app 并停用新 webhook，保留公网数据库的一致性备份和排障文件。公网开始接受新写入后，不能直接用旧本机快照覆盖这些写入；先由维护者决定如何保全或接受丢失新增数据。需要恢复切换前基线时，在本机服务停止的条件下保留原库/WAL/SHM，移走目标旧 WAL/SHM，用切换前一致性备份恢复本机数据库，核对二进制 schema 兼容性和文件属主，恢复 agent 的本机地址，再 `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.dengnaichen.inftask.plist`。核对健康和票据后恢复写入；公网保持停止，避免双写。

### 不改现有入口的 SSH 隧道试运行

现有 80/443 有生产服务时，不能直接运行上面的 Caddy Compose。IP 证书的 ACME HTTP-01/TLS-ALPN-01 验证仍要求 80/443，换成 8443 不能绕过验证；本地试运行可以使用 SSH 隧道，不开放新的公网入口。

[compose.ssh.yaml](../deploy/compose.ssh.yaml) 只发布 `127.0.0.1:18787`，限制 CPU/内存、移除 Linux capabilities 并使用只读根文件系统。它使用独立空数据库，不自动复制维护者数据，也不启用 GitHub 同步或 webhook。UC callback 是 `http://127.0.0.1:18787/auth/callback`，需要 UC 对复用的 client 放行该回调。

在独立目录（例如 `/opt/inftask-trial`）准备：权限 `0600` 的 `runtime.env`（UC base URL/client ID/client secret/可选 issuer）、`infone/infone.pub` 公钥、归 UID 10001 所有的 `data/`，以及该 Compose 文件。secret 不写进命令行、镜像或日志。镜像可用标准 Dockerfile 构建；也可先构建静态 Linux 二进制，再用 [Dockerfile.binary](../deploy/Dockerfile.binary) 在仅包含二进制和系统可信 CA bundle 的目录构建，避免把 runtime.env 放入构建上下文。

```sh
# 本地隔离工作树，先构建前端，再编译服务器架构对应的 Linux 二进制。
npm --prefix web ci
npm --prefix web run build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/inftask-linux ./cmd/inftask

# 服务器独立目录内，镜像标签对应已检查的构建。
INFTASK_IMAGE=inftask-trial:<commit> docker compose -p inftask-trial -f compose.ssh.yaml up -d

# 客户端保持此连接运行；SSH Host 别名必须指向实际部署机器。
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:18787:127.0.0.1:18787 <ssh-host>
```

浏览器打开 `http://127.0.0.1:18787`。本机 HTTP 只走 loopback，跨主机通信由 SSH 加密；这不是公网 HTTPS，也不能接收 GitHub 的公网 webhook。其他团队成员需要各自建立 SSH 隧道才能访问。先检查本地和远端 18787 是否空闲，且不要把 Compose 的绑定地址改为 `0.0.0.0`。

健康检查应返回 200，未登录 `/api/workspaces` 和 `/w/default/mcp` 应返回 401。真实登录需要完成 UC 授权，不能把授权页可打开当作登录成功。停止此实例用同一目录、同一 image 变量执行 `docker compose -p inftask-trial -f compose.ssh.yaml stop`，保留 data 以便继续试用；关闭 SSH 连接只停止本机转发。部署前后核对生产入口进程和配置摘要，不重载生产代理。
