# 验证与结果解释

## Project 时间轴与 Initiative

```sh
go test ./internal/app -run 'TestProjectTimeline|TestInitiative' -count=1
go test ./internal/httpapi ./internal/mcpserver -run TestInitiative -count=1
npm --prefix web run test -- ProjectTimeline.test.tsx InitiativesPage.test.tsx
```

AL-78 的 Red：先添加 TestProjectTimelineDates，在旧实现以 JSON 提交日期并读取，实际得到 dates not persisted 的 FAIL。业务测试覆盖日期清空/省略/非法日期/逆序、Project 列表一致性、Initiative 生命周期/多对多/归档恢复、关联替换事务回滚、跨 Workspace SQL 插入/更新限制、私有项目过滤与隐藏关联保留、非成员拒绝、新旧库结构和数据保留。HTTP/MCP 测试覆盖输入输出和错误；组件测试覆盖完整/单端/无日期、尺度及窗口定位、保存失败草稿与关联移除、恢复归档和离开后响应隔离。jsdom 的 dialog API 使用测试替身，不证明真实浏览器视觉布局。

这些自动化检查不覆盖桌面/窄屏视觉布局，不能用通过结果代替截图验收；需要视觉验收时使用隔离临时数据库。

## 聚合入口

在工作树根目录先按 [开发](DEVELOPMENT.md) 安装前端依赖。推送前执行既有 git-workflow 要求：

```sh
go vet ./...
make test
```

`go vet ./...` 不需要先构建前端，原因见 [开发](DEVELOPMENT.md)。`make test` 先构建前端，再运行仓库接票 hook 合同测试、Hooks lint、前端单次组件测试、`npm --prefix desktop test` 和 `go test ./...`。前端 build 只证明 TypeScript 严格类型检查与 Vite 打包；测试独立执行，本地成功不等于已配置自动 Hook。

GitHub Actions（`.github/workflows/ci.yml`）在目标为 `main` 的 pull request 和 push 到 `main` 时，于 `ubuntu-latest` 上安装前端依赖并运行 `go vet ./...` 与 `make test`，结果与本地同一组命令。它运行 Vitest + React Testing Library 的 jsdom 行为测试，不覆盖真实浏览器布局及 UC、GitHub 集成；是否要求 CI 通过才能合并由分支保护决定，仓库目前未设置。

