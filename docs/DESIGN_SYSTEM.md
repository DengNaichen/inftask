# 共享视觉约定

## 来源与复用

`web/src/app.css` 的 `:root` 定义基础尺度和 light 颜色；`@media (prefers-color-scheme: dark)` 只覆盖主题颜色与 `color-scheme`。无深色偏好时使用 light，页面打开期间跟随系统变化。使用 Inter Variable，没有手动主题开关或主题偏好存储。`main.tsx` 在共享样式后依次加载 `issue-views.css`、`issue-detail.css` 和 `navigation.css`，分别承载列表/看板、详情/任务表单及导航/筛选的局部布局，沿用同一套 token。

基础尺度保留现有紧凑密度，消费规则用 CSS custom properties；原始视觉值集中在 token 定义。同一个数值承担不同角色时可分别命名，避免改一个控件时意外改变另一个。

| 类别 | Token 与用途 |
| --- | --- |
| 表面与交互 | `--bg` 画布、`--panel` 面板、`--raised` 抬升表面、`--surface-card` 卡片；`--group-gradient-start`/`--group-gradient-end` 分组渐变；`--hover`/`--selected`、`--button-bg`/`--button-hover`、`--chip-bg`、`--card-hover`、`--drop-bg` 分开表达状态 |
| 文字与边界 | `--text`、`--text-2`、`--text-3`、`--muted`、`--faint`；`--border`/`--border-strong` 与 `--link`/`--link-underline` |
| 操作与反馈 | `--accent`/`--accent-hover` 配 `--accent-text`；`--success`、`--error-text`/`--error-bg`/`--error-border`；`--pr-open`/`--pr-merged`/`--pr-closed` 各有 border token |
| 弹窗与代码 | `--dialog-bg`、`--backdrop`、`--shadow-dialog`、`--shadow-card`、`--code-bg` |
| Agent 标志 | `--agent-mark-bg`/`--agent-mark-color`：属性栏 agent 标志的白色圆底与深色标志，明暗主题相同 |
| 间距与字体 | `--space-*` 是现有像素间距尺度；`--space-prose-*` 是随正文大小变化的 em 间距；`--font-size-*`、`--weight-*`、`--line-*`、`--tracking-*`、`--font-ui`、`--mono`、`--font-features`、`--font-optical-heading` 控制字体 |
| 形状与尺寸 | `--radius*`、`--border-width`、`--focus-width`、`--icon-size`/`--icon-stroke`、`--status-icon-size`；`--sidebar-width`、`--board-column-width`、`--detail-properties-width`、`--detail-property-row-height`、`--agent-mark-glyph-size`、`--content-max-width`、`--dialog-width`、`--composer-width` 等表达组件尺寸 |
| 侧栏角色 | `--sidebar-font-*`/`--sidebar-line-height`、`--sidebar-caption-size`、`--sidebar-row-height`/`--sidebar-identity-*`、`--sidebar-icon-size`/`--sidebar-emoji-size`/`--sidebar-avatar-size`、`--sidebar-inset`/`--sidebar-*-gap`/`--sidebar-child-inset`、`--sidebar-radius`、`--sidebar-*-color`、`--sidebar-hover`/`--sidebar-selected`/`--sidebar-selected-hover`；在 `app.css` 引用基础 token，由 `navigation.css` 统一消费 |
| 其他共享值 | `--opacity-disabled`、`--duration-fast`、`--font-code-relative`、`--code-tab-size`、`--scrollbar-thumb`；列表三种控件共用 `--list-control-size`，日期列用 `--list-date-width` |

