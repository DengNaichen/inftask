# macOS 云端客户端

Electron 外壳固定连接 <https://harnessagent.tokensinfinity.com/>，业务页面、权限、API 与数据都来自现有云端。首版仅打包 Apple Silicon（arm64），不包含 Go 服务或本地 SQLite，不启动监听端口，不改变 Claude/Codex 的云端 MCP 配置，不提供离线业务、自动更新或开机自启。

## 开发与本地打包

Node 范围与前端一致，见 [开发](../docs/DEVELOPMENT.md)。在仓库根目录：

```sh
npm --prefix desktop ci
npm --prefix desktop test
npm --prefix desktop start
```

桌面构建不依赖 `make build`、Go 或 `web/node_modules`：

```sh
npm --prefix desktop run dist:mac
codesign --verify --deep --strict desktop/dist/mac-arm64/InfTask.app
```

输出：

- `desktop/dist/mac-arm64/InfTask.app`
- `desktop/dist/InfTask-0.1.0-arm64.dmg`

只需要 App 目录时用 `npm --prefix desktop run pack:mac`。构建需要 macOS、Xcode Command Line Tools、网络和足够空间，首次安装/启动及打包会下载 Electron 和构建工具。版本号来自 `desktop/package.json`。

DMG 提供拖到 Applications 的安装入口。不要把源码、依赖目录或 `dist` 加入 Git；`dist` 中的 blockmap 不是自动更新功能。

默认显式使用 `mac.identity: "-"` 做 **ad-hoc 本地签名**，不会自动选择维护者 Keychain 中的 Developer ID。这是让本地 Apple Silicon App 的签名完整，不代表 Apple 信任或公证。保留 Hardened Runtime；electron-builder 默认 entitlement 包含 JIT 和关闭 library validation，后者用于无 Team ID 的 ad-hoc App。不要用 `CSC_IDENTITY_AUTO_DISCOVERY=false` 跳过签名，否则重打包后的 Electron 签名可能不完整。

`electron-builder` 固定版本；`@electron/get` 的 `global-agent` 覆盖到 4.1.3，避免旧代理日志依赖引入的 sprintf-js DoS 通告。升级时重新执行 audit、fixture smoke 与 DMG 构建；覆盖只影响构建工具，不进入 App 的运行依赖。

## 登录、窗口和恢复

- 第一次在 App 内完成现有 UC 登录；UC 页面允许的 HTTPS origin 是 `https://www.tokensinfinity.com`，回调回到 InfTask。桌面会话不共享 Safari/Chrome 的 cookies。
- 登录状态取决于现有云端会话和 UC 续期。Electron 在独立的持久化 session 中保存 cookies；正常退出后有效会话仍可使用，不保证永不要求重新登录。
- 本机 `app.getPath('userData')`（通常为 `~/Library/Application Support/InfTask/`）只有 Chromium 会话、缓存、页面 localStorage 等客户端状态，不是云端业务备份。该目录可能含登录凭据，不提交、不上传排障。
- 红色关闭按钮关闭窗口，App 留在 Dock；点击 Dock 或再次启动会重开/聚焦主窗口。`⌘Q` 完全退出。没有本地后台服务。
- `⌘R` 刷新；View 菜单提供 Home、缩放和全屏，保留原生编辑菜单。
- 云端已有的重新登录新窗口会使用同一受限 session，不覆盖原页面的未保存草稿。外部 HTTP(S) 链接交给系统浏览器，其他协议不启动本机程序。
- 主页面加载失败、HTTP 5xx 或 renderer 崩溃时显示本地恢复页；Retry / `⌘R` 从 InfTask Home 重试。恢复页不会保存失败页面的草稿。已经加载页面的 API 失败仍由现有网页处理，桌面不重发业务写入。

## 安全边界

