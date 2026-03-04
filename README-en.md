# Rico

**Remote Claude Code Operator**

[한국어](README.md) | English

### Q. What is this?

**A. I made this because I wanted to work with Claude Code while lying down.**

It's a PWA client that lets you use Claude Code on mobile devices. It communicates with Claude Code CLI through a Go bridge server and provides fast responses via real-time WebSocket connections.

## Features

- **Mobile-First PWA**: Install on home screen and use like a native app
- **Real-time Chat**: WebSocket-based real-time conversation
- **Push Notifications**: Push alerts when new messages arrive
- **File Browser**: Browse server file system and view files
- **Session Management**: Save and manage conversation sessions
- **SOUL System**: Customizable AI persona
- **Skills**: Extended features via slash commands
- **i18n**: Multi-language support (Korean/English)

## Tech Stack

| Component | Technology |
|-----------|------------|
| Frontend | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| Backend | Go (gorilla/websocket) |
| Communication | WebSocket + REST API |
| Deployment | PWA (Progressive Web App) |

---

## Requirements

### Prerequisites

| Requirement | Version | Purpose |
|-------------|---------|---------|
| Node.js | 18+ | Frontend build |
| Go | 1.21+ | Server build |
| Claude Code CLI | latest | AI communication |
| HTTPS + Domain | - | PWA requirement (see below) |

### Why HTTPS and Domain are Required

PWA core features (Service Worker, Push Notifications, Home Screen Installation) **only work over HTTPS** for security reasons. Additionally, a **domain** is required to obtain SSL certificates.

There are several ways to get a domain and SSL certificates, but this project uses Tailscale. You can also use other methods (Cloudflare Tunnel, ngrok, your own domain, etc.).

---

## Tailscale Setup (Method Used in This Project)

Tailscale is a VPN service that also provides free domains and SSL certificates. It's convenient when you want easy HTTPS setup for personal projects.

### 1. Install Tailscale

**PC (computer where server will run):**
- Install from https://tailscale.com/download for your OS
- Login after installation (Google, GitHub, etc.)

**Mobile:**
- Install "Tailscale" from App Store / Play Store
- Login with the **same account**

### 2. Check Domain

After installing Tailscale, run in terminal:

```bash
tailscale status
```

Example output:
```
100.94.195.110  your-machine    your-email@...
```

Domain format: `your-machine.tail1234.ts.net`

