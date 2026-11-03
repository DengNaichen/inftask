# 信任边界与数据外发

## 本地访问边界

`cmd/inftask/main.go` 默认监听 `127.0.0.1:8787`。`internal/httpapi/server.go` 用 `http.NewCrossOriginProtection` 包裹 REST、MCP 和静态页面；跨站浏览器 POST 被拒绝、无对应浏览器头的 MCP 请求可通过。跨站保护独立于用户认证及成员授权。未配置 UC 时只允许 loopback 监听，保留本地业务操作但没有云端成员管理；配置 UC 后启用网页会话及 REST/MCP token 认证，并按工作区活动成员限制业务访问。

HTTP 组装层没有独立的本地 Host allowlist，不能从默认 loopback 监听推断所有非本地 Host 都被拒绝。修改监听范围或代理部署会改变信任边界，需要另行确定任务范围；云端通过 HTTPS 反向代理发布，配置与边界见 [开发](../DEVELOPMENT.md#云端团队部署)。

## macOS Electron 客户端

`desktop/` 只连接固定正式云端，不启动本地服务、不接管旧 SQLite。InfTask 与核实的 UC HTTPS origin 是唯一可在窗口/frame 内导航的来源；未知重定向被拒绝，其他 HTTP(S) 链接在系统浏览器打开，非 HTTP(S) 和带用户凭据的 URL 不交给操作系统。

远程页面没有 Node integration、preload 或 IPC 本机桥，开启 context isolation、sandbox 和 webSecurity。登录子窗口沿用同一受限 session；页面/设备权限、webview 和下载均拒绝。默认 Chromium 证书校验保持启用，不修改云端认证或 Origin 检查。Chromium 本机 profile 含 cookies 等登录状态，不是业务数据库，也不能上传排障。自动测试只在未打包的入口按 SHA-256 指纹信任自己的临时 HTTPS fixture；真实云端检查不忽略证书错误。

默认 ad-hoc 签名仅支持本地试用，不代表 Developer ID 或 Apple 公证。构建、独立持久化 session、实际验证和正式分发要求见 [桌面客户端](../../desktop/README.md)。

## 输入与输出

HTTP JSON 请求限制为 1 MiB，拒绝未知字段和多余 JSON 值；业务规则仍由 Service 统一校验。SQL 值使用绑定参数，动态条件由源码构造。修改筛选时核对值是否仍走参数绑定，不能拼接请求值进 SQL。

HTTP 将未分类内部错误返回为通用 `internal server error`；MCP 使用 SDK 的错误通道，不能假设两种协议错误形式相同。不要在新增诊断中打印 token 或完整敏感正文。

详情的 Markdown 使用 `react-markdown` 和 `remark-gfm`，当前没有 raw HTML 插件。修改渲染链时核对不可信正文、链接和图片的输出行为，不为展示方便直接注入 HTML。

## GitHub

配置 `-github-repo` 才启用同步，必须提供 `GITHUB_TOKEN`；凭据来自环境变量，不进入前端、源码、fixture 或日志。测试沿用受控 HTTP 服务，不用真实 token。

`-github-linkback` 默认关闭；启用后会把 issue 标题和描述发送到 open/draft PR 的评论区，读者范围取决于目标 GitHub 仓库。添加本地 link 与向 GitHub 发评论是不同副作用，前者不隐含授权后者。远端成功但本地记录失败的后果见 [可靠性](RELIABILITY.md)。

修改入口后至少运行 [HTTP/MCP 验证](../VALIDATION.md)，报告实际覆盖，不能宣称已有测试证明完整认证或 Host 校验。


## UC 云端认证

`internal/ucauth` 参考 HarnessKit 的 confidential OAuth 接入方式。验证过的 subject/name/email 经请求上下文进入 Service；客户端正文不能选择操作者。成员表决定工作区访问权，只有 active 成员能读写业务数据，Admin 管理角色、停用/恢复、邀请及改名。所有公开团队业务向活动 Member 开放；私有团队另按显式团队成员限制；当前没有 Guest 或 Agent 独立授权。临时共用 HarnessKit client 时，其符合条件的 UC access token 可用于认证，但仍须通过 InfTask 成员检查。

HTTP/MCP 使用 `ForRequests(true)` 的 Service，缺少可信身份时拒绝；后台 GitHub 同步保留原始内部 Service，不将外部请求的缺身份解释为系统权限。每次业务方法检查成员状态；停用使之后的请求失效，不取消已经通过检查的在途操作。最后一个活动 Admin 的保护在成员更新事务中实施。已有工作区只允许本地 `bootstrap-access` 显式初始化，不能由任意已登录用户认领。

邀请是 32 字节随机 token 的可复用链接，只存 SHA-256 摘要；管理员重置使旧链接失效。受邀者只成为 Member，重复接受不降级现有 Admin，suspended 成员不能通过邀请恢复。`/auth/join?token=...` 要求登录和主动接受，业务页面使用 no-referrer，响应 no-store；分享链接等同分享加入权，不能把 token 写进日志或 ticket。

`INFONE_UC_CLIENT_SECRET` 只在后端换 token 时使用；前端只持有 opaque HttpOnly 会话 cookie。Web callback 必须匹配浏览器 cookie 和未消费、未过期的随机 state。会话最长 30 天，随机 cookie 对应的身份、access 到期时间和 refresh token 在服务端加密文件中持久化；退出持久删除，正常重启保留。access 到期后按需向 UC 续期，成功响应必须验签且 subject 不变；UC 拒绝刷新时撤销会话，暂时故障返回 503。写操作另要求 Origin 精确匹配 `INFTASK_PUBLIC_URL`。会话不放宽现有 Go 跨站保护。

API bearer 与 MCP bearer 都验证配置公钥的 RS256/ES256 签名、非空 sub、配置的 client_id、type=access、exp/nbf，以及可选 issuer。UC 当前不保证 aud，不能把 client_id 检查描述成 OAuth resource audience 隔离。显式 bearer 验证失败不能回退到 cookie；MCP 不接受 cookie。离线 token 校验不实时查询 UC 的撤销状态；InfTask 自己的工作区成员状态在每次业务访问时查询。

每个 workspace 的 `/w/<slug>/mcp` 是一个受保护资源，元数据在 `/.well-known/oauth-protected-resource/w/<slug>/mcp`；该路径只检查 slug 的形状，不查询 workspace 是否存在。MCP OAuth 仅接受固定 public client `inftask`、本站 `/w/<slug>/mcp` 形状的 resource、loopback callback 和 S256 PKCE；授权码只能用授权时的 resource 兑换，grant 仅可消费一次，2 分钟到期。由于 UC token 不带 resource audience，一个 workspace 授权得到的 token 也可用于认证其他 workspace 的请求；是否可访问由 InfTask 成员表决定，resource 本身不构成隔离。登录 flow 和 grant 各最多 256 个，网页 session 最多 4096 个，过期条目在创建时清理。密钥、上游响应正文和 token 不写日志或错误响应。网页会话文件用专用派生密钥和 AES-GCM 加密，权限 0600；MCP token 仍由宿主管理。会话文件损坏或密钥不匹配时启动失败，不回退到匿名或无认证模式。

公开的 `/api/health` 不返回业务数据。公开的 webhook 校验原始 body 的 `X-Hub-Signature-256`，上限 1 MiB，只对已配置仓库的 PR 通知调度同步，同步只写入该仓库绑定的 workspace；不依据 payload 修改业务数据。重复通知最多合并为一个待处理刷新，读取最新 GitHub 状态，轮询补漏；不承诺 delivery 恰好处理一次。webhook 不隐含启用 linkback。

## 私有团队

公开团队允许所有活动 workspace 成员协作。私有团队任务的列表、直达、写入、评论、链接、父子关系、关联任务与 Project 目录统一过滤；管理员必须主动确认加入才能访问内容。Admin 的设置目录发现权限只暴露团队元数据，不返回真实编号进度或私有任务。非成员不可借 ID、名称、parent、Project、批量标签或 limit 绕过。

团队创建者及转为 private 的人自动加入为 owner。Owner 与 Workspace Admin 管理成员及可见性；没有最后 owner 限制。私有成员退出后需被 owner/Admin 再添加，Admin 可再次显式确认加入。工作区停用优先于团队成员身份。

GitHub 内部同步仍可处理私有团队，但启用 linkback 时只发送任务 identifier，不发送私有标题或说明。当前没有公共任务链接配置，因此不拼接外部 issue URL。转私有不撤回已经发送到 GitHub 的旧评论；启用 linkback 的管理员须考虑此前外发的内容。

## Agent 接票来源

`delegate_issue` 复用现有活动成员和任务团队访问，不建立 Agent 独立授权。可信 actor 来自请求上下文，客户端只提供有界的执行来源；输入未知 actor 字段被拒绝。来源不是密码学证明，客户端可以自报，因此不能用 session/agent/hostname 扩大访问权。详情来源信息与任务共用可见性边界。

Service 要求每次委派时任务已有非空人类 assignee，主/子 Agent 和重复调用都遵守；先检查访问边界，再检查负责人，避免泄露不可见任务的归属。此条件不要求调用者等于负责人，不重新校验历史负责人的分配资格，也不自动分配或授予额外权限。清空负责人只阻止后续委派，已有来源记录仍按任务权限可读。

仓库 `PreToolUse` 只匹配已知 InfTask 服务的 `delegate_issue`，保留业务参数并整体覆盖 `client_context`，不读取 token、环境凭据、transcript 或本地会话文件，不自行发请求。缺宿主 session 时拒绝，不输出原始 hook JSON；第三方和其他工具 no-op。明确允许的 hook 决策只针对这一接票操作，服务端仍重新检查权限。记录会外发机器 hostname、session ID 和可选子 Agent ID/profile 到所连接工作区，不能把它们当成无数据外发的纯本地操作。不要在展示或日志中把来源当成身份认证；页面以普通文本展示，不插入 raw HTML。

## 负责人和审计

人的 assignee 不附加独占编辑限制；有任务写权限的活动成员可分配、清空、改派。Service 在写入事务内校验被分配者为本工作区活动成员且有团队访问权。候选目录先验证调用者的团队内容访问权，Workspace Admin 的设置发现权限不能借候选目录读出私有团队成员。任务/审计读取沿用任务可见性边界。审计操作者只来自已认证的请求上下文，REST/MCP 的未知 actor 参数被拒绝；内部无身份操作明确记录为空，不冒充用户。停用保留历史负责人，不允许新分配。

## 工作记录

`add_work_log`、`list_work_logs`、`get_work_log` 与详情决策摘要均先经过工作区活动成员和任务团队访问检查；按记录 id 读正文还必须匹配同一票的内部 id。操作者来自可信请求上下文，未知 actor 输入由 MCP 严格 schema 拒绝。kind=decision/progress 是内容分类，不是 Agent 独立鉴权或人类审批证明。Markdown 复用无 raw HTML 的渲染器；摘要单行最多 240 字符，正文最多 65,536 字节。REST 只提供工作记录读取，网页写入继续使用评论入口。

Spec 的 REST（`/specs`）与 MCP（`list_specs`、`get_spec`、`create_spec`、`update_spec`、`list_spec_revisions`、`get_spec_revision`）先经过工作区活动成员检查，再按所属 team 的可见性过滤；不可读的 spec 和修订按不存在处理，创建与更新不接收客户端指定的作者，作者取可信请求上下文。内容是普通文本字段，不是 Markdown 或 HTML，不执行也不渲染任何 agent 编写的代码；之后的自定义 canvas 在运行 agent 代码前须先补安全响应头和本文件的规则。更新要求 `base_revision`，避免多个调用方静默互相覆盖。
