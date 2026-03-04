# Project Structure

## Rico

**Mobile Claude Code Client (Svelte PWA)**

- Frontend: Svelte + Vite (PWA)
- Backend: Go bridge server
- Communication: WebSocket + REST API

```
rico/
├── web/                    # Svelte PWA Frontend
│   ├── src/
│   │   ├── App.svelte      # Main app component
│   │   ├── app.css         # Global styles
│   │   └── lib/
│   │       ├── ChatScreen.svelte   # Chat screen
│   │       └── websocket.ts        # WebSocket client
│   ├── public/             # Static files (icons, etc.)
│   └── package.json
├── server/                 # Go bridge server
│   └── main.go             # API server + WebSocket hub
├── context/                # Context system (CLAUDE.md table of contents)
│   ├── SOUL.md             # Current persona (English)
│   ├── SOUL.default.md     # Default persona (English)
│   ├── souls/              # Persona variants
│   │   └── EB-ko.md        # Korean persona option
│   ├── i18n.md             # Internationalization & speak tag system
│   ├── CHANGELOG.md        # Change history
│   ├── memory.md           # User promises/rules
│   └── ...
├── CLAUDE.md               # Agent rules (table of contents)
├── scripts/                # Build and run scripts
└── rico.exe                # Built Go server
```

## Tech Stack

| Area | Technology |
|------|------------|
| Frontend | Svelte, Vite, TypeScript |
| Backend | Go (gorilla/websocket) |
| Deployment | PWA (Mobile home screen install) |
| Communication | WebSocket (real-time), REST API |
