# Rico

**Remote Claude Code Operator**

[English](../README.md) | [한국어](README-ko.md) | 简体中文 | [日本語](README-ja.md) | [Español](README-es.md) | [Português (BR)](README-pt-br.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Deutsch](README-de.md)

### Q. 这是什么？

**A. 因为想躺着用 Claude Code 工作，所以做了这个。**

这是一个可以在移动设备上使用 Claude Code 的 PWA 客户端。通过 Go 桥接服务器与 Claude Code CLI 通信，并通过实时 WebSocket 连接提供快速响应。

![Rico Screenshots](../screenshots/rico-mockup.png)

## 功能

- **Mobile-First PWA**：安装到主屏幕，像原生应用一样使用
- **实时聊天**：基于 WebSocket 的实时对话
- **推送通知**：新消息到达时推送提醒
- **文件浏览器**：浏览服务器文件系统和查看文件
- **会话管理**：保存和管理对话会话
- **SOUL 系统**：可自定义的 AI 人格
- **Skills**：通过斜杠命令使用扩展功能
- **i18n**：多语言支持（韩语/英语）

## 技术栈

| 组件 | 技术 |
|------|------|
| 前端 | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| 后端 | Go (gorilla/websocket) |
| 通信 | WebSocket + REST API |
| 部署 | PWA (Progressive Web App) |

---

## 系统要求

### 前置条件

| 要求 | 版本 | 用途 |
|------|------|------|
| Node.js | 18+ | 前端构建 |
| Go | 1.21+ | 服务器构建 |
| Claude Code CLI | latest | AI 通信 |
| HTTPS + 域名 | - | PWA 必要条件（见下文） |

### 为什么需要 HTTPS 和域名

PWA 的核心功能（Service Worker、推送通知、主屏幕安装）出于安全原因**仅在 HTTPS 下工作**。此外，获取 SSL 证书需要**域名**。

获取域名和 SSL 证书有多种方式，本项目使用了 Tailscale。您也可以使用其他方式（Cloudflare Tunnel、ngrok、自有域名等）。

---

## Tailscale 设置（本项目使用的方式）

Tailscale 是一个 VPN 服务，同时免费提供域名和 SSL 证书。在个人项目中需要简单配置 HTTPS 时非常方便。

### 1. 安装 Tailscale

**PC（运行服务器的电脑）：**
- 从 https://tailscale.com/download 安装对应操作系统的版本
- 安装后登录（Google、GitHub 等）

**移动端：**
- 从 App Store / Play Store 安装 "Tailscale"
- 使用**相同账号**登录

### 2. 确认域名

安装 Tailscale 后，在终端运行：

```bash
tailscale status
```

输出示例：
```
100.94.195.110  your-machine    your-email@...
```

域名格式：`your-machine.tail1234.ts.net`

> 也可以在 Tailscale Admin Console（https://login.tailscale.com/admin）中确认

### 3. 获取 SSL 证书

```bash
# 获取证书（免费）
tailscale cert your-machine.tail1234.ts.net
```

生成的文件：
- `your-machine.tail1234.ts.net.crt`（证书）
- `your-machine.tail1234.ts.net.key`（私钥）

将这些文件复制到 `server/certs/` 文件夹。

---

## 安装 Claude Code CLI

```bash
# 通过 npm 安装
npm install -g @anthropic-ai/claude-code

# 登录
claude login
```

> 登录 Claude Code CLI 后即可使用。无需单独配置 API 密钥。

---

## 安装

### 1. 克隆

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. 生成 VAPID 密钥

首先生成推送通知所需的 VAPID 密钥：

```bash
npx web-push generate-vapid-keys
```

输出示例：
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

记下这些密钥，后续配置中会用到。

### 3. 服务器配置

```bash
cd server
go mod download
cp .env.example .env
```

编辑 `server/.env`：
```env
# VAPID 密钥（上面生成的密钥）
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# 服务器端口
SERVER_PORT=8080

# SSL 证书（Tailscale 颁发的文件路径）
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. 前端配置

```bash
cd ../web
npm install
cp .env.example .env
```

编辑 `web/.env`：
```env
# API 服务器地址（使用 Tailscale 域名）
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# VAPID 公钥（与 server/.env 相同的公钥）
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. 构建和运行

在项目根目录（`rico/`）运行。

**Windows：**
```bash
scripts\run-windows.bat
```

**Linux/macOS：**
```bash
chmod +x scripts/run-linux.sh   # 仅首次
./scripts/run-linux.sh
```

脚本按顺序处理：安装依赖 → 构建前端 → 构建服务器 → 启动服务器。

成功日志：
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### 从移动设备访问

