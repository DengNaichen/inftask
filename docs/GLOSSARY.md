# 项目术语

| 术语 | 在本项目中的含义与边界 |
| --- | --- |
| workspace | 一组 team、Project 和标签，也是云端成员访问边界。由不能修改的 slug 命名：页面在 `/<slug>/`，REST 在 `/api/w/<slug>/`，MCP 在 `/w/<slug>/mcp`。不能删除。不是 Git 工作树或 HarnessKit 的 workspace membership。 |
| Admin / Member | 工作区内的人类角色，采用 Linear 的基础成员模型。Admin 管理邀请、成员角色、停用/恢复和工作区改名；Member 可协作处理当前公开团队的业务数据。新建者为 Admin；已有工作区显式初始化。Guest 与 Agent 独立授权尚未实现。 |
| Team owner | 团队管理角色，创建团队或将其转为私有的人默认成为 owner；Workspace Admin 有团队管理能力，但读私有任务仍须显式加入。团队可以没有 owner，不应用工作区的最后 Admin 保护。 |
| private team | 只有显式团队成员能读写其中任务；普通成员不可发现未加入的私有团队。Admin 可在设置目录发现并管理，确认加入后才有内容访问权。公开团队允许未加入的工作区成员协作。 |
| suspended | 某人在某个工作区被停用，保留历史记录；后续 REST/MCP 请求失去访问权，邀请不能自行恢复。由活动 Admin 恢复，不能移除最后一个活动 Admin。 |
| assignee | 单一人类负责人，可为空；代表责任归属，不是编辑锁。任何有任务写权限的活动成员可改派或清空。候选必须是活动工作区成员且可访问任务团队；停用保留历史显示。Agent 执行身份不写入此字段；委派要求已有人的负责人。 |
| delegation / delegate_issue | 当前主或子 Agent 显式登记参与一张已有人类负责人的票，记录宿主 agent/session/subagent/hostname 与可信操作者。认领成功时原子地设为 `in_progress`（含重复调用及从 `in_review`、`done`、`canceled` 重新开始）；已在进行中不刷新时间或追加状态事件。保留人的 assignee，不是编辑锁、任务调度或进程存活证明。相同执行身份去重，不同子 Agent 即使共享父 session 也各有记录；清空负责人保留历史。 |
| Issue / ticket | 同一工作记录，标识符由服务端生成，形如 `<team key>-<number>`，编号由所属 team 依次分配，只在所属 workspace 内唯一。不能推测下一个编号。 |
| Spec | 和 Issue 并列的实体，描述系统应该是什么样：长期存在、不断演进、不会“完成”；Issue 则是实现它的一次变化。标识符 `SPEC-<n>`，在 workspace 内递增，属于一个 team，可见性与该 team 的任务一致。内容是按 GitHub Spec Kit `spec-template.md` 组织的结构化 JSON（带 `US-n` 的用户故事、`FR-n` 需求、`SC-n` 成功标准和澄清记录），每次保存追加一个不可修改的修订。不是 issue 的 `type=spec`（那是共享一个用户可见结果的父票，之后会改名）。 |
| team | 有自己票号前缀（key）和独立编号的 issue 归属，如本仓库的 `AL`。建票必须指定 team，没有默认值；key 在 workspace 内唯一，建好后不能改，显示名称可改，team 不能删除或换 workspace，票不能换 team。状态所有 workspace 共用，Project 显式关联同一 workspace 的一个或多个 team。不是 GitHub team 或 UC 用户组。 |
| Project | 按交付结果组织 ticket 的具名记录，不是 Git 仓库或源码目录。名称在 workspace 内唯一，可通过稳定 id 改名；当前名称也可寻址，旧名称不是别名。内部 id、Team 关联和任务引用不随改名变化。显式关联同一 workspace 的一个或多个 Team，空项目也按关联出现在 Team 列表。历史无任务项目保留待归属，仅 workspace 列表可见；新建必须指定 Team。任务只能属于兼容 Team 的项目；归档后默认列表不展示且不接受新任务归属，已有任务可保留。 |
| triage / backlog / todo | triage 是新进来、还没决定要不要的票；backlog 是决定保留、但还没承诺什么时候做的票；todo 是已承诺要做的工作，本身不表示可直接由 Agent 执行。 |
| Cycle | Team 的时间规划维度，与 Project、status、agent_ready 独立；每票最多一个同 Team 周期。按配置时区的日历周重复，结束时已承诺未完成任务自动滚入下一轮；结束统计是快照，当前票列表可继续变化。不等于发布版本或依赖图中的环。 |
| Project Timeline | 以 Project 为横条的计划时间轴，使用可选的开始与目标日期；不是 Issue 级甘特图。无日期项目留在未排期区，单端日期显示标记。Workspace、Team 和 Initiative 的项目集合都可查看，不要求先加入 Initiative。 |
| Initiative | Workspace 内围绕共同目标组织多个 Project 的记录；项目可加入多个 Initiative，也可不加入。首版只有名称、说明、归档和项目关联，没有层级。元数据对活动 Workspace 成员可见，关联列表仅返回调用者可见的 Project；不是 Team、Cycle 或 Project 的必填父级。 |
| agent_ready | 通过仓库 ticket skill 的执行门槛后进入 Agent 队列。门槛是贡献流程，不能宣称 Service 自动审核 ticket 描述。 |
| in_progress / in_review / done / canceled | 正在处理、PR 可审阅、合并后完成、取消或被替代。GitHub 同步仅按源码中的特定规则自动推进。 |
| type | 可选分类：`spec`、`feature`、`bug`、`improvement`、`chore`；与 status 相互独立。 |
| label / label group | workspace 内的 issue 标签；普通标签多选，单层标签组内互斥。标签 ID 稳定、名称可改、创建后所属组固定；归档保留关联和筛选，合并转移关联并删除源 ID。与 type、Project 和 status 独立。 |
| parent / children | 工作归属与子任务。不能自引用；当前服务不检查较长的 parent 环。 |
| blocks / blocked_by | 有方向的阻塞关系；`blocked_by` 以反向 `blocks` 存储。Service 与 SQLite INSERT/UPDATE 触发器均禁止成环，依赖关系构成 DAG；`related` 与 parent 不参与此图。 |
| related | 无方向关联，数据库规范化两端顺序以避免重复。与 parent 的归属含义不同。 |
| link / pull_requests | 前者是人工附加的标题和 URL；后者是 GitHub 同步记录的 PR，有仓库、编号与状态。 |
| linkback | 默认关闭的 GitHub 回写：把 issue 标题、描述作为评论发到关联的 open/draft PR。不是本地 link。 |
| events / comments | events 记录创建与状态变化；comments 用于人的讨论，Agent 仅在用户明确要求代发时使用。历史评论可能混有人与 Agent 的记录，保留且不推断来源。评论不能编辑或删除；`author` 只有从 Linear 导入的评论才有，本地追加的为空。 |
| work log / work_logs | Agent 追加的工作记录，含简短摘要和详细正文，供追溯、验证和交接；与人的 comments 分开，不能编辑或删除。表示内容用途，不是命令输出、委派登记或可信 Agent 身份。 |
| decision | work log 的一种：已确定的选择及其背景和理由，详情页单独提取决策摘要。提议、执行进展、测试结果使用 progress；decision 不是待决问题票或独立 ADR 文件。 |

定义以 `internal/app` 的 `workspace.go`、`issue.go`、`project.go`、`team.go`、`schema.sql`、`github.go`、MCP 工具描述及 [ticket skill](../.agents/skills/ticket/SKILL.md) 为依据。