`git diff --name-only origin/main...HEAD` 列出的文件全部是 `.md` 时，推送前不需要 `go vet ./...` 和 `make test`，改做文档检查：相对链接和路径都存在，文中命令与 Makefile、package manifest 和源码一致。改了 `.agents/skills/` 下的 skill 时，再做 [skill 校验](#skill-校验)。

## 按改动选择证据

| 改动 | 定向检查 | 证明范围 |
| --- | --- | --- |
| Service、输入、关系 | `go test ./internal/app -run TestIssueLifecycle` | 真实临时 SQLite 上的生命周期与业务规则 |
| schema / 迁移 | `go test ./internal/app -run TestMigrate` | 新数据库跳过历史迁移、重建引用表保留数据、外键违规回滚、迁移 1 按前缀回填 team 和编号、迁移 2 把引用改为 id 后每张票输出不变、迁移 3 把现有数据放进 workspace `default` 且每张票输出不变、迁移 4 增加 backlog 且全部表数据与引用不变、迁移 6 给评论加空作者、本地评论作者为空，且新库与升级库的列、外键、索引和触发器一致 |
| 项目/团队 emoji | `go test ./internal/app -run 'TestEntityEmojiLifecycle\|TestMigrateEntityEmoji'`，`go test ./internal/httpapi ./internal/mcpserver -run TestEntityEmoji` | Unicode 序列校验、保存/清空/省略、workspace 隔离、编号保留、迁移备份和新旧 schema 一致、REST 跨站及 MCP 工具契约 |
| 标签、互斥组、筛选与已有库安装 | `go test ./internal/app -run 'TestLabels\|TestLabelFilters\|TestLabelUpgrade'` | 真实 SQLite 上的互斥、幂等、批量回滚、归档、合并、筛选与旧数据保留 |
| 备份 | `go test ./internal/app -run TestBackup` | 每日副本与保留行为 |
| Linear 导入 | `go test ./internal/app -run TestImportLinear` | 手写虚构快照（`internal/app/testdata/linear/`）上的全部映射：字段原样、状态、priority、type、Project 新建/匹配/归档、parent、blocks/related、评论顺序与作者、非 GitHub 附件转链接、`created` 事件、跳过回收站；team 不存在、编号不小于 `next_number`、编号已占用及映射外的输入都中止且不写入。不证明真实快照的数量，那要在 `.backup` 副本上试导入核对 |
| 导入命令 | `go test ./cmd/inftask -run TestImportLinear` | `inftask import-linear` 的参数校验、退出码和输出，重复导入非 0 退出并列出已占用编号，导入后 REST 和 MCP `get_issue` 的评论带 `author` |
| REST 读写、跨站边界 | `go test ./internal/httpapi` | httptest 请求、临时数据库、跨站 POST 被拒绝、workspace 的新建/列出/改名与 slug 校验 |
| MCP 工具契约 | `go test ./internal/mcpserver` | MCP 客户端经 HTTP 执行工作流和非法输入，两个 workspace 经各自 `/w/<slug>/mcp` 互不可见 |
| UC 云端认证 | `go test ./internal/ucauth ./cmd/inftask` | 假 UC 上的 state/cookie/PKCE/token 边界及监听配置，不证明真实 UC 配置或公网部署 |
| GitHub 仓库参数 | `go test ./cmd/inftask -run TestGitHubRepoFlag -v` | 单绑定、重复 flag、逗号列表和混合传法；空列表项与缺失 workspace 报错，失败列表不部分追加。不证明真实 GitHub 权限或 Compose 部署 |
| GitHub 同步 | `go test ./internal/app -run TestGitHubSync` | 受控 HTTP 服务上的关联、推进、游标、仓库失败隔离、每轮重读所绑定 workspace 的 team key、不匹配其他 workspace 的同名 key 以及 webhook 用的 Notify 唤醒，不证明真实凭据权限 |
| UI、导航、共享外观 | `npm --prefix web run build`，另加 [UI 检查](#ui-检查) | 编译/打包与实际操作分别报告 |

新增测试围绕真实可发生的输入与边界；数据库不变量需要真实隔离 SQLite，不能由 mock 成功推出迁移正确。文档改动检查相对链接、路径、命令与源码的一致性，不为可逆低影响文字编辑添加实现镜像测试。

## macOS 云端客户端

先 `npm --prefix desktop ci`。桌面检查入口与证明范围见 [desktop/README](../desktop/README.md#验证)：

- `npm --prefix desktop test`：导航、子窗口、权限、会话配置和失败恢复的单元合同，不启动真实 UC。
- `npm --prefix desktop run smoke -- <仓库外截图目录>`：macOS 上实际 Electron + 临时 HTTPS fixture，覆盖虚构登录、cookie 跨进程保留、外链/重定向、Dock 重开、自签证书拒绝和服务失败后的重试。
- `npm --prefix desktop run dist:mac` 与 `codesign --verify --deep --strict desktop/dist/mac-arm64/InfTask.app`：arm64 DMG 构建与 ad-hoc 签名完整性，不证明 Apple 公证或 Gatekeeper 认可。
- `npm --prefix desktop run smoke:packaged -- <仓库外截图目录>`：实际打包 App 的正式云端只读登录页检查；无真实凭据，不提交账号，不证明登录后的权限、续期或退出。
- `npm --prefix desktop audit`：锁文件依赖通告检查，不代替运行验证。

CI 的 Linux 聚合检查包含桌面单元测试；独立 macOS job 执行单元测试、隔离 fixture smoke、arm64 DMG 和签名检查，不连接正式实例。真实 UC 登录进入工作区、重启保留、Sign out 和会话过期后重新登录，以及非开发机器的下载/Gatekeeper 验收，仍需用户交互并分别记录。所有自动测试 profile 均为临时目录；不读取维护者的浏览器会话或已停用本机库。

## 前端定向检查

- `npm --prefix web run typecheck`：TypeScript 严格类型检查。
- `npm --prefix web run lint`：ESLint 的 `rules-of-hooks`、`exhaustive-deps`，零 warning；使用 Babel 的 TypeScript parser 保留现有 TypeScript 7，不引入第二套 TypeScript 编译器。暂未启用 React Compiler 诊断或全量风格规则。
- `npm --prefix web run test`：Vitest 单次运行；`npm --prefix web run test:watch` 是本地 watch 入口。复用 Vite plugin，环境为 jsdom，测试用虚构 fetch 响应，不连接数据库或正式实例。
- `npm --prefix web run test -- IssueDetail.test.tsx`：评论后续草稿、失败保留、旧读取覆盖、跨字段写入、focus 延后刷新和离开页面取消未发出的排队写入。
- `npm --prefix web run test -- useIssues.test.tsx App.test.tsx`：列表查询隔离、失败回滚、批量标签顺序、设置目录边界、搜索不重读目录、导航保留外壳与重置页面状态。
- `npm --prefix web run build`：检查输出中独立的 `IssuePage`、`IssuesPage`、`ProjectsPage`、`LabelsPage`、`SettingsPage` chunk。bundle 大小不是浏览器耗时证据。

组件测试不证明虚拟列表几何、原生 dialog、拖动或菜单焦点；仍需下方浏览器验收。

## Agent 接票

```sh
go test ./internal/app -run 'TestIssueDelegation|TestDelegationUpgrade' -count=1
go test ./internal/mcpserver ./internal/httpapi -run TestIssueDelegation -count=1
node scripts/test-delegation-hook.mjs
npm --prefix web run test -- IssueDetail.test.tsx
```

覆盖显式接票自动进入 `in_progress`、主/子身份和并发去重、可信 actor、私有团队/workspace/停用、非法输入、存储失败、新旧库结构与升级备份、MCP 严格输入和 REST 详情。`TestIssueDelegationStartsProgress` 覆盖各初始状态、重复身份重新开始、已进行中重复/子 Agent 不刷新时间或增加事件；`TestIssueDelegationProgressRollback` 注入来源、状态和事件写入失败，验证事务回滚。`TestIssueDelegationRequiresAssignee` 覆盖未分配主/子拒绝、分配后成功、清空后重复/新调用拒绝、历史保留和重新分配后的原记录去重；MCP 生命周期验证清晰的负责人前置错误。hook 合同执行实际仓库 command（含稀疏 PATH、空格路径），只用虚构主/子事件，不联网；不能把它当成真实宿主触发证据。真实两端 smoke 使用隔离客户端配置、临时库和 loopback MCP，记录实际 `PreToolUse` session/agent ID，不复制维护者凭据或向正式实例造数。先用自己的临时服务通过本地 `bootstrap-access` 初始化测试成员，创建一张测试票并指定该成员为 assignee，再运行 `node scripts/probe-delegation-hosts.mjs http://127.0.0.1:<端口> <测试票号> <仓库外输出目录>`；需要 PATH 上的 Claude Code/Codex。脚本使用本地模拟模型 API 触发真实主/子 Agent 工具与 hook，不调用真实模型；只在临时 Codex 进程中绕过审核来测试链路，不修改用户信任或审批配置。各工具的日常客户端审批仍由宿主决定。

在同一临时票上先准备主/子和长 ID 记录，再运行 `node scripts/browser/delegation.mjs http://127.0.0.1:<端口> <测试票号> <仓库外截图目录>`，检查真实详情的属性值文字与点击区域在各自属性格内左对齐（相差不超过 1px）、agent 行位于 Assignee 下且标志与负责人名字对齐、点击 agent 行滚动、主/子记录、长 ID 选择和 390px 无溢出；截图与点击结果分别报告。测试票的正文要足够长，详情能纵向滚动，否则滚动检查会超时；造数后的 AL-2 满足。分发插件本票不改，不能宣称插件已获得 hooks。

## Skill 校验

改了 `.agents/skills/<name>/SKILL.md` 时，在工作树根目录对每个改动的 skill 运行：

```sh
uv run --offline --python 3.11 --isolated --with pyyaml==6.0.3 python ~/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/<name>
```

exit 0 并打印 `Skill is valid!` 表示通过。脚本只检查 frontmatter 和正文里未完成的 TODO 占位，不检查链接，也不检查 name 与目录名是否一致。

前提：本机装有 Codex 自带的 skill-creator（上面的脚本路径存在）和 uv。`--offline` 不联网下载，所以 `uv python find 3.11` 要能找到已安装的 Python 3.11，uv 缓存里要已有 pyyaml 6.0.3；缓存里没有时命令报依赖无法满足并 exit 1。没有这些前提时，手动检查 frontmatter 的 `name` 与目录名一致、`description` 非空，并在报告里写明没有运行脚本。

## UI 检查

改动 UI 时用临时开发数据库，检查列表/看板切换、筛选下新建、状态选择和拖动、详情直达/刷新、编辑及评论失败后文本保留。共享样式还需核对 1024px、760px 两侧和窄屏，键盘焦点与原生 select/dialog。具体行为见 [交互](INTERACTION_DESIGN.md)。这些操作靠人工检查；下面的脚本只负责造数和截图。

`scripts/browser/` 只用 Node 内置的 `fetch` 和 `WebSocket`，需要 Node 22 及以上，截图默认用 `/Applications/Google Chrome.app` 里的 Chrome，可用环境变量 `CHROME_PATH` 指向别的 Chrome 可执行文件；路径不存在或启动失败时脚本 exit 1，报错写出该路径。不要用 Chrome 命令行的 `--headless --screenshot`：在维护者的 Mac 上它写出 PNG 后进程不退出，会一直卡住。截图脚本改走 DevTools 协议。

1. 按 [开发](DEVELOPMENT.md) 构建前端，检查端口后用临时库启动开发服务。没构建前端时页面返回 404。
2. 造数：

   ```sh
   node scripts/browser/seed.mjs --url http://127.0.0.1:<端口>
   ```

   脚本先经 `POST /api/workspaces` 建 workspace `default`，再经 `/w/default/mcp` 建 team `AL`，写入两个 Project 和 11 张 AL ticket，覆盖除 `backlog` 外的状态和全部 type、带两个子票的 spec、blocks / blocked_by / related、一个链接，以及含表格和代码块的正文和评论，逐行打印标识符、状态、type 和标题。服务里已有任何 workspace 时拒绝写入并 exit 1。AL-2 就是这张 spec，详情页的各种元素都在它上面；页面在 `/default/`，`/` 会跳到这里。
3. 在仓库外写截图列表，每项给出 `url`、`width`、`height` 和输出 `path`，再运行截图脚本：

   ```sh
   shots=$(mktemp -d)
   cat > "$shots/shots.json" <<EOF
   [
     {"url": "http://127.0.0.1:<端口>/default/", "width": 1280, "height": 900, "path": "$shots/list-1280.png"},
     {"url": "http://127.0.0.1:<端口>/default/", "width": 390, "height": 844, "path": "$shots/list-390.png"},
     {"url": "http://127.0.0.1:<端口>/default/issue/AL-2", "width": 1280, "height": 900, "path": "$shots/issue-1280.png"},
     {"url": "http://127.0.0.1:<端口>/default/issue/AL-2", "width": 390, "height": 844, "path": "$shots/issue-390.png"}
   ]
   EOF
   node scripts/browser/screenshot.mjs "$shots/shots.json"
   ```

   每项还可加两个可选字段：`click` 是 CSS 选择器，页面网络空闲后点击该元素，等两帧再截图，匹配不到元素时 exit 1；`scale` 是设备像素比，默认 1。看板没有自己的 URL，需在浏览器先打开 Display，再选择 Board；这个单次点击脚本不覆盖两步切换。14px 的状态图标用 `scale` 放大：

   ```json
   {"url": "http://127.0.0.1:<端口>/default/", "width": 800, "height": 600, "path": "<目录>/zoom.png", "scale": 4}
   ```

   `scale` 为 4 时 800x600 的视口输出 3200x2400 的 PNG。等待 DevTools 启动和每一项的 `networkIdle` 各最多 30 秒，超时 exit 1，报错写明在等什么；页面一直有请求不结束时就会超时。

   脚本用临时 profile 启动 headless Chrome，DevTools 端口由 Chrome 自选。每项按给定宽高加载页面，等网络空闲后截取视口并打印输出路径；输出目录必须已存在。结束或出错时关闭 Chrome、删除 profile。多个 agent 各用自己的服务端口，可以同时运行。
4. 逐张打开截图核对内容。只截视口，要看下方内容就加大 `height`。截图只证明该宽度下的渲染结果，不证明点击、拖动、键盘和失败恢复。

## 报告

记录实际命令、退出码、失败或跳过原因。定向测试通过只证明其执行范围；需要推送时仍完成全量要求。没有运行的浏览器检查明确写未运行，不能用构建输出代替。所有开发写入与测试均须隔离维护者数据。

## 分组展开与收起的滚动定位

前提：Node 22+、Chrome（可用 `CHROME_PATH` 指定），执行 `make build` 后按开发文档启动新的隔离临时数据库服务，端口不能为 8700。

```sh
node scripts/browser/group-scroll.mjs http://127.0.0.1:<端口> <临时截图目录>
```

脚本只向空服务写入专用 workspace 和 1,273 张虚构任务（其中 Done 1,000 张）；已有服务必须匹配专用 fixture 名称及各状态计数才能只读复跑，否则拒绝。不要对正式实例运行。脚本检查 1280px/390px 下普通与虚拟分组的吸顶收起、再展开、连续切换、鼠标与 Enter/Space、滚轮释放定位、第一帧与稳定后的标题位置，以及虚拟行挂载量和滚动后内容可见性。可用滚动空间充足时位移不超过 1px；最后分组收起受底部边界限制时标题完整可见，无额外占位。截图保存在指定目录。AL-74 的 Red 在旧实现上收起吸顶 Todo 时从 105px 跳至 -245px，脚本 exit 1；修复后应 exit 0。此检查不覆盖 Safari/Firefox 或生产部署。

## OAuth 自动刷新与网页登录

`go test -race ./internal/ucauth ./cmd/inftask` 覆盖 `TestMCPRefreshTemporaryFailure`、`TestMCPReportsRemainingSignedLifetime`、`TestExpiredBearerChallenge`，以及 `TestWebSessionSurvivesRestart`、`TestWebSessionRefreshesAfterAccessExpiry`、`TestWebSessionRotationAndFailure`、`TestWebSessionStoreEncryptedAndBound`、`TestWebSessionLogoutDuringRefresh`。测试证明受控时钟下续期、轮换、临时失败恢复、撤销和重启保留；不能替代真实 UC/宿主验收。

真实验收分别在 Claude Code、Codex 登录一次，跨越两次 access token 到期后调用 `list_teams`，记录自动刷新成功和无人工登录的证据。浏览器登录后跨越 access 到期访问页面及 REST，应用重启后再次访问仍已登录；退出后重启也不得恢复该会话。不要缩短共享生产 UC token 生命周期来加速验收，不记录 token、cookie 或上游敏感响应。

`python3 scripts/auth/probe_codex_refresh.py` 需要 PATH 上的 Codex，使用临时配置与虚构 token，先模拟 503，再跨过短 token 有效期，验证凭据保留、连续轮换和进程重启后的 `list_teams` 工具发现。每次 RPC 最多等待 20 秒，中间等待 36 秒；不运行模型，不使用真实 UC 凭据。此脚本验证宿主行为，不替代 InfTask 服务端测试或真实 UC 验收。

## 千条任务列表性能

使用隔离临时数据库，准备 1,000 条 Done 和其它状态各一条虚构任务。比较相同视口、浏览器和生产构建的改动前后版本；不要用 Vite 开发模式或浏览器工具调用总耗时代表用户交互耗时。

- 用浏览器 Performance 录制展开 Done、搜索输入和单条勾选，比较主线程任务及绘制；统计 `.list-row` 挂载量。1280×900 下展开后的总数应保持 1,007，挂载行应低于 100。
- 通过 React Profiler 或仅用于验证的行渲染计数，检查搜索草稿输入不渲染任务行，单条勾选只更新对应行；滚动导致新行挂载另计。临时探针不提交到产品代码。
- 滚动至组首尾，检查吸顶边界、列宽稳定性、390px 与 1024px 两侧行高无重叠；焦点跨虚拟窗口时 Tab/Shift+Tab 连续，菜单关闭恢复触发器焦点。
- 检查鼠标、Enter/Space、方向键、字母查找和 Escape，覆盖列表、看板及原生创建/编辑 dialog。关闭的菜单不挂载选项树。
- 检查搜索提交/清除、排序、状态筛选及批选全部查询结果；停止自己的临时服务，核对优先级失败保留原值和状态失败回滚，再重启验证恢复。

虚拟列表的未挂载行不参与浏览器页内查找；全范围检索使用应用 Search。以上是浏览器验收流程，不由 `make test` 自动覆盖。
## 工作区成员权限

AL-62 的 Red 是新增 `TestWorkspaceAuthorizationHTTP` 后先运行旧实现，两个未加入工作区的 UC 用户仍能列出和读取已有工作区，必须观察到 `--- FAIL`；在未包含该测试的旧提交直接 `-run` 会得到无测试的假绿。

```sh
go test ./internal/app -run 'TestWorkspaceAccess|TestWorkspaceMembership|TestWorkspaceInvite|TestWorkspaceAccessUpgrade' -count=1
go test ./internal/ucauth ./internal/httpapi ./internal/mcpserver -run 'TestWorkspaceAuthorization|TestIdentity' -count=1
go test ./cmd/inftask -run TestBootstrapAccess -count=1
```

覆盖 Service 操作的即时成员检查、并发降级保留最后一个 Admin、邀请轮换/重复接受/停用、REST/MCP 同一身份与撤权、Cookie 传递身份、缺中间件身份拒绝、旧库数据/备份/新旧结构一致与显式初始化。真实 UC 和生产部署不在这些测试范围内。

浏览器验收需要先 `npm --prefix web run build`、Node 22+ 与 Chrome，使用前述 `CHROME_PATH` 可选路径。以下两个终端使用同一临时目录：

```sh
# 终端一：先用 mktemp -d 创建目录，将路径作为 <临时目录>；服务随机选择 loopback 端口。
INFTASK_ACCESS_BROWSER=<临时目录>/fixture.json go test ./internal/ucauth -run TestWorkspaceAuthorizationBrowser -count=1 -timeout=11m
# 终端二：等 fixture.json 出现再执行。
node scripts/browser/access.mjs <临时目录>/fixture.json <临时目录>/screenshots
```

脚本在真实嵌入页面、临时数据库、伪 UC 的两个真实登录 Cookie 上操作：最后 Admin 错误、重置链接、无效/有效邀请接受、Member 隐藏管理按钮、停用/恢复和提升角色；检查 390px 无横向溢出，保存桌面错误态、Member、窄屏和深色截图。脚本结束写 `.done` 让 fixture 退出，fixture 最长等待十分钟。重复运行使用新的临时目录；fixture JSON 含临时凭据，不提交、不用于正式实例。

## 私有团队权限

`TestPrivateTeamAccess` 先在原实现上添加，以 JSON 输入 private=true 创建团队；原实现忽略此字段，未加入用户可读列表/Get/Details，观察到真实 FAIL 后才实现过滤。

```sh
go test ./internal/app -run 'TestPrivateTeamAccess|TestTeamMembership|TestPrivateProjectVisibility|TestTeamAccessUpgrade|TestPrivateTeamGitHubLinkback' -count=1
go test ./internal/ucauth -run TestPrivateTeamAuthorization -count=1
```

后者使用真实伪 UC bearer 与 MCP SDK 会话，同时验证 REST 与 MCP 在管理员加入前、确认加入后和退出后的结果。其余测试覆盖可见性矩阵、Project 隐藏关联保留、父子关系、limit 前过滤、不可见写入/批量回滚、旧数据与成员升级保留、新旧 schema 一致，以及 GitHub 不外发私有标题/说明。

使用新的临时目录，先构建页面，在两个终端启动/验证：

```sh
INFTASK_TEAM_BROWSER=1 INFTASK_ACCESS_BROWSER=<临时目录>/fixture.json go test ./internal/ucauth -run TestWorkspaceAuthorizationBrowser -count=1 -timeout=11m
node scripts/browser/access.mjs <临时目录>/fixture.json <截图目录> teams
```

三个伪 UC Cookie 分别代表 Admin、私有团队 owner 和普通 Member；覆盖公开加入/退出、私有直达拒绝、owner 添加成员、成员退出、管理员确认加入、转公开，以及 390px 截图。环境与凭据清理要求同上，不验证真实 UC 或生产部署。

## 人的任务负责人

AL-64 的 Red 是先在未改实现上添加 `TestIssueAssigneeLifecycle` 并观察创建缺少 assignee 的真实 `--- FAIL`。以下检查分别覆盖生命周期、可信操作者、重复值不增审计、失败原子回滚、跨 workspace/私有团队/停用限制、新旧 schema 和升级备份，以及 REST/MCP 输入伪造拒绝：

```sh
go test ./internal/app -run 'TestIssueAssigneeLifecycle|TestIssueAssigneeAuthorization|TestAssigneeUpgrade' -count=1
go test ./internal/httpapi ./internal/mcpserver -run TestIssueAssignee -count=1
```

浏览器沿用前述临时库与伪 UC fixture，先构建前端；以下两个终端使用同一新建临时目录：

```sh
INFTASK_ACCESS_BROWSER=<临时目录>/fixture.json INFTASK_TEAM_BROWSER=1 go test ./internal/ucauth -run TestWorkspaceAuthorizationBrowser -count=1 -timeout=11m
node scripts/browser/access.mjs <临时目录>/fixture.json <临时目录>/screenshots assignees
```

验证创建指定、另一成员改派/清空、可信历史、断网失败保留原值、自己/未分配/指定成员过滤、列表/看板展示，保存桌面与 390px 截图并检查窄屏溢出。不是生产 UC 或正式部署验收。

## Agent 工作记录

```sh
go test ./internal/app -run 'TestWorkLog|TestWorkLogsUpgrade' -count=1
go test ./internal/httpapi ./internal/mcpserver -run TestIssueWorkLogs -count=1
npm --prefix web run test -- IssueDetail.test.tsx
```

覆盖评论分离、可信 actor、progress/decision、摘要和正文边界、倒序 cursor 分页（页间新插入）、工作区/私有团队/停用、失败写入、新旧 schema/升级前备份/历史评论作者保留、REST/MCP 严格输入和按票读记录。HTTP Red 在未改实现上请求工作记录入口为 404，新增测试实际失败后才实现。组件检查默认评论、工作记录按需加载、决策正文读取、失败重试、旧页追加、离开页签忽略旧响应和评论草稿保留。

真实浏览器验证使用隔离空库，运行既有 seed 后，经本地 MCP 在 AL-2 写入 decision 和超过 20 条 progress，再运行 `node scripts/browser/work-logs.mjs http://127.0.0.1:<端口> AL-2 <仓库外截图目录>`。脚本检查桌面/1024/760/390px、明暗主题、键盘页签、决策展开、分页、断网读取重试以及评论草稿跨切换/失败保留和成功提交；只写隔离库，不用于正式实例。Chrome、Node 和截图环境同上。

## 依赖 DAG

```sh
go test ./internal/app -run 'TestRelationDAG|TestImportLinear' -count=1
go test ./internal/mcpserver -run TestRelationDAG -count=1
```

AL-75 的 Red：先新增 `TestRelationDAG` 和 `TestRelationDAGSQL`，在未改实现上执行，Service 的 blocks/blocked_by 和 SQL INSERT/UPDATE/批量写入均观察到真实 `--- FAIL`。测试覆盖服务层独立校验、两点与长环、自引用、合法菱形、重复/删除、related 排除、私有中间路径不外泄、直接 SQL UPDATE 和冲突写入策略、旧库升级/备份/结构一致、旧环失败回滚，以及 Linear 导入整体回滚和 MCP 错误。全部使用隔离临时库；不验证生产旧数据是否含环，不改变 parent 的既有语义。

## Cycle 周期

```sh
go test ./internal/app ./internal/httpapi ./internal/mcpserver -run TestCycle -count=1
npm --prefix web run test -- Cycles.test.tsx IssueDetail.test.tsx
```

覆盖周期参数、同 Team/工作区/私有权限、归属及清空筛选、DST 日历周、连续补算、重复执行、结算快照、失败事务回滚、旧库备份和新旧结构一致，以及 REST/MCP 严格输入。Red 是先添加 TestCycleLifecycle，在旧实现上观察未知 cycle_id 被静默忽略导致真实 --- FAIL。

浏览器需要构建最新二进制、Chrome、Node 22+，在空临时数据库及空闲 loopback 端口启动服务，先运行既有 seed，再执行：

```sh
node scripts/browser/seed.mjs --url http://127.0.0.1:<端口>
node scripts/browser/cycles.mjs http://127.0.0.1:<端口> <仓库外截图目录>
```

脚本向该临时 fixture 启用周期并创建任务，检查配置、新建预选、详情加入/清空/刷新、Team 筛选、断网保存保留、Escape 和 1280/1024/760/390px 明暗主题。每次完整重跑使用新的空库；不用于正式实例。页面截图不证明自动时间推进；时间推进由受控时钟 Service 测试覆盖。