1. 在移动设备上运行 Tailscale 应用（确认连接）
2. 在浏览器中访问 `https://your-machine.tail1234.ts.net:8080`

**iOS (Safari)：**
3. 点击底部分享按钮 → 选择**"添加到主屏幕"**
4. PWA 安装完成

**Android (Chrome)：**
3. 菜单(⋮) → 选择**"添加到主屏幕"**或**"安装应用"**
4. PWA 安装完成

> 本项目在 iOS Safari 上测试过。预计在 Android 上也能运行，但未经测试。

---

## 项目结构

```
rico/
├── scripts/
│   ├── run-windows.bat     # Windows 构建 & 运行
│   └── run-linux.sh        # Linux/macOS 构建 & 运行
├── web/                    # Svelte PWA 前端
│   ├── src/
│   │   ├── App.svelte
│   │   └── lib/
│   │       ├── ChatScreen.svelte
│   │       └── websocket.ts
│   ├── .env.example
│   └── package.json
├── server/                 # Go 桥接服务器
│   ├── main.go
│   ├── certs/              # SSL 证书文件夹
│   └── .env.example
├── context/                # 上下文系统
│   ├── personas/           # 人格设置
│   │   ├── active.json     # 当前活跃人格
│   │   └── default/        # 默认人格
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Agent 规则
└── README.md
```

---

## 配置

### 环境变量概要

#### 服务器 (`server/.env`)

| 变量 | 必需 | 说明 |
|------|------|------|
| `VAPID_PUBLIC_KEY` | 是 | 推送通知公钥 |
| `VAPID_PRIVATE_KEY` | 是 | 推送通知私钥 |
| `SERVER_PORT` | 否 | 服务器端口（默认：8080） |
| `SSL_CERT_FILE` | 是 | SSL 证书文件路径 |
| `SSL_KEY_FILE` | 是 | SSL 密钥文件路径 |

#### Web (`web/.env`)

| 变量 | 必需 | 说明 |
|------|------|------|
| `VITE_VAPID_PUBLIC_KEY` | 是 | 推送通知公钥（与服务器相同） |
| `VITE_API_BASE` | 是 | API 服务器地址 (https://...) |

---

## 自定义

### SOUL（AI 人格）

通过编辑 `context/personas/default/SOUL.md` 来自定义 AI 的性格、说话风格和行为方式。

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

默认设置仅包含最少的指令，让 Claude 自然地响应。您可以根据需要添加详细的人格。

### 语言 (i18n)

可以在应用设置菜单中更改语言。目前支持韩语和英语。

**添加新语言：**

1. 在 `web/src/locales/` 文件夹中创建 `{语言代码}.json`（例如：`ja.json`）
2. 复制 `ko.json` 结构并翻译
3. 编辑 `web/src/lib/i18n.ts`：
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. 更新语言切换 UI（SessionListScreen.svelte）

---

## 故障排除

### 推送通知不工作

- 检查 VAPID 密钥是否正确配置
- 确认 `server/.env` 和 `web/.env` 的**公钥是否一致**
- 检查浏览器中是否允许了通知权限

### HTTPS 连接不工作

- 确认 Tailscale 在 PC 和移动设备上都已连接
- 检查 SSL 证书文件路径是否正确
- 检查证书是否已过期（每 3 个月需要续期）
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # 重新颁发
  ```

### "无法连接到站点"错误

- 检查服务器是否正在运行
- 确认移动设备上 Tailscale 应用是否已连接
- 检查域名地址是否正确

### Claude Code 无响应

- 确认已通过 `claude login` 登录
- 检查服务器日志中的错误：`server/logs/`
- 测试 Claude Code CLI 是否正常工作：
  ```bash
  claude "Hello"
  ```

---

## 开发

### 本地开发（HTTP）

在没有 SSL 证书的情况下本地测试：

**终端 1 - 服务器：**
```bash
cd server
# 在 .env 中注释掉 SSL_CERT_FILE、SSL_KEY_FILE
go run main.go
```

**终端 2 - 前端：**
```bash
cd web
npm run dev
```

> 注意：在 HTTP 模式下，推送通知、PWA 安装等功能将不可用。

---

## 说明

这是一个在个人桌面上运行的自托管工具。不包含内置认证系统，请通过网络访问控制（如 Tailscale VPN）来管理安全。

---

## 贡献

欢迎提交 Issues 和 Pull Requests。

### 开发环境设置

1. Fork & Clone 此仓库
2. 按照 [安装](#安装) 步骤配置环境
3. 参考 [开发](#开发) 部分运行本地开发服务器

### PR 指南

- 提交信息请使用英语
- 请遵循现有的代码风格
- 尽量在 PR 正文中包含变更说明

---

## 许可证

MIT License - 见 [LICENSE](LICENSE)