light 使用浅色画布、白色面板、深色文字和靛蓝操作色；状态色按浅色背景调整明度。dark 使用深色画布、面板和抬升卡片。AL-54 以已登录的 Linear 实际深色界面为布局与密度参考：36px 分组栏、44px 列表行、12px 主面板圆角及 8px 卡片圆角和间距。两套颜色都在 `app.css`，是 InfTask 的设计选择；不声称所有窗口、主题或像素都与 Linear 一致。[Refero 的 Linear 风格参考](https://styles.refero.design/style/90ce5883-bb24-4466-93f7-801cd617b0d1) 仅作为此前的 token 组织参考，没有提供 light 配色，也不是 Linear 官方应用规范。

共享状态控件与元信息在 `shared.tsx` 的 `StatusIcon`、`StatusSelect`、`IssueMeta`；`.button`/`.primary`、`.chip`、`.error`、`.issue-dialog` 与 `.icon` 在 CSS 中统一消费。改全局角色先改已有 token 或共享规则，局部布局则定位消费选择器，不另造平行主题系统。滚动条全局使用 `thin`、透明轨道及 `--scrollbar-thumb`，实际呈现随浏览器和系统滚动条设置变化。

允许保留的字面量边界：

- `0`、百分比、viewport 单位、`auto`/`none`/`transparent`/`currentColor`、Grid 列位置与 flex 比例表达布局结构；旋转角度表达箭头几何。
- 状态和优先级 SVG 的 viewBox、路径、圆心、半径、扇形、虚线和属性尺寸分别是 14×14 和 16×16 几何；实际显示尺寸由 CSS token 控制。lucide 路径由库管理；agent 标志的 24×24 viewBox 与路径按来源图标包原样复制。
- CSS media 条件保留 `1024px` 和 `760px`：原生 custom property 不能用于 media 条件。不能为 token 整理改变断点或引入另一套预处理系统。
- `LabelBadge` 的 `label.color` 与标签编辑器的初始颜色是业务数据，不随主题改写。新增组件颜色不属于此例外。
- `web/index.html` 的两条 `theme-color` meta 分别用带 media 的 `#f5f6f8` 和 `#08090a`，必须与两个 `--bg` 同步；meta content 不能直接引用 CSS var。这是 HTML 平台边界，其他消费点不可照此复制色值。

## 应用标识

应用图标使用奶油底色与桃色、珊瑚色、橙色层叠任务卡片，前景为浅色任务线条和勾选。可编辑源文件是 [`assets/InfTask.icon`](../assets/InfTask.icon)，包含四个 SVG 图层和四个深度组；背景为 `#fff4e1`。应用图标保留自身配色，明暗主题使用同一导出；Workspace 入口及工作区头像属于另一类标识，本次不修改。

`web/src/assets/inftask-icon.png` 为 128px 导出，供 HTML favicon 使用；Vite 构建时产生带内容哈希的资源地址。图片由 Icon Composer 的 Default/macOS 外观导出，图层修改后重新导出。

安装包含 design generation 27 的 Icon Composer 后，可在仓库根目录使用它自带的 `ictool`（不同于 `xcrun ictool`）重新生成，命令中的输入和输出使用绝对路径：

```sh
icon_tool='/Applications/Xcode.app/Contents/Applications/Icon Composer.app/Contents/Executables/ictool'
"$icon_tool" "$PWD/assets/InfTask.icon" --export-image --output-file "$PWD/web/src/assets/inftask-icon.png" --platform macOS --rendition Default --width 128 --height 128 --scale 1 --design-generation 27
```

网页运行和常规构建只消费已提交的 PNG，不要求安装 Icon Composer。macOS Electron 打包使用 `desktop/assets/icon.icns`，由同一源文件导出 Default/macOS 外观；图层改变后运行 `npm --prefix desktop run icon` 更新 ICNS，常规桌面打包也不要求 Icon Composer。桌面业务页面直接沿用云端样式，没有第二套业务 UI；本地恢复页用系统字体和同一 light/dark 画布、文字与操作色角色，不依赖网络加载字体。

## 图标

标签使用 `shared.tsx` 的 `LabelBadge`：颜色仅用于小圆点，文字沿用主题 token；长名称截断，title 显示完整名称和描述，归档标记单独展示。管理页和选择器样式集中在 `app.css` 的标签区块。

界面图标来自 `lucide-react`（ISC 许可）。组件写 `className="icon"`，尺寸和描边由 `.icon` 统一设为 16px、1.5，不在组件上传 `size` 或 `strokeWidth`。现有对应：`List` 是全部 issue 和列表视图，`Columns2` 是看板视图，`Box` 是 Project，`ChevronDown` 是分组折叠，`Search` 是搜索，`Tag` 是 type 筛选，`X` 是清除筛选和关闭弹窗。lucide 在没有 accessible name 时自动加 `aria-hidden`；只有图标的按钮在按钮上写 `aria-label`。

详情属性栏用 `AgentMark.tsx` 标出登记执行的 coding agent：Codex 显示 OpenAI 标志，Claude 显示 Anthropic 标志，放在 16px 圆形白底上，外加 `--border` 细边以便在 light 白色面板上可见。路径数据复制自 `@lobehub/icons-static-svg@1.95.1`（MIT）的 `icons/openai.svg` 与 `icons/anthropic.svg`，没有新增依赖；标志属于各自商标所有者，只用于标识 agent，不用于其他界面。人的头像是 16px 圆形首字母（`personName` 首字符），用 `--button-bg` 底和 `--text-2` 字，不按人着色；详情属性与新建弹窗的 Assignee 共用。

状态图标由 `shared.tsx` 的 `StatusIcon` 在 14×14 网格上自绘，按 14px 显示，不用 lucide，也不从其他项目复制代码。形状写在组件里，`statusColors: Record<Status, string>` 指向 `--status-*` 颜色 token。dark 颜色参照 Linear 的状态配色：todo、in_progress、done 按 Linear 截图取色；triage、backlog 采用 AL-32 指定的颜色；in_review、canceled 的取值没有找到公开来源核实；agent_ready 在 Linear 没有对应状态，用自定的紫色。

| 状态 | 形状 | Token | dark | light |
| --- | --- | --- | --- | --- |
| triage | 实心圆加 ⇄ 形状的双箭头 | `--status-triage` | `#fc7840` | `#fc7840` |
| backlog | 虚线圈 | `--status-backlog` | `#bec2c8` | `#737b86` |
| todo | 空心圈 | `--status-todo` | `#e2e2e2` | `#626974` |
| agent_ready | 空心圈加中心小点 | `--status-agent-ready` | `#b49bdd` | `#8665b0` |
| in_progress | 圈内填一半扇形 | `--status-in-progress` | `#f2c94c` | `#987000` |
| in_review | 圈内填四分之三扇形 | `--status-in-review` | `#4cb782` | `#258253` |
| done | 实心圆加对勾 | `--status-done` | `#5e6ad2` | `#5e6ad2` |
| canceled | 实心圆加叉 | `--status-canceled` | `#95a2b3` | `#697789` |

双箭头、对勾和叉用 `--panel` 色画在实心圆上。侧栏里状态图标和 16px 图标同列，`.nav-item .status-icon` 左右各留 1px 让文字对齐。列表的选择、优先级及状态控件各占 24px；分组头紧凑排列展开箭头、状态、名称和数量，不保留任务行的空列。

## Workspace 菜单

左上角 `WorkspaceMenu.tsx` 使用已有 Radix Popover 与原生 ModalDialog：触发器显示名称和 ChevronDown，长名称截断并用 title 提供完整名称；紧凑列表显示文字头像、名称、与名称不同的 slug 和当前项 Check，长名称及 slug 用 title 提供全文。弹层消费已有 sidebar 宽度、主题、边界、圆角及菜单阴影 token，受视口宽高限制，列表内部滚动。新建和改名复用 `.issue-dialog` 表单；窄屏隐藏 sidebar 分组时保留 Workspace 触发器。账户入口位于侧栏底部，名称截断，打开账户弹层后展示身份和 Sign out。

## 项目与团队 emoji

`EmojiPicker.tsx` 使用 `@radix-ui/react-popover`（MIT）。项目标题与团队 Settings 表单提供 emoji 选择入口；空值沿用 Project 的 Box 和 team 的缩写。布局参考本地 Linear 应用的 Emoji 页：顶部搜索、Frequently used 和带标题的连续分类网格；只提供 emoji，没有 Icons 页和颜色选择。桌面弹层宽/高约 450px、14 列，窄屏按可用宽度减少列数（390px 视口为 11 列），宽度不超过视口减 16px，高度按可用空间缩短，内容内部滚动。`--emoji-picker-width`、`--emoji-picker-height`、`--emoji-cell-height` 和 `--font-emoji` 控制尺寸与系统 emoji 字体，其余消费现有主题 token。原生 dialog 内的 portal 挂在 dialog 上。

目录来自 Unicode 16.0 的 fully-qualified emoji 数据，存于 `internal/app/emojidata/emoji.json`，Go 和前端共用；来源与 Unicode 许可见同目录 LICENSE 和 `emoji.go`。默认分类浏览省略肤色变体，按完整英文名称或粘贴 emoji 可搜索这些序列。表情用本机字体绘制，不同系统的字形可能不同。Frequently used 由最近成功选择及常用默认项组成，浏览器 localStorage 仅保存最多 28 个 emoji，无外部图片/CDN 请求。

## 优先级

Priority 的档位与 [Linear](https://linear.app/docs/issue-property#priority) 对齐：No priority、Urgent、High、Medium、Low，对应 0–4。`shared.tsx` 的 `PriorityIcon` 自绘 16px 图形，参照官方菜单截图：未设置为三段横线，紧急为圆角方块加感叹号，其他档位为三根递增的实心柱、以亮柱数量表示级别。紧急使用 `--priority-urgent`（dark `#f2994a`、light `#a75b14`，项目自定强调色），其余消费现有文字 token；菜单阴影使用 `--shadow-priority-menu`、宽度使用 `--priority-menu-width`、未亮柱透明度使用 `--priority-muted-opacity`；不声称所有像素或取色与 Linear 完全一致。

`PrioritySelect` 使用开源 `@radix-ui/react-select` 基础控件（MIT），项目 CSS 消费现有 light/dark token 控制菜单、悬停、当前项勾选和焦点；没有引入 shadcn/Tailwind。列表和看板为 24px 图标触发器，详情及创建/编辑弹窗展示图标加名称。列表共享列依次为选择、优先级、编号、状态、标题、元信息及日期。原生 dialog 内的菜单 portal 挂到 dialog 本身，保持在 modal 的 top layer 中，否则会被遮挡或无法操作。

## 状态与可访问外观

状态图标供侧栏、列表、看板和详情复用。`StatusSelect` 使用 Radix Select，与优先级共享菜单表面；列表/卡片展示带 accessible name 的 24px 图标按钮，详情展示图标加名称。菜单保留全部八种状态，支持键盘操作，原生 dialog 内的 portal 挂到该 dialog。`.status-control:focus-within` 显示焦点，链接和按钮有 `:focus-visible` 描边。禁用按钮降低不透明度并改变 cursor；保存流程见 [交互](INTERACTION_DESIGN.md)。

列表与卡片的复选框默认仅视觉隐藏，仍在 Tab 顺序中；条目 hover、focus-within、已选中或已有批量选择时显示。无 hover 或粗指针设备始终显示。选中条目消费 `--selected`，卡片同时强调边界。Team 侧栏无改名按钮；编辑集中在 Settings 正式页面。

增加状态需同时核对共享 `columns` 和 `StatusIcon` 的颜色与图形（两张表按 `Status` 类型定义，缺项时前端构建报错），再在列表、卡片及详情检查，不凭单页截图证明全部场景。仓库没有自动视觉回归测试或对比度认证。

## 范围导航与项目列表

Sidebar 按 Workspace 与 Team 分组，各自的任务/项目是链接，当前项消费 `.nav-item.active`。Workspace 导航项是整栏的 baseline：导航行使用 13px 字号、500 字重、16px 文字行高、28px 高度、8px 内容间距与左右内距、6px 圆角，相邻行间距为 1px；Workspace 与 Team 共用同一条导航行规则和文字、hover、selected token。所有侧栏 SVG 图标为 14px，原生 emoji 使用 14px 字号及 14px 方形图标槽，Team 缩写槽也是 14px；子导航相对 Team 标题的额外缩进为图标槽与内容间距之和的一半，即 11px，加上基础内距后为 19px，团队之间总间距为 8px。Workspace 与底部账户入口共用 32px 身份行、13px 字号、590 字重和 24px 头像 token，菜单文字与箭头消费同一套侧栏 token。分组说明共用 12px 字号及 muted 颜色；只有交互行显示 hover，Team 标题为展开/收起按钮，最右侧使用 14px 箭头，展开向下、收起向右。侧栏布局规则集中在 `navigation.css`，角色 token 集中在 `app.css`；全局其它页面的 `.icon` 仍使用 16px。

Workspace 分组的 Settings 链接直达设置页。Settings 沿用主面板，居中内容最大 768px，各团队表单展示名称、emoji 和只读编号；窄屏字段上下排列。名称输入框与图标按钮均高 32px，图标按钮为方形，内部 emoji 使用 14px 字号和 16px 方形容器。任务工具栏提供 Filter 和 Display 弹层：前者容纳状态、Team、type、priority 与标签筛选，后者容纳 List/Board、排序及 Select visible issues；搜索和结果数留在工具栏，已应用筛选显示可清除 chip。批量工具栏仅在当前范围有选中条目时出现。`ProjectList.tsx` 将项目名称链接、说明和关联 Team 排成紧凑行，长文本截断，复用主题文字/边界/间距与 EntityEmoji。窄屏保留 Workspace 入口，用 Navigation 按钮展开可滚动导航；选择目的地重新挂载并收起。项目说明、待归属/归档提示和错误恢复使用现有 token。

## 项目排期

`project-timeline.css` 提供横向滚动日期网格、项目链接、日期条和未排期列表，颜色与控件复用已有主题 token；时间轴专用尺寸集中在该文件的 `:root`。日期区以项目为行，区间用 accent 横条、单日期用圆形标记，今天用细线；日期文本提供完整起止信息和键盘编辑入口。Timeline 容器独立横向滚动，项目较多时在内容区纵向滚动。

Initiative 目录复用 `ProjectList` 的行样式；编辑使用原生 ModalDialog、现有表单和项目 checkbox，归档操作放在编辑表单。首版没有自定义颜色、健康度或层级图标体系。

## 布局

桌面 `.app-shell` 是 220px 侧栏与圆角 12px 的主面板。列表使用 CSS Grid 和 subgrid 共享控件、编号与日期列宽；大分组的虚拟行以占位高度保留自然布局和吸顶范围，标题和元信息在同一 flex 容器内按每条任务的徽章宽度分配空间；编号列用查询结果中各 Team 最长编号预留宽度，避免虚拟行滚动时标题起点跳动，标题起点一致，桌面列间距为 8px、行高 44px。分组按钮高 36px，背景从左到右微渐变；吸顶由不透明的外层承载，顶部留白与圆角后方都由面板色遮挡，滚动容器本身没有顶部 padding。列表左右留白为 16px，760px 以下为 8px；空组文字及第二行元信息与编号列起点对齐。

看板横向滚动，各状态列内的卡片独立纵向滚动；列宽默认 320px，并按窄视口收缩。卡片使用 8px 圆角和间距，顶部是淡化的编号，状态在标题旁，优先级、Project、标签及 type 位于下方；没有常驻拖动符号，整张卡片仍可拖动。

详情的标题、正文和属性组共用同一画布及纵向滚动区域，属性没有通高边界或独立滚动。属性栏参照 Linear 的详情侧栏，依次是 Properties（Status、Priority、Assignee）、Labels、Project，以及有父票时的 Parent；分组只用 muted 12px 小标题和 24px 间距，没有分隔线，也不显示创建/更新时间。每行是 16px 图标槽、8px 间距加文字，行高 `--detail-property-row-height`（32px），14px 状态图标在槽内左右各留 1px，值文字起点一致。可点击的行（状态、优先级、Assignee、Project/Parent 链接）和标签行都比图标槽向左外扩 8px，点击区域左边缘对齐，hover 显示 `--hover`；Assignee 保留原生 select，去掉系统箭头，头像叠在 select 左侧内距上，整行可点。可操作的空值用 muted 操作提示（Set priority、Add label），详情不单列 Team 和 Type，标签仍在 Labels 组查看和编辑；只读空值写 No project，未指派为 muted Unassigned。登记过的 agent 作为 Assignee 的子行：agent 标志与负责人名字的起点对齐，点击区域从标志前 8px 开始，不与其它行左对齐；`--border-strong` 的 L 形连线从负责人图标中线连到标志的竖直中线，留在点击区域外，点击滚动到执行记录。桌面属性置于正文右侧；1024px 以下移入主内容流，Properties 排成两列（Assignee 与 agent 子行同一格），Labels、Project、Parent 两列换行。列表最右侧日期来自 `created_at`，悬停显示完整创建时间；标签/type 徽章带彩色圆点，Project 徽章显示项目 emoji，空 emoji 使用青色 Box。桌面列表徽章高 24px，长名称截断并保留 title。列表在 1024px 以下将元信息和创建日期换到第二行，元信息可换行；760px 以下页面切换单列，侧栏目的地收进可展开 Navigation，列表/看板保留 Project 元信息。任务新建与编辑使用最大 720px 的 `.issue-composer`：标题和说明直接排列，归属及其它属性以可换行的紧凑控件展示；其它弹窗保留共享 `.issue-dialog` 尺寸。

最小 body 宽度为 320px。改变断点或共享尺寸时检查两侧布局、长标题、Markdown 表格/代码横向滚动与焦点可见性，按 [验证](VALIDATION.md) 分别报告构建与人工结果。

## Cycle

Team 周期目录使用 CalendarRange 图标，当前周期边框用 --accent；其余周期用 --border，完成进度用原生 progress 和主题 accent。cycles.css 消费共享间距、字号、边框与圆角 token；目录最大宽度沿用 --content-max-width。统计明确区分实时数量与结束快照，窄屏日期独立成行。
