# NovelGen Web UI

NovelGen 的现代化 Web 界面，提供直观的图形界面来管理小说创作流程。

## 功能特性

- 📊 **项目概览** - 查看项目统计和创作进度
- 📖 **大纲管理** - 可视化浏览和编辑故事大纲
- 👥 **角色管理** - 查看和管理故事角色
- 🗺️ **地点管理** - 管理故事中的地点
- 📦 **物品管理** - 管理故事中的物品
- 📝 **草稿管理** - 生成和改进草稿章节
- 📄 **章节管理** - 生成最终章节并导出
- ⚡ **任务队列** - 实时监控 AI 生成任务

## 技术栈

- **后端**: Go + Gin + WebSocket
- **前端**: React + TypeScript + Tailwind CSS + Vite

## 快速开始

### Windows

```bash
./start.bat
```

### Linux/Mac

```bash
./start.sh
```

### 手动启动

1. 构建前端:
```bash
cd frontend
npm install
npm run build
cd ..
```

2. 启动后端:
```bash
go mod tidy
go run .
```

3. 打开浏览器访问 `http://localhost:8080`

### 安全说明

后端默认只监听 `127.0.0.1`，并且只接受来自本机的浏览器请求（其它站点发起的跨站请求会被拒绝）。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-host` | `127.0.0.1` | 监听地址。绑定到非回环地址（如 `0.0.0.0`）时会自动生成 API token，必须在启动日志给出的带 `?token=` 的 URL 打开页面 |
| `-port` | `8080` | 监听端口 |
| `-auth-token` | 自动生成 | 固定 API token；也可通过 `X-NovelGen-Token` 请求头传递 |
| `-projects-root` | `../books` | 项目目录；只有该目录和服务器工作目录下的项目可以访问 |
| `-allow-project-root` | 空 | 额外的可访问项目根目录，逗号分隔 |
| `-no-browser` | `false` | 启动时不自动打开浏览器 |

不要在没有 token 的情况下把服务暴露到公网或不可信的局域网。

## 开发模式

### 前端开发服务器

```bash
cd frontend
npm run dev
```

前端开发服务器运行在 `http://localhost:5173`，会自动代理 API 请求到后端。

### 后端开发

```bash
go run .
```

后端 API 服务器运行在 `http://localhost:8080`。

## 项目结构

```
webui/
├── main.go              # 后端入口
├── go.mod               # Go 依赖
├── start.bat            # Windows 启动脚本
├── start.sh             # Linux/Mac 启动脚本
├── frontend/            # 前端项目
│   ├── src/
│   │   ├── components/  # React 组件
│   │   ├── api.ts       # API 客户端
│   │   ├── types.ts     # TypeScript 类型
│   │   └── ...
│   └── package.json
└── README.md
```

## API 端点

- `GET /api/projects` - 列出所有项目
- `POST /api/projects` - 创建新项目
- `GET /api/content/outline` - 获取大纲
- `GET /api/content/characters` - 获取角色
- `GET /api/content/locations` - 获取地点
- `GET /api/content/items` - 获取物品
- `GET /api/content/chapters` - 获取章节
- `GET /api/content/drafts` - 获取草稿
- `POST /api/tasks` - 创建任务
- `GET /api/tasks` - 列出任务
- `GET /ws` - WebSocket 连接

## 工作流程

1. **初始化项目** - 在 NovelGen CLI 中创建项目
2. **生成大纲** - 使用 Web UI 生成故事大纲
3. **创建世界元素** - 生成角色、地点、物品
4. **生成草稿** - 基于大纲生成草稿章节
5. **改进草稿** - 使用 AI 改进草稿质量
6. **生成章节** - 生成最终润色的章节
7. **导出小说** - 导出完整的小说文件
