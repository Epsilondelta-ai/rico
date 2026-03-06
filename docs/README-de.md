# Rico

**Remote Claude Code Operator**

[English](README-en.md) | [한국어](../README.md) | [简体中文](README-zh.md) | [日本語](README-ja.md) | [Español](README-es.md) | [Português (BR)](README-pt-br.md) | [Français](README-fr.md) | [Русский](README-ru.md) | Deutsch

### Q. Was ist das?

**A. Ich habe das gemacht, weil ich im Liegen mit Claude Code arbeiten wollte.**

Es ist ein PWA-Client, mit dem man Claude Code auf Mobilgeräten nutzen kann. Er kommuniziert über einen Go-Bridge-Server mit der Claude Code CLI und liefert schnelle Antworten über WebSocket-Verbindungen in Echtzeit.

![Rico Screenshots](../screenshots/rico-mockup.png)

## Funktionen

- **Mobile-First PWA**: Auf dem Startbildschirm installieren und wie eine native App nutzen
- **Echtzeit-Chat**: WebSocket-basierte Echtzeit-Konversation
- **Push-Benachrichtigungen**: Push-Alerts bei neuen Nachrichten
- **Dateibrowser**: Server-Dateisystem durchsuchen und Dateien anzeigen
- **Sitzungsverwaltung**: Gesprächssitzungen speichern und verwalten
- **SOUL-System**: Anpassbare KI-Persona
- **Skills**: Erweiterte Funktionen über Slash-Befehle
- **i18n**: Mehrsprachige Unterstützung (Koreanisch/Englisch)

## Technologie-Stack

| Komponente | Technologie |
|------------|-------------|
| Frontend | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| Backend | Go (gorilla/websocket) |
| Kommunikation | WebSocket + REST API |
| Deployment | PWA (Progressive Web App) |

---

## Anforderungen

### Voraussetzungen

| Anforderung | Version | Zweck |
|-------------|---------|-------|
| Node.js | 18+ | Frontend-Build |
| Go | 1.21+ | Server-Build |
| Claude Code CLI | latest | KI-Kommunikation |
| HTTPS + Domain | - | PWA-Anforderung (siehe unten) |

### Warum HTTPS und eine Domain benötigt werden

Die Kernfunktionen von PWA (Service Worker, Push-Benachrichtigungen, Startbildschirm-Installation) **funktionieren aus Sicherheitsgründen nur über HTTPS**. Außerdem wird eine **Domain** benötigt, um SSL-Zertifikate zu erhalten.

Es gibt verschiedene Möglichkeiten, eine Domain und SSL-Zertifikate zu bekommen, aber dieses Projekt verwendet Tailscale. Sie können auch andere Methoden verwenden (Cloudflare Tunnel, ngrok, eigene Domain usw.).

---

## Tailscale-Einrichtung (in diesem Projekt verwendete Methode)

Tailscale ist ein VPN-Dienst, der auch kostenlose Domains und SSL-Zertifikate bereitstellt. Es ist praktisch, wenn man eine einfache HTTPS-Einrichtung für persönliche Projekte möchte.

### 1. Tailscale installieren

**PC (Computer, auf dem der Server läuft):**
- Von https://tailscale.com/download für Ihr Betriebssystem installieren
- Nach der Installation anmelden (Google, GitHub usw.)

**Mobilgerät:**
- "Tailscale" aus dem App Store / Play Store installieren
- Mit dem **gleichen Konto** anmelden

### 2. Domain überprüfen

Nach der Installation von Tailscale im Terminal ausführen:

```bash
tailscale status
```

Beispielausgabe:
```
100.94.195.110  your-machine    your-email@...
```

Domain-Format: `your-machine.tail1234.ts.net`

