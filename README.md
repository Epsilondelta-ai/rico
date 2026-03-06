# Rico

**Remote Claude Code Operator**

[English](docs/README-en.md) | 한국어 | [简体中文](docs/README-zh.md) | [日本語](docs/README-ja.md) | [Español](docs/README-es.md) | [Português (BR)](docs/README-pt-br.md) | [Français](docs/README-fr.md) | [Русский](docs/README-ru.md) | [Deutsch](docs/README-de.md)

### Q. 이게 뭐임?

**A. 누워서 Claude Code랑 작업하고 싶어서 만들었습니다.**

모바일에서 Claude Code를 사용할 수 있게 해주는 PWA 클라이언트예요. Go 브릿지 서버를 통해 Claude Code CLI와 통신하고, 실시간 WebSocket 연결로 빠른 응답을 제공합니다.

![Rico Screenshots](./screenshots/rico-mockup.png)

## Features

- **Mobile-First PWA**: 홈 화면에 설치하여 네이티브 앱처럼 사용
- **Real-time Chat**: WebSocket 기반 실시간 대화
- **Push Notifications**: 새 메시지 도착 시 푸시 알림
- **File Browser**: 서버 파일 시스템 탐색 및 파일 보기
- **Session Management**: 대화 세션 저장 및 관리
- **SOUL System**: 커스터마이징 가능한 AI 페르소나
- **Skills**: 슬래시 커맨드로 확장 기능 사용
- **i18n**: 다국어 지원 (한국어/영어)

## Tech Stack

| Component | Technology |
|-----------|------------|
| Frontend | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| Backend | Go (gorilla/websocket) |
| Communication | WebSocket + REST API |
| Deployment | PWA (Progressive Web App) |

---

## Requirements

### 필수 요구사항

| 요구사항 | 버전 | 용도 |
|---------|------|------|
| Node.js | 18+ | 프론트엔드 빌드 |
| Go | 1.21+ | 서버 빌드 |
| Claude Code CLI | latest | AI 통신 |
| HTTPS + Domain | - | PWA 필수 요건 (아래 참조) |

### HTTPS와 도메인이 필요한 이유

PWA의 핵심 기능들(Service Worker, Push Notifications, 홈 화면 설치)은 **보안상의 이유로 HTTPS에서만 동작**합니다. 또한 SSL 인증서 발급을 위해서는 **도메인**이 필요합니다.

도메인과 SSL 인증서를 얻는 방법은 여러 가지가 있지만, 이 프로젝트에서는 Tailscale을 사용했습니다. 다른 방법(Cloudflare Tunnel, ngrok, 자체 도메인 등)을 사용해도 됩니다.

---

## Tailscale 설정 (이 프로젝트에서 사용한 방법)

Tailscale은 VPN 서비스인데, 부가적으로 도메인과 SSL 인증서를 무료로 제공해줍니다. 개인 프로젝트에서 HTTPS 설정이 귀찮을 때 편하게 쓸 수 있어서 선택했습니다.

### 1. Tailscale 설치

**PC (서버가 실행될 컴퓨터):**
- https://tailscale.com/download 에서 OS에 맞는 버전 설치
- 설치 후 로그인 (Google, GitHub 등)

**모바일:**
- App Store / Play Store에서 "Tailscale" 설치
- **같은 계정**으로 로그인

### 2. 도메인 확인

Tailscale 설치 후 터미널에서:

```bash
tailscale status
```

출력 예시:
```
100.94.195.110  your-machine    your-email@...
```

도메인 형식: `your-machine.tail1234.ts.net`

