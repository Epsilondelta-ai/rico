# Rico

**Remote Claude Code Operator**

[English](../README.md) | [한국어](README-ko.md) | [简体中文](README-zh.md) | [日本語](README-ja.md) | [Español](README-es.md) | [Português (BR)](README-pt-br.md) | Français | [Русский](README-ru.md) | [Deutsch](README-de.md)

### Q. C'est quoi ?

**A. Je l'ai créé parce que je voulais travailler avec Claude Code en étant allongé.**

C'est un client PWA qui permet d'utiliser Claude Code sur appareils mobiles. Il communique avec Claude Code CLI via un serveur bridge en Go et fournit des réponses rapides grâce aux connexions WebSocket en temps réel.

![Rico Screenshots](../screenshots/rico-mockup.png)

## Fonctionnalités

- **Mobile-First PWA** : Installer sur l'écran d'accueil et utiliser comme une app native
- **Chat en temps réel** : Conversation en temps réel basée sur WebSocket
- **Notifications push** : Alertes push à l'arrivée de nouveaux messages
- **Explorateur de fichiers** : Parcourir le système de fichiers du serveur et afficher les fichiers
- **Gestion de sessions** : Sauvegarder et gérer les sessions de conversation
- **Système SOUL** : Persona IA personnalisable
- **Skills** : Fonctionnalités étendues via commandes slash
- **i18n** : Support multilingue (coréen/anglais)

## Stack technique

| Composant | Technologie |
|-----------|-------------|
| Frontend | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| Backend | Go (gorilla/websocket) |
| Communication | WebSocket + REST API |
| Déploiement | PWA (Progressive Web App) |

---

## Prérequis

### Conditions préalables

| Exigence | Version | Usage |
|----------|---------|-------|
| Node.js | 18+ | Build du frontend |
| Go | 1.21+ | Build du serveur |
| Claude Code CLI | latest | Communication IA |
| HTTPS + Domaine | - | Exigence PWA (voir ci-dessous) |

### Pourquoi HTTPS et un domaine sont nécessaires

Les fonctionnalités principales des PWA (Service Worker, notifications push, installation sur l'écran d'accueil) **ne fonctionnent que via HTTPS** pour des raisons de sécurité. De plus, un **domaine** est nécessaire pour obtenir des certificats SSL.

Il existe plusieurs façons d'obtenir un domaine et des certificats SSL, mais ce projet utilise Tailscale. Vous pouvez aussi utiliser d'autres méthodes (Cloudflare Tunnel, ngrok, domaine propre, etc.).

---

## Configuration de Tailscale (méthode utilisée dans ce projet)

Tailscale est un service VPN qui fournit aussi des domaines et certificats SSL gratuits. C'est pratique quand vous voulez une configuration HTTPS simple pour des projets personnels.

### 1. Installer Tailscale

**PC (ordinateur où le serveur sera exécuté) :**
- Installer depuis https://tailscale.com/download pour votre OS
- Se connecter après l'installation (Google, GitHub, etc.)

**Mobile :**
- Installer "Tailscale" depuis l'App Store / Play Store
- Se connecter avec le **même compte**

### 2. Vérifier le domaine

Après l'installation de Tailscale, exécuter dans le terminal :

```bash
tailscale status
```

Exemple de sortie :
```
100.94.195.110  your-machine    your-email@...
```

Format du domaine : `your-machine.tail1234.ts.net`

