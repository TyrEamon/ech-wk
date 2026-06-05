# ECH Workers Stitch GUI Design

这是一个参考 `D:\Desktop\stitch` 设计稿的未来 Wails 版 GUI 视觉和交互原型。当前目录先保持为静态 HTML/CSS/JS，方便直接打开看方向，不影响现有 PyQt5 版本。

## 技术方向

- 外壳：Wails
- 前端：React + TypeScript + Tailwind CSS
- 内核：现有 Go `ech-workers` 逻辑继续作为代理核心
- 前端职责：页面、动画、表单、日志展示、状态展示
- Go 后端职责：启动/停止代理、系统代理开关、配置读写、测速、日志流

## 设计系统

- 风格：minimal line UI / black and white utility
- 深色背景：`#090a0d`
- 浅色背景：`#fbfaf7`
- 主面板：使用黑白灰变量，不使用霓虹高亮
- 高亮方式：只用文字反相、描边加粗、单色填充
- 字体：UI 使用 Plus Jakarta Sans，技术数据使用 JetBrains Mono
- 圆角：主卡片 8-12px，电源按钮 34px，FAB 全圆
- 动效：150-180ms，主要使用 opacity/transform，支持 reduced motion
- 图标：通用导航/操作用 CSS `.icon-*` 组件，服务器节点统一使用 `#icon-globe` 小地球 SVG sprite

## 页面结构

1. Home
   - 大连接按钮
   - 当前服务器下拉切换
   - 延迟、监听地址、分流模式摘要
   - 实时 session/log 小窗

2. Servers
   - 左侧服务器列表
   - 顶部搜索、测速、新建服务器
   - 右侧配置编辑面板
   - 删除后回到服务器列表，不停在已删除项

3. Logs
   - 终端式运行日志
   - 清空日志

不再单独做设置页。底部导航只有 `Home / Proxies / Logs`，配置页作为 `Proxies` 内的详情页进入。

## Wails 后端建议接口

```ts
ListServers(): Promise<ServerProfile[]>
GetCurrentServer(): Promise<ServerProfile>
SelectServer(id: string): Promise<ServerProfile>
SaveServer(profile: ServerProfile): Promise<void>
DeleteServer(id: string): Promise<ServerProfile>
StartProxy(id: string): Promise<void>
StopProxy(): Promise<void>
SetSystemProxy(enabled: boolean): Promise<void>
TestLatency(target: string): Promise<LatencyResult>
SubscribeLogs(callback: (line: LogLine) => void): Unsubscribe
```

## 下一步落地顺序

1. 确认这个视觉方向。
2. 新建 `gui-wails/`，用 Wails 初始化 React + TypeScript 前端。
3. 把本原型拆成 React 组件：`HomePage`、`ServersPage`、`LogsPage`、`ServerCard`、`ConfigForm`。
4. Go 后端先接配置读写和日志，再接启动/停止代理。
5. GitHub Actions 增加 Wails Windows 打包任务。
