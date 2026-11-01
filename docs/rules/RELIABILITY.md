# 数据可靠性与恢复

## Schema 的两条路径

真实实例有数据，任何变更都不能要求删除数据库。`internal/app/schema.sql` 始终描述当前形状；新数据库直接使用它，`user_version` 设为迁移数量，不执行历史迁移。增加表使用 `CREATE TABLE IF NOT EXISTS`。

修改已有表时同时更新 schema，并向 `internal/app/migrate.go` 的 `migrations` 追加升级 SQL；已交付迁移不得编辑。当前有 17 项：

- 迁移 1 新建 `teams`，重建 `issues` 并按 identifier 回填 `team` 和 `number`，每个前缀成为一个 team，`next_number` 是该前缀最大编号加一。迁移 1 之后，旧版二进制仍能读库，但建票会因缺 `team` 和 `number` 失败；回滚到旧版要恢复 `before-migration-1.db`，此后的写入会丢失。
- 迁移 2 把表之间的引用从 identifier、Project 名称和 team key 改为整数 id：`teams`、`projects` 改用 `id` 主键（`key`、`name` 仍全实例唯一），`issues` 改存 `team_id`、`project_id`、`parent_id`，`relations` 改存 `issue_id`、`other_id`，`links`、`comments`、`issue_labels` 改存 `issue_id`。issue、链接和评论保留原 id；related 关系按 id 重排为 `issue_id < other_id`。`pull_requests.issue_identifier` 不变。迁移 2 之后，旧版二进制仍能启动，但读写 issue 会因旧列名不存在而失败；回滚到旧版要停服务并恢复 `before-migration-2.db`，此后的写入会丢失。
- 迁移 3 新建 `workspaces(id, slug, name)` 和 workspace `default`（名称 `Default`），重建 `teams`、`projects`、`issues`、`label_groups`、`labels`、`pull_requests`、`github_sync`，各加 `workspace_id` 并全部填为 `default`，所有行保留原 id。唯一约束改为 workspace 内唯一：team key、Project 名称、标签组名、标签名和 issue identifier；`labels` 通过 `(workspace_id, group_id)` 复合外键引用同一 workspace 的组；`pull_requests` 主键和 `github_sync` 游标都带 `workspace_id`。重建 `labels` 前先删掉引用它的 `issue_labels_exclusive` 触发器，结束时重建。迁移 3 之后，旧版二进制仍能启动、读取和修改已有记录，但新建 issue、team、Project 和标签会因 `workspace_id` 缺失或冲突子句不匹配而失败；回滚到旧版要停服务并恢复 `before-migration-3.db`，此后的写入会丢失。

- 迁移 4 重建 `issues`，在 status CHECK 中加入 `backlog`；保留全部字段、内部 id、workspace 和自增序列上限，重建 team/number、status/updated 和 parent 索引。引用 `issues(id)` 的表不重建，迁移事务内检查外键。已有版本 3 数据库升级前生成 `before-migration-4.db`；新库直接使用含 backlog 的 schema。旧版服务不接受 backlog 输入，页面也没有该状态，回滚时要停服务并恢复升级前备份，此后的写入会丢失。

- 迁移 5 用 `ALTER TABLE issues ADD COLUMN` 增加 `priority INTEGER NOT NULL DEFAULT 0 CHECK (priority IN (0, 1, 2, 3, 4))`，已有票回填为 No priority，原引用、正文及时间保留。迁移前备份为 `before-migration-5.db`（从版本 4 升级时）；旧版程序可以忽略此新增列，但不会读取或设置优先级。

- 迁移 6 用 `ALTER TABLE comments ADD COLUMN` 增加 `author TEXT NOT NULL DEFAULT ''`，已有评论的作者为空，其他列和 id 不变。从版本 5 升级前备份为 `before-migration-6.db`。旧版程序能直接打开升级后的库，读写评论和票都正常，只是不返回作者，它新加的评论作者为空。回滚有两种：停服务换回旧版二进制，作者留在库里、页面和接口不显示；或停服务恢复 `before-migration-6.db`，此后的写入（包括从 Linear 导入的数据）会丢失。