> Vous pouvez aussi vérifier sur la Console Admin de Tailscale (https://login.tailscale.com/admin)

### 3. Obtenir le certificat SSL

```bash
# Obtenir le certificat (gratuit)
tailscale cert your-machine.tail1234.ts.net
```

Fichiers générés :
- `your-machine.tail1234.ts.net.crt` (certificat)
- `your-machine.tail1234.ts.net.key` (clé privée)

Copiez ces fichiers dans le dossier `server/certs/`.

---

## Installation de Claude Code CLI

```bash
# Installer via npm
npm install -g @anthropic-ai/claude-code

# Connexion
claude login
```

> Disponible après connexion à Claude Code CLI. Aucune configuration de clé API séparée n'est nécessaire.

---

## Installation

### 1. Cloner

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. Générer les clés VAPID

D'abord, générez les clés VAPID pour les notifications push :

```bash
npx web-push generate-vapid-keys
```

Exemple de sortie :
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

Notez ces clés. Elles seront utilisées dans la configuration ci-dessous.

### 3. Configuration du serveur

```bash
cd server
go mod download
cp .env.example .env
```

Modifier `server/.env` :
```env
# Clés VAPID (générées ci-dessus)
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# Port du serveur
SERVER_PORT=8080

# Certificat SSL (chemins des fichiers de Tailscale)
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. Configuration du frontend

```bash
cd ../web
npm install
cp .env.example .env
```

Modifier `web/.env` :
```env
# Adresse du serveur API (utiliser le domaine Tailscale)
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# Clé publique VAPID (même que server/.env)
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. Compiler et exécuter

Exécuter depuis la racine du projet (`rico/`).

**Windows :**
```bash
scripts\run-windows.bat
```

**Linux/macOS :**
```bash
chmod +x scripts/run-linux.sh   # première fois seulement
./scripts/run-linux.sh
```

Le script gère dans l'ordre : installation des dépendances → build du frontend → build du serveur → démarrage du serveur.

Log de succès :
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### Accéder depuis le mobile

1. Lancer l'app Tailscale sur le mobile (vérifier la connexion)
2. Accéder à `https://your-machine.tail1234.ts.net:8080` dans le navigateur

**iOS (Safari) :**
3. Appuyer sur le bouton de partage → Sélectionner **"Sur l'écran d'accueil"**
4. Installation PWA terminée

**Android (Chrome) :**
3. Menu (⋮) → Sélectionner **"Ajouter à l'écran d'accueil"** ou **"Installer l'application"**
4. Installation PWA terminée

> Ce projet a été testé sur iOS Safari. Il devrait fonctionner sur Android mais n'a pas été testé.

---

## Structure du projet

```
rico/
├── scripts/
│   ├── run-windows.bat     # Build & exécution Windows
│   └── run-linux.sh        # Build & exécution Linux/macOS
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
│   ├── certs/              # Dossier des certificats SSL
│   └── .env.example
├── context/                # Système de contexte
│   ├── personas/           # Paramètres de persona
│   │   ├── active.json     # Persona active actuelle
│   │   └── default/        # Persona par défaut
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Règles de l'Agent
└── README.md
```

---

## Configuration

### Résumé des variables d'environnement

#### Serveur (`server/.env`)

| Variable | Requis | Description |
|----------|--------|-------------|
| `VAPID_PUBLIC_KEY` | Oui | Clé publique des notifications push |
| `VAPID_PRIVATE_KEY` | Oui | Clé privée des notifications push |
| `SERVER_PORT` | Non | Port du serveur (par défaut : 8080) |
| `SSL_CERT_FILE` | Oui | Chemin du fichier de certificat SSL |
| `SSL_KEY_FILE` | Oui | Chemin du fichier de clé SSL |

#### Web (`web/.env`)

| Variable | Requis | Description |
|----------|--------|-------------|
| `VITE_VAPID_PUBLIC_KEY` | Oui | Clé publique des notifications push (même que le serveur) |
| `VITE_API_BASE` | Oui | Adresse du serveur API (https://...) |

---

## Personnalisation

### SOUL (Persona IA)

Vous pouvez personnaliser la personnalité, le style de parole et le comportement de l'IA en éditant `context/personas/default/SOUL.md`.

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

La configuration par défaut inclut des instructions minimales, permettant à Claude de répondre naturellement. Vous pouvez ajouter des personas détaillées selon vos besoins.

### Langue (i18n)

Vous pouvez changer la langue dans le menu des paramètres de l'app. Actuellement, le coréen et l'anglais sont supportés.

**Ajouter une nouvelle langue :**

1. Créer `{code_langue}.json` dans le dossier `web/src/locales/` (ex : `ja.json`)
2. Copier la structure de `ko.json` et traduire
3. Modifier `web/src/lib/i18n.ts` :
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. Mettre à jour l'UI de changement de langue (SessionListScreen.svelte)

---

## Dépannage

### Les notifications push ne fonctionnent pas

- Vérifier que les clés VAPID sont correctement configurées
- Confirmer que les **clés publiques sont identiques** dans `server/.env` et `web/.env`
- Vérifier que les permissions de notification sont autorisées dans le navigateur

### La connexion HTTPS ne fonctionne pas

- Vérifier que Tailscale est connecté sur le PC et le mobile
- Vérifier que les chemins des fichiers de certificat SSL sont corrects
- Vérifier que le certificat n'a pas expiré (renouvellement nécessaire tous les 3 mois)
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # ré-émettre
  ```

### Erreur "Impossible de se connecter au site"

- Vérifier que le serveur est en cours d'exécution
- Confirmer que l'app Tailscale est connectée sur le mobile
- Vérifier que l'adresse du domaine est correcte

### Pas de réponse de Claude Code

- Vérifier la connexion avec `claude login`
- Vérifier les logs du serveur pour des erreurs : `server/logs/`
- Tester si Claude Code CLI fonctionne correctement :
  ```bash
  claude "Hello"
  ```

---

## Développement

### Développement local (HTTP)

Pour tester localement sans certificats SSL :

**Terminal 1 - Serveur :**
```bash
cd server
# Commenter SSL_CERT_FILE, SSL_KEY_FILE dans .env
go run main.go
```

**Terminal 2 - Frontend :**
```bash
cd web
npm run dev
```

> Note : En mode HTTP, les notifications push, l'installation PWA, etc. ne fonctionneront pas.

---

## Note

Ceci est un outil auto-hébergé qui s'exécute sur votre bureau personnel. Il n'inclut pas de système d'authentification intégré, veuillez donc gérer la sécurité via le contrôle d'accès réseau (ex : Tailscale VPN).

---

## Contribuer

Les Issues et Pull Requests sont les bienvenues.

### Configuration de l'environnement de développement

1. Fork & Clone de ce dépôt
2. Suivre les étapes d'[Installation](#installation) pour configurer l'environnement
3. Consulter la section [Développement](#développement) pour lancer le serveur de développement local

### Directives pour les PR

- Écrire les messages de commit en anglais
- Suivre le style de code existant
- Inclure une description de vos modifications dans le corps de la PR quand c'est possible

---

## Licence

MIT License - voir [LICENSE](LICENSE)