> Sie können auch in der Tailscale Admin-Konsole nachsehen (https://login.tailscale.com/admin)

### 3. SSL-Zertifikat erhalten

```bash
# Zertifikat erhalten (kostenlos)
tailscale cert your-machine.tail1234.ts.net
```

Generierte Dateien:
- `your-machine.tail1234.ts.net.crt` (Zertifikat)
- `your-machine.tail1234.ts.net.key` (privater Schlüssel)

Kopieren Sie diese Dateien in den Ordner `server/certs/`.

---

## Claude Code CLI Installation

```bash
# Über npm installieren
npm install -g @anthropic-ai/claude-code

# Anmelden
claude login
```

> Verfügbar nach Anmeldung bei Claude Code CLI. Keine separate API-Key-Konfiguration erforderlich.

---

## Installation

### 1. Klonen

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. VAPID-Schlüssel generieren

Generieren Sie zunächst VAPID-Schlüssel für Push-Benachrichtigungen:

```bash
npx web-push generate-vapid-keys
```

Beispielausgabe:
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

Notieren Sie diese Schlüssel. Sie werden in der Konfiguration unten verwendet.

### 3. Server-Konfiguration

```bash
cd server
go mod download
cp .env.example .env
```

`server/.env` bearbeiten:
```env
# VAPID-Schlüssel (oben generiert)
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# Server-Port
SERVER_PORT=8080

# SSL-Zertifikat (Pfade zu Tailscale-Dateien)
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. Frontend-Konfiguration

```bash
cd ../web
npm install
cp .env.example .env
```

`web/.env` bearbeiten:
```env
# API-Serveradresse (Tailscale-Domain verwenden)
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# VAPID öffentlicher Schlüssel (gleich wie server/.env)
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. Bauen und ausführen

Vom Projektverzeichnis (`rico/`) ausführen.

**Windows:**
```bash
scripts\run-windows.bat
```

**Linux/macOS:**
```bash
chmod +x scripts/run-linux.sh   # nur beim ersten Mal
./scripts/run-linux.sh
```

Das Skript verarbeitet der Reihe nach: Abhängigkeiten installieren → Frontend bauen → Server bauen → Server starten.

Erfolgslog:
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### Zugriff vom Mobilgerät

1. Tailscale-App auf dem Mobilgerät starten (Verbindung überprüfen)
2. `https://your-machine.tail1234.ts.net:8080` im Browser öffnen

**iOS (Safari):**
3. Teilen-Button antippen → **"Zum Home-Bildschirm"** auswählen
4. PWA-Installation abgeschlossen

**Android (Chrome):**
3. Menü (⋮) → **"Zum Startbildschirm hinzufügen"** oder **"App installieren"** auswählen
4. PWA-Installation abgeschlossen

> Dieses Projekt wurde auf iOS Safari getestet. Es sollte auf Android funktionieren, wurde aber nicht getestet.

---

## Projektstruktur

```
rico/
├── scripts/
│   ├── run-windows.bat     # Windows Build & Ausführung
│   └── run-linux.sh        # Linux/macOS Build & Ausführung
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
│   ├── certs/              # SSL-Zertifikate-Ordner
│   └── .env.example
├── context/                # Kontextsystem
│   ├── personas/           # Persona-Einstellungen
│   │   ├── active.json     # Aktuell aktive Persona
│   │   └── default/        # Standard-Persona
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Agent-Regeln
└── README.md
```

---

## Konfiguration

### Zusammenfassung der Umgebungsvariablen

#### Server (`server/.env`)

| Variable | Erforderlich | Beschreibung |
|----------|-------------|--------------|
| `VAPID_PUBLIC_KEY` | Ja | Öffentlicher Schlüssel für Push-Benachrichtigungen |
| `VAPID_PRIVATE_KEY` | Ja | Privater Schlüssel für Push-Benachrichtigungen |
| `SERVER_PORT` | Nein | Server-Port (Standard: 8080) |
| `SSL_CERT_FILE` | Ja | Pfad zur SSL-Zertifikatsdatei |
| `SSL_KEY_FILE` | Ja | Pfad zur SSL-Schlüsseldatei |

#### Web (`web/.env`)

| Variable | Erforderlich | Beschreibung |
|----------|-------------|--------------|
| `VITE_VAPID_PUBLIC_KEY` | Ja | Öffentlicher Schlüssel für Push-Benachrichtigungen (gleich wie Server) |
| `VITE_API_BASE` | Ja | API-Serveradresse (https://...) |

---

## Anpassung

### SOUL (KI-Persona)

Sie können die Persönlichkeit, den Sprachstil und das Verhalten der KI anpassen, indem Sie `context/personas/default/SOUL.md` bearbeiten.

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

Die Standardeinstellung enthält minimale Anweisungen, sodass Claude natürlich antworten kann. Sie können nach Bedarf detaillierte Personas hinzufügen.

### Sprache (i18n)

Sie können die Sprache im Einstellungsmenü der App ändern. Derzeit werden Koreanisch und Englisch unterstützt.

**Neue Sprache hinzufügen:**

1. Erstellen Sie `{Sprachcode}.json` im Ordner `web/src/locales/` (z.B.: `ja.json`)
2. Kopieren Sie die Struktur von `ko.json` und übersetzen Sie
3. Bearbeiten Sie `web/src/lib/i18n.ts`:
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. Aktualisieren Sie die Sprachumschaltungs-UI (SessionListScreen.svelte)

---

## Fehlerbehebung

### Push-Benachrichtigungen funktionieren nicht

- Prüfen Sie, ob die VAPID-Schlüssel korrekt konfiguriert sind
- Bestätigen Sie, dass die **öffentlichen Schlüssel identisch** in `server/.env` und `web/.env` sind
- Prüfen Sie, ob Benachrichtigungsberechtigungen im Browser erlaubt sind

### HTTPS-Verbindung funktioniert nicht

- Überprüfen Sie, ob Tailscale sowohl auf dem PC als auch auf dem Mobilgerät verbunden ist
- Prüfen Sie, ob die SSL-Zertifikatsdateipfade korrekt sind
- Prüfen Sie, ob das Zertifikat abgelaufen ist (muss alle 3 Monate erneuert werden)
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # neu ausstellen
  ```

### Fehler "Seite nicht erreichbar"

- Prüfen Sie, ob der Server läuft
- Bestätigen Sie, dass die Tailscale-App auf dem Mobilgerät verbunden ist
- Prüfen Sie, ob die Domain-Adresse korrekt ist

### Keine Antwort von Claude Code

- Überprüfen Sie die Anmeldung mit `claude login`
- Prüfen Sie die Server-Logs auf Fehler: `server/logs/`
- Testen Sie, ob die Claude Code CLI korrekt funktioniert:
  ```bash
  claude "Hello"
  ```

---

## Entwicklung

### Lokale Entwicklung (HTTP)

Zum lokalen Testen ohne SSL-Zertifikate:

**Terminal 1 - Server:**
```bash
cd server
# SSL_CERT_FILE, SSL_KEY_FILE in .env auskommentieren
go run main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
npm run dev
```

> Hinweis: Im HTTP-Modus funktionieren Push-Benachrichtigungen, PWA-Installation usw. nicht.

---

## Hinweis

Dies ist ein selbst gehostetes Tool, das auf Ihrem persönlichen Desktop läuft. Es enthält kein eingebautes Authentifizierungssystem. Bitte verwalten Sie die Sicherheit über Netzwerkzugangskontrolle (z.B. Tailscale VPN).

---

## Mitwirken

Issues und Pull Requests sind willkommen.

### Entwicklungsumgebung einrichten

1. Fork & Clone dieses Repositories
2. Folgen Sie den [Installations](#installation)-Schritten zur Umgebungskonfiguration
3. Beziehen Sie sich auf den [Entwicklung](#entwicklung)-Abschnitt, um den lokalen Entwicklungsserver zu starten

### PR-Richtlinien

- Commit-Nachrichten auf Englisch schreiben
- Dem bestehenden Code-Stil folgen
- Wenn möglich eine Beschreibung der Änderungen im PR-Body einfügen

---

## Lizenz

MIT License - siehe [LICENSE](LICENSE)