> Tailscale Admin Console(https://login.tailscale.com/admin)에서도 확인 가능

### 3. SSL 인증서 발급

```bash
# 인증서 발급 (무료)
tailscale cert your-machine.tail1234.ts.net
```

발급된 파일:
- `your-machine.tail1234.ts.net.crt` (인증서)
- `your-machine.tail1234.ts.net.key` (개인키)

이 파일들을 `server/certs/` 폴더에 복사하세요.

---

## Claude Code CLI 설치

```bash
# npm으로 설치
npm install -g @anthropic-ai/claude-code

# 로그인
claude login
```

> Claude Code CLI 로그인 후 사용 가능합니다. 별도의 API 키 설정은 필요 없습니다.

---

## Installation

### 1. Clone

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. VAPID 키 생성

푸시 알림을 위한 VAPID 키를 먼저 생성합니다:

```bash
npx web-push generate-vapid-keys
```

출력 예시:
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

이 키들을 메모해두세요. 아래 설정에서 사용합니다.

### 3. Server 설정

```bash
cd server
go mod download
cp .env.example .env
```

`server/.env` 수정:
```env
# VAPID 키 (위에서 생성한 키)
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# 서버 포트
SERVER_PORT=8080

# SSL 인증서 (Tailscale에서 발급받은 파일 경로)
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. Frontend 설정

```bash
cd ../web
npm install
cp .env.example .env
```

`web/.env` 수정:
```env
# API 서버 주소 (Tailscale 도메인 사용)
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# VAPID 공개키 (server/.env와 동일한 공개키)
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. 빌드 및 실행

프로젝트 루트(`rico/`)에서 실행합니다.

**Windows:**
```bash
scripts\run-windows.bat
```

**Linux/macOS:**
```bash
chmod +x scripts/run-linux.sh   # 최초 1회
./scripts/run-linux.sh
```

스크립트가 의존성 설치 → Frontend 빌드 → Server 빌드 → 서버 실행을 순서대로 처리합니다.

성공 시 로그:
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### 모바일에서 접속

1. 모바일에서 Tailscale 앱 실행 (연결 확인)
2. 브라우저에서 `https://your-machine.tail1234.ts.net:8080` 접속

**iOS (Safari):**
3. 하단 공유 버튼 → **"홈 화면에 추가"** 선택
4. PWA로 설치 완료

**Android (Chrome):**
3. 메뉴(⋮) → **"홈 화면에 추가"** 또는 **"앱 설치"** 선택
4. PWA로 설치 완료

> 이 프로젝트는 iOS Safari에서 테스트되었습니다. Android에서도 동작할 것으로 예상되지만 테스트되지 않았습니다.

---

## Project Structure

```
rico/
├── scripts/
│   ├── run-windows.bat     # Windows용 빌드 & 실행
│   └── run-linux.sh        # Linux/macOS용 빌드 & 실행
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
│   ├── certs/              # SSL 인증서 폴더
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

### 환경변수 요약

#### Server (`server/.env`)

| 변수 | 필수 | 설명 |
|------|------|------|
| `VAPID_PUBLIC_KEY` | O | 푸시 알림 공개키 |
| `VAPID_PRIVATE_KEY` | O | 푸시 알림 비밀키 |
| `SERVER_PORT` | - | 서버 포트 (기본: 8080) |
| `SSL_CERT_FILE` | O | SSL 인증서 파일 경로 |
| `SSL_KEY_FILE` | O | SSL 키 파일 경로 |

#### Web (`web/.env`)

| 변수 | 필수 | 설명 |
|------|------|------|
| `VITE_VAPID_PUBLIC_KEY` | O | 푸시 알림 공개키 (서버와 동일) |
| `VITE_API_BASE` | O | API 서버 주소 (https://...) |

---

## Customization

### SOUL (AI Persona)

`context/personas/default/SOUL.md`를 수정하여 AI의 성격, 말투, 행동 방식을 커스터마이징할 수 있습니다.

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

기본 설정은 최소한의 지시만 포함되어 있어, Claude가 자연스럽게 응답합니다. 필요에 따라 상세한 페르소나를 추가할 수 있습니다.

### Language (i18n)

앱 내 설정 메뉴에서 언어를 변경할 수 있습니다. 현재 한국어와 영어를 지원합니다.

**새 언어 추가 방법:**

1. `web/src/locales/` 폴더에 `{언어코드}.json` 생성 (예: `ja.json`)
2. `ko.json` 구조를 복사하여 번역
3. `web/src/lib/i18n.ts` 수정:
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. 언어 토글 UI 업데이트 (SessionListScreen.svelte)

---

## Troubleshooting

### 푸시 알림이 안 와요

- VAPID 키가 올바르게 설정되었는지 확인
- `server/.env`와 `web/.env`의 **공개키가 동일**한지 확인
- 브라우저에서 알림 권한이 허용되었는지 확인

### HTTPS 연결이 안 돼요

- Tailscale이 PC와 모바일 모두에서 연결되어 있는지 확인
- SSL 인증서 파일 경로가 올바른지 확인
- 인증서가 만료되지 않았는지 확인 (3개월마다 갱신 필요)
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # 재발급
  ```

### "사이트에 연결할 수 없음" 오류

- 서버가 실행 중인지 확인
- 모바일에서 Tailscale 앱이 연결되어 있는지 확인
- 도메인 주소가 정확한지 확인

### Claude Code 응답이 없어요

- `claude login`으로 로그인되어 있는지 확인
- 서버 로그에서 에러 확인: `server/logs/`
- Claude Code CLI가 정상 동작하는지 테스트:
  ```bash
  claude "Hello"
  ```

---

## Development

### 로컬 개발 (HTTP)

SSL 인증서 없이 로컬에서 테스트할 때:

**Terminal 1 - Server:**
```bash
cd server
# .env에서 SSL_CERT_FILE, SSL_KEY_FILE 주석 처리
go run main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
npm run dev
```

> 주의: HTTP 모드에서는 푸시 알림, PWA 설치 등이 동작하지 않습니다.

---

## Note

이 프로젝트는 개인 데스크탑에서 실행하는 셀프 호스팅 도구입니다. 별도의 인증 시스템은 포함되어 있지 않으므로, 네트워크 접근 제어(Tailscale VPN 등)를 통해 보안을 관리해주세요.

---

## Contributing

Issues와 Pull Requests를 환영합니다.

### 개발 환경 세팅

1. 이 저장소를 Fork & Clone
2. [Installation](#installation) 단계를 따라 환경 설정
3. [Development](#development) 섹션을 참고하여 로컬 개발 서버 실행

### PR 가이드라인

- 커밋 메시지는 영어로 작성
- 기존 코드 스타일을 따라주세요
- 가능하면 변경 사항에 대한 설명을 PR 본문에 포함

---

## License

MIT License - see [LICENSE](LICENSE)