远程页面关闭 Node integration，没有 preload 或 IPC 桥；启用 context isolation、renderer sandbox 和 webSecurity。主窗口、frame、新窗口和 HTTP 重定向只允许固定 InfTask/UC HTTPS origin；其他 origin 不在 Electron 中加载。子资源仍按浏览器规则加载，不把图片/CDN 当成可导航页面。

拒绝页面权限、设备权限、webview 和下载，不忽略证书错误，不修改后端 CORS/Origin/认证规则。session 的持久化不改变服务端权限。安全细节见 [安全边界](../docs/rules/SECURITY.md)。

## 验证

```sh
npm --prefix desktop test
npm --prefix desktop audit
# macOS + openssl；可省略输出目录，此时测试结束删除所有临时文件。
npm --prefix desktop run smoke -- /tmp/inftask-desktop-evidence
# 只读正式实例：仅到 UC 登录页，不填写或提交真实账号。
npm --prefix desktop run smoke -- /tmp/inftask-desktop-evidence --cloud
# 先 dist:mac，再检查真正打包的 App，而不只是开发入口。
npm --prefix desktop run smoke:packaged -- /tmp/inftask-desktop-evidence
```

fixture smoke 使用随机 loopback HTTPS 端口、临时 profile、虚构身份和单独生成的证书，覆盖 UC 式跨 origin 跳转、HttpOnly cookie 跨进程保留、同 session 登录窗口保留草稿、外链、拒绝未知重定向、关闭/重开、拒绝自签证书、HTTP 503 以及服务停止/重试。只在 **不打入 App 的测试入口** 按证书 SHA-256 指纹信任自己的 fixture。

packaged smoke 临时启用 loopback DevTools，使用临时 profile，仅渲染真实 UC 登录页；结束后退出自己启动的 App 并清理 profile。不要在已经登录真实账号的 App 上运行 DevTools 排障或保存完整 URL、cookie、token。

自动检查不证明真实 UC 登录成功。最终人工验收：从 DMG 安装，完成 UC 登录进入已有工作区；退出/重开仍有有效会话；Sign out 后重开不恢复会话；在会话过期时完成新窗口登录后返回原窗口。仅浏览即可，不需要创建测试业务数据。另需在一台非开发机器上验收 Gatekeeper，签名检查成功不代表下载后可以直接打开。

## 正式分发

默认 DMG 仅供本地试用，不是可直接公开分发的已公证版本。正式分发需要发布者自己的 Apple Developer ID Application 证书及 Apple 公证凭据；不得把凭据写进仓库、命令记录或日志。

由发布者在已配置证书和受保护环境变量的环境中运行：

```sh
npm --prefix desktop run dist:mac -- \
  -c.mac.identity="Developer ID Application: <发布者名称> (<TEAM_ID>)" \
  -c.mac.notarize=true
codesign --verify --deep --strict desktop/dist/mac-arm64/InfTask.app
xcrun stapler validate desktop/dist/mac-arm64/InfTask.app
spctl --assess --type execute --verbose desktop/dist/mac-arm64/InfTask.app
```

electron-builder 可从环境读取 `APPLE_API_KEY` / `APPLE_API_KEY_ID` / `APPLE_API_ISSUER`，或 `APPLE_ID` / `APPLE_APP_SPECIFIC_PASSWORD` / `APPLE_TEAM_ID`。选择一组，不在代码中放任何值。其公证流程处理 App 的提交和 stapling；分发前检查签名、staple 和下载后 Gatekeeper。正式签名和公证不由本地试用命令自动执行。

后续网页版本由云端发布；Electron/Chromium 安全更新、外壳和图标变更仍需要发布新 App。手动替换 App 不应清理 userData。

## 图标

`desktop/assets/icon.icns` 是已提交的构建输入，来自共享 [Icon Composer 源文件](../assets/InfTask.icon)。常规打包不要求 Icon Composer。源图层改变后，安装含 design generation 27 的 Xcode Icon Composer，再运行：

```sh
npm --prefix desktop run icon
```

脚本只使用临时目录导出 Default/macOS 图像并生成 ICNS，不修改网页 favicon。