> You can also check on Tailscale Admin Console (https://login.tailscale.com/admin)

### 3. Get SSL Certificate

```bash
# Get certificate (free)
tailscale cert your-machine.tail1234.ts.net
```

Generated files:
- `your-machine.tail1234.ts.net.crt` (certificate)
- `your-machine.tail1234.ts.net.key` (private key)

Copy these files to `server/certs/` folder.

---

## Claude Code CLI Installation

```bash
# Install via npm
npm install -g @anthropic-ai/claude-code

# Login
claude login
```

> Available after logging into Claude Code CLI. No separate API key setup required.

---

## Installation

### 1. Clone

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. Generate VAPID Keys

First, generate VAPID keys for push notifications:

```bash
npx web-push generate-vapid-keys
```

Example output:
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

Save these keys. They will be used in the setup below.

### 3. Server Setup

```bash
cd server
go mod download
cp .env.example .env
```

Edit `server/.env`:
```env
# VAPID keys (generated above)
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# Server port
SERVER_PORT=8080

# SSL certificate (paths to files from Tailscale)
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. Frontend Setup

```bash
cd ../web
npm install
cp .env.example .env
```

Edit `web/.env`:
```env
# API server address (use Tailscale domain)
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# VAPID public key (same as server/.env)
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. Build and Run

Run from project root (`rico/`).

**Windows:**
```bash
scripts\run-windows.bat
```

**Linux/macOS:**
```bash
chmod +x scripts/run-linux.sh   # first time only
./scripts/run-linux.sh
```

The script handles: dependency installation → Frontend build → Server build → server startup in order.

Success log:
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### Access from Mobile

1. Run Tailscale app on mobile (verify connection)
2. Access `https://your-machine.tail1234.ts.net:8080` in browser

**iOS (Safari):**
3. Tap share button → Select **"Add to Home Screen"**
4. PWA installation complete

**Android (Chrome):**
3. Menu (⋮) → Select **"Add to Home Screen"** or **"Install App"**
4. PWA installation complete

> This project was tested on iOS Safari. It's expected to work on Android but hasn't been tested.

---

## Project Structure

```
rico/
├── scripts/
│   ├── run-windows.bat     # Windows build & run
│   └── run-linux.sh        # Linux/macOS build & run
├── web/                    # Svelte PWA Frontend
│   ├── src/
│   │   ├── App.svelte
│   │   └── lib/
│   │       ├── ChatScreen.svelte
│   │       └── websocket.ts
│   ├── .env.example
│   └── package.json
├── server/                 # Go Bridge Server
│   ├── main.go
│   ├── certs/              # SSL certificate folder
│   └── .env.example
├── context/                # Context System
│   ├── personas/           # Persona settings
│   │   ├── active.json     # Current active persona
│   │   └── default/        # Default persona
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Agent Rules
└── README.md
```

---

## Configuration

### Environment Variables Summary

#### Server (`server/.env`)

| Variable | Required | Description |
|----------|----------|-------------|
| `VAPID_PUBLIC_KEY` | Yes | Push notification public key |
| `VAPID_PRIVATE_KEY` | Yes | Push notification private key |
| `SERVER_PORT` | No | Server port (default: 8080) |
| `SSL_CERT_FILE` | Yes | SSL certificate file path |
| `SSL_KEY_FILE` | Yes | SSL key file path |

#### Web (`web/.env`)

| Variable | Required | Description |
|----------|----------|-------------|
| `VITE_VAPID_PUBLIC_KEY` | Yes | Push notification public key (same as server) |
| `VITE_API_BASE` | Yes | API server address (https://...) |

---

## Customization

### SOUL (AI Persona)

You can customize the AI's personality, speech style, and behavior by editing `context/personas/default/SOUL.md`.

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

The default setting includes minimal instructions, allowing Claude to respond naturally. You can add detailed personas as needed.

### Language (i18n)

You can change the language in the app's settings menu. Currently supports Korean and English.

**Adding a new language:**

1. Create `{language_code}.json` in `web/src/locales/` folder (e.g., `ja.json`)
2. Copy `ko.json` structure and translate
3. Edit `web/src/lib/i18n.ts`:
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. Update language toggle UI (SessionListScreen.svelte)

---

## Troubleshooting

### Push notifications not working

- Check if VAPID keys are correctly configured
- Verify that **public keys are identical** in `server/.env` and `web/.env`
- Check if notification permissions are allowed in browser

### HTTPS connection not working

- Verify Tailscale is connected on both PC and mobile
- Check if SSL certificate file paths are correct
- Check if certificate has expired (needs renewal every 3 months)
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # re-issue
  ```

### "Cannot connect to site" error

- Check if server is running
- Verify Tailscale app is connected on mobile
- Check if domain address is correct

### No response from Claude Code

- Verify logged in with `claude login`
- Check server logs for errors: `server/logs/`
- Test if Claude Code CLI works properly:
  ```bash
  claude "Hello"
  ```

---

## Development

### Local Development (HTTP)

When testing locally without SSL certificates:

**Terminal 1 - Server:**
```bash
cd server
# Comment out SSL_CERT_FILE, SSL_KEY_FILE in .env
go run main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
npm run dev
```

> Note: Push notifications, PWA installation, etc. won't work in HTTP mode.

---

## Roadmap

### Next Improvements

1. **Voice Input**: Easier text input via speech

> This is still a work in progress and needs improvement. Contributions are welcome!

---

## License

MIT License - see [LICENSE](LICENSE)

## Contributing

Issues and Pull Requests are welcome.