- 迁移 7 给 `projects` 和 `teams` 追加 `emoji TEXT NOT NULL DEFAULT ''`，现有图标沿用默认值；名称、描述、归档、内部 id、team key 和 next_number 保留。从版本 6 升级前备份为 `before-migration-7.db`。旧版服务可忽略新增列继续读写；回滚二进制后 emoji 仍在库中，但旧页面不显示。恢复升级前备份会丢失此后的写入。

- 迁移 8 新建 `project_teams` 和 Team 反查索引、workspace 插入检查触发器；按已有任务（含已关闭任务）的 Project/Team 集合回填，不重建原表，不改 id、identifier、引用、编号或时间。历史空项目没有关联，保留待归属。从版本 7 升级前备份为 `before-migration-8.db`。旧版程序不维护此关联，不能用旧版继续业务写入；回滚需停服务并恢复升级前备份，此后的写入会丢失。后续重建 `projects` 或 `teams` 时，应先移除引用它们的 `project_teams_workspace`，重建后恢复触发器；模拟重建测试覆盖该依赖。

- 迁移 9 新建 `users`、`workspace_members` 和 `workspace_invites`，不回填任意人的成员身份，不改业务数据。从版本 8 升级前保存 `before-migration-9.db`；已有工作区按 [开发](../DEVELOPMENT.md#初始化已有工作区的管理员) 显式指定初始 Admin。新库直接创建同样的表。旧二进制不会执行成员检查，不能在云端直接回滚到旧二进制继续服务；恢复升级前备份也会移除这些授权状态，必须停服务并评估访问边界。

- 迁移 10 追加 `teams.private`（默认 false），新建 `team_members` 和 workspace 成员一致性插入触发器。保留工作区成员、原任务、团队编号、Project 和关联；不自动生成团队成员或 owner。版本 9 升级前备份 `before-migration-10.db`，新库/升级库结构一致。旧二进制不实施私有团队过滤，不能回滚旧二进制继续云端服务。

- 迁移 11 追加 `issues.assignee_subject`（可空、引用 `users.subject`），新建 `issue_assignments` 审计表及查询索引。已有任务保持未分配，业务字段、引用和时间不变。版本 10 升级前备份 `before-migration-11.db`，新旧库结构一致。旧二进制忽略新增负责人字段，但不会维护分配审计；需停服务再回滚，不能由旧版继续分配业务。恢复备份会丢失此后的写入。

- 迁移 12 新建 `issue_delegations` 和 issue/id 查询索引，不重建原表、不回填来源。版本 11 升级前保存 `before-migration-12.db`；新库/升级库结构一致。旧二进制可忽略新增表继续既有操作，但不登记或展示 Agent 接票；停服务切回旧二进制时记录仍保留，恢复备份会丢失此后的写入。

- 迁移 13 新建 `work_logs` 及票内倒序/决策查询索引，保留全部 comments、作者、业务字段、委派与引用，不猜测历史来源。版本 12 升级前保存 `before-migration-13.db`，新库和升级库结构一致。旧二进制可忽略新增表，停服换回旧版后记录仍在库中，但旧接口不会读写或展示；恢复升级前备份会丢失之后的数据。

- 迁移 14 新建 `relations_blocks_insert` / `relations_blocks_update` 触发器，使用递归可达性检查保证 `blocks` 无环；不改表、引用、时间或其他关系类型。安装后在迁移事务中校验所有旧 blocks；发现旧环时报 `dependency cycle is not allowed`，回滚本项迁移及版本号，不删边。从版本 13 升级前保存 `before-migration-14.db`。新库/升级库结构一致。旧二进制仍受 SQL 约束保护，但不提供新的 Service 错误分类。停服务回滚二进制不会移除触发器；恢复备份会丢失之后写入。

- 迁移 16为 projects 追加可空 start_date/target_date，创建 initiatives、initiative_projects、项目反查索引及跨 Workspace 关联 INSERT/UPDATE 触发器。历史日期保留为空，不改项目、任务、Team、引用或时间。升级前仍按下一项迁移位置备份；新库与升级库结构一致。旧二进制可忽略这些新增字段与表，但不能维护排期或 Initiative；停服务回滚二进制后数据仍在，恢复备份会丢失之后写入。重建 projects 时需同时移除两个 initiative_projects_workspace 触发器，完成后恢复，并复制日期列；模拟重建测试覆盖该依赖。

旧环导致升级失败时保留原库与失败前备份，先在隔离副本定位并确定要移除的关系；由维护者确认后停服务修复原库，不自动删边。重试前将同名迁移备份另存到受保护位置，避免现有 `VACUUM INTO` 因目标已存在而失败，再启动升级。

- 迁移 15 新建 team_cycle_settings/cycles，给 issues 追加可空 cycle_id 和索引、同 Team 插入/更新触发器；旧票保持无周期，所有原字段、引用和时间不变。版本 14 升级前保存 before-migration-15.db，新库和升级库结构一致。旧二进制不会执行周期滚动或维护快照，回滚前须停服务；恢复备份会丢失升级后写入。

- 迁移 17 新建 `specs`、`spec_revisions` 及 team 查询索引，加上 `specs` 与 team 同 workspace 的插入/更新触发器和禁止 `spec_revisions` 更新、删除的触发器；不重建原表，不改任何已有 id、引用、编号或时间。版本 16 升级前保存 `before-migration-17.db`，新库和升级库结构一致。旧二进制可忽略新增表继续既有操作，但不读写 Spec；停服务换回旧版后数据仍在库中，恢复升级前备份会丢失之后写入的 Spec 和修订。不要在迁移之外手动更新或删除 `spec_revisions`，触发器会拒绝；需要清理时先停服务并备份。

登记的迁移只覆盖这些历史形状，不代表能把任意旧结构自动转换。

更改 CHECK（例如增加状态）要新建目标表、复制数据、删除旧表、重命名。迁移期间关闭外键，每项迁移在事务内执行并做 `foreign_key_check`，违规回滚该项；此前已经提交的迁移不会随后一项失败一起回滚。新旧路径都要测试，参考 `migrate_test.go`。

既有贡献约定要求迁移 n 前保留 `backups/before-migration-<n>.db`。当前实现是在待执行批次开始前，用下一项版本号生成一次备份，并非批次内每项都备份；修改迁移执行器时需明确这个差异，不能把约定当作已实现保证。

## Schema 票的排队与合并

所有修改 `internal/app/schema.sql` 或 `internal/app/migrate.go` 的票遵守以下规则：

1. 票、分支和计划不预写迁移编号，只写“追加一个迁移”。编号由合并时该项在 `internal/app/migrate.go` 的 `migrations` 中的位置决定（从 1 开始），不能提前占号。
2. 同一时间只执行一张改 schema 的票。设为 `agent_ready` 前，分别用 `list_issues` 查询 `agent_ready`、`in_progress`、`in_review` 的票（`limit: -1`，避免默认上限漏票），再用 `get_issue` 阅读候选票的范围，确认是否也修改上述任一文件；有冲突就用 `blocks` 明确先后顺序，前票合并后再执行后票。门禁流程见 [ticket skill](../../.agents/skills/ticket/SKILL.md#agent_ready-gate)。
3. 合并前 rebase 到最新 `main`。若 `main` 新增了迁移，将自己的未交付迁移挪到 `migrations` 末尾，并按合并后的当前 `schema.sql` 重写其中的建表和重建语句；复制数据的语句也必须保留别人刚加入的列及其数据。已交付迁移仍不得改写。合并流程见 [git-workflow skill](../../.agents/skills/git-workflow/SKILL.md#commits-and-pull-requests)。

完成 rebase 和迁移调整后，重新验证新库与已有库的升级路径，检查要求见 [验证指南](../VALIDATION.md)。

## 写入与同步

创建 issue、更新状态及对应事件、修改负责人及对应审计需要一起成功，沿用 Service 的事务；单语句不另包事务。`inftask import-linear` 把一份 Linear 快照的全部写入放在一个事务里，任何检查或写入失败都不留下数据（包括 `project_teams` 关联）；它打开数据库时会执行迁移，所以运行前要停掉使用该库的服务，并先备份，步骤见 [开发](../DEVELOPMENT.md#从-linear-快照导入)。数据库使用 WAL、外键和单个打开连接，不能以单连接推断外部 GitHub 操作也具备事务性。

GitHub 同步每轮顺序处理仓库，某仓库失败（包括绑定的 workspace 还不存在）会记录日志，下轮再读；游标按 workspace 和仓库分别记录，仅在完整读取成功后推进，仓库改绑到另一个 workspace 后第一轮从头读取。PR 关联可重复 upsert，自动状态只按既有条件向前推进，不应增加额外重试。

linkback 先发送 GitHub 评论，再记录本地时间戳。远端成功但响应丢失或本地记录失败时，下轮可能再次评论；当前不保证恰好一次。改变此流程要明确部分成功状态，不能以本地事务声称外部副作用可回滚。

Agent 接票在同一事务内检查任务的人类 assignee 非空，执行 `INSERT ... ON CONFLICT DO NOTHING` 登记来源，并将非 `in_progress` 状态改为 `in_progress`，更新 updated_at 和追加状态事件；任何一步失败全部回滚。已是 `in_progress` 时不刷新时间或增加事件。同一 issue/agent/session_id/agent_id/hostname 的并发或重复请求保留首次记录，但每次调用仍须有负责人，并执行上述状态推进（包括从 `in_review`、`done`、`canceled` 重新开始）。清空负责人不删除历史。详情按审计 id 排序，不根据客户端时间或 session ID 推断接手顺序。记录不提供进程状态、租约或排他性。

## 备份与恢复

启动后检查每日备份，此后每小时检查；使用 `VACUUM INTO` 生成 UTC 日期命名的 `daily-YYYY-MM-DD.db`，保留最新 14 份，每日已有副本则跳过。迁移前副本不参加日常清理。备份失败记录日志，不代表业务写入自动停止。

恢复时先停止使用该数据库的服务，保留需调查的数据，再按 README 清理对应 WAL/SHM 并恢复选定备份。运行中不能用普通 `cp` 复制主数据库冒充一致备份。开发验证只能操作自身临时目录；维护者数据恢复不属于常规测试。

验证入口见 [验证指南](../VALIDATION.md)。

## 网页会话持久化

`internal/ucauth/sessions.go` 将有上限的会话集合加密后写入同目录 `0600` 临时文件，Sync、关闭、rename 后再 Sync 目录。业务 SQLite 备份不包含 `.sessions` 文件；它是可通过重新登录重建的凭据状态。正常重启读回同一文件；损坏或密钥不匹配时启动失败，运维保留后移走才能重新登录。不要恢复旧会话快照，以免重新激活已注销会话。单实例使用，不能共享给两个写进程。

登录和注销在落盘失败时返回 503，不声称已完成。UC token rotation 与本地落盘不具备跨系统事务；刷新成功但落盘失败后仍保留进程内新 token，重启可能需要重新登录，不自动回滚或重放上游刷新请求。

## Cycle 自动推进

启动提供请求前、每分钟以及授权后的周期读取执行补算。每个 Team 在单事务内生成周期、按结束顺序保存快照并转移未完成任务；失败回滚整队，重复执行不重复结算。跨多轮停机按顺序追赶，空周期也形成历史。日期按 IANA 时区使用 AddDate 推进，不以固定小时数代替本地周；二进制嵌入 tzdata。

停用先补算已过期周期，再结算已开始周期，清空未来周期的任务归属并删除未来计划。恰好在新周期起点停用时将零时长周期作为未开始计划移除。重新启用必须从今天或之后且不与历史重叠的日期开始；进行中的节奏不能直接变更，减少未来数量保留已生成计划。周期保存当时的时区，历史统计和边界不会随新配置变化。只有新周期分配/滚动会改变任务的 cycle_id，完成快照不随之后改票重算；未保存历史成员快照。
