# Rico

**Remote Claude Code Operator**

[English](README-en.md) | [한국어](../README.md) | [简体中文](README-zh.md) | [日本語](README-ja.md) | [Español](README-es.md) | Português (BR) | [Français](README-fr.md) | [Русский](README-ru.md) | [Deutsch](README-de.md)

### Q. O que é isso?

**A. Fiz isso porque queria trabalhar com o Claude Code deitado.**

É um cliente PWA que permite usar o Claude Code em dispositivos móveis. Ele se comunica com o Claude Code CLI através de um servidor bridge em Go e fornece respostas rápidas via conexões WebSocket em tempo real.

![Rico Screenshots](../screenshots/rico-mockup.png)

## Funcionalidades

- **Mobile-First PWA**: Instale na tela inicial e use como um app nativo
- **Chat em tempo real**: Conversa em tempo real baseada em WebSocket
- **Notificações push**: Alertas push quando novas mensagens chegam
- **Navegador de arquivos**: Navegue pelo sistema de arquivos do servidor e visualize arquivos
- **Gerenciamento de sessões**: Salve e gerencie sessões de conversa
- **Sistema SOUL**: Persona de IA personalizável
- **Skills**: Funcionalidades estendidas via comandos slash
- **i18n**: Suporte multilíngue (coreano/inglês)

## Stack tecnológica

| Componente | Tecnologia |
|------------|------------|
| Frontend | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| Backend | Go (gorilla/websocket) |
| Comunicação | WebSocket + REST API |
| Deploy | PWA (Progressive Web App) |

---

## Requisitos

### Pré-requisitos

| Requisito | Versão | Finalidade |
|-----------|--------|------------|
| Node.js | 18+ | Build do frontend |
| Go | 1.21+ | Build do servidor |
| Claude Code CLI | latest | Comunicação com IA |
| HTTPS + Domínio | - | Requisito PWA (veja abaixo) |

### Por que HTTPS e domínio são necessários

As funcionalidades principais do PWA (Service Worker, notificações push, instalação na tela inicial) **só funcionam sobre HTTPS** por razões de segurança. Além disso, é necessário um **domínio** para obter certificados SSL.

Existem várias formas de obter um domínio e certificados SSL, mas este projeto usa o Tailscale. Você também pode usar outros métodos (Cloudflare Tunnel, ngrok, domínio próprio, etc.).

---

## Configuração do Tailscale (método usado neste projeto)

O Tailscale é um serviço VPN que também fornece domínios e certificados SSL gratuitos. É conveniente quando você quer uma configuração HTTPS simples para projetos pessoais.

### 1. Instalar o Tailscale

**PC (computador onde o servidor será executado):**
- Instale a partir de https://tailscale.com/download para o seu SO
- Faça login após a instalação (Google, GitHub, etc.)

**Celular:**
- Instale "Tailscale" na App Store / Play Store
- Faça login com a **mesma conta**

### 2. Verificar domínio

Após instalar o Tailscale, execute no terminal:

```bash
tailscale status
```

Exemplo de saída:
```
100.94.195.110  your-machine    your-email@...
```

Formato do domínio: `your-machine.tail1234.ts.net`

> Você também pode verificar no Console Admin do Tailscale (https://login.tailscale.com/admin)

### 3. Obter certificado SSL

```bash
# Obter certificado (gratuito)
tailscale cert your-machine.tail1234.ts.net
```

Arquivos gerados:
- `your-machine.tail1234.ts.net.crt` (certificado)
- `your-machine.tail1234.ts.net.key` (chave privada)

Copie esses arquivos para a pasta `server/certs/`.

---

## Instalação do Claude Code CLI

```bash
# Instalar via npm
npm install -g @anthropic-ai/claude-code

# Login
claude login
```

> Disponível após fazer login no Claude Code CLI. Não é necessária configuração separada de API key.

---

## Instalação

### 1. Clonar

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. Gerar chaves VAPID

Primeiro, gere as chaves VAPID para notificações push:

```bash
npx web-push generate-vapid-keys
```

Exemplo de saída:
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

Salve essas chaves. Elas serão usadas na configuração abaixo.

### 3. Configuração do servidor

```bash
cd server
go mod download
cp .env.example .env
```

Edite `server/.env`:
```env
# Chaves VAPID (geradas acima)
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# Porta do servidor
SERVER_PORT=8080

# Certificado SSL (caminhos dos arquivos do Tailscale)
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. Configuração do frontend

```bash
cd ../web
npm install
cp .env.example .env
```

Edite `web/.env`:
```env
# Endereço do servidor API (usar domínio do Tailscale)
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# Chave pública VAPID (mesma do server/.env)
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. Compilar e executar

Execute a partir da raiz do projeto (`rico/`).

**Windows:**
```bash
scripts\run-windows.bat
```

**Linux/macOS:**
```bash
chmod +x scripts/run-linux.sh   # apenas na primeira vez
./scripts/run-linux.sh
```

O script lida com: instalação de dependências → build do frontend → build do servidor → inicialização do servidor, nessa ordem.

Log de sucesso:
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### Acessar pelo celular

1. Execute o app Tailscale no celular (verificar conexão)
2. Acesse `https://your-machine.tail1234.ts.net:8080` no navegador

**iOS (Safari):**
3. Toque no botão de compartilhar → Selecione **"Adicionar à Tela de Início"**
4. Instalação do PWA concluída

**Android (Chrome):**
3. Menu (⋮) → Selecione **"Adicionar à tela inicial"** ou **"Instalar app"**
4. Instalação do PWA concluída

> Este projeto foi testado no iOS Safari. Espera-se que funcione no Android, mas não foi testado.

---

## Estrutura do projeto

```
rico/
├── scripts/
│   ├── run-windows.bat     # Build & execução Windows
│   └── run-linux.sh        # Build & execução Linux/macOS
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
│   ├── certs/              # Pasta de certificados SSL
│   └── .env.example
├── context/                # Sistema de contexto
│   ├── personas/           # Configurações de persona
│   │   ├── active.json     # Persona ativa atual
│   │   └── default/        # Persona padrão
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Regras do Agent
└── README.md
```

---

## Configuração

### Resumo de variáveis de ambiente

#### Servidor (`server/.env`)

| Variável | Obrigatório | Descrição |
|----------|-------------|-----------|
| `VAPID_PUBLIC_KEY` | Sim | Chave pública de notificações push |
| `VAPID_PRIVATE_KEY` | Sim | Chave privada de notificações push |
| `SERVER_PORT` | Não | Porta do servidor (padrão: 8080) |
| `SSL_CERT_FILE` | Sim | Caminho do arquivo de certificado SSL |
| `SSL_KEY_FILE` | Sim | Caminho do arquivo de chave SSL |

#### Web (`web/.env`)

| Variável | Obrigatório | Descrição |
|----------|-------------|-----------|
| `VITE_VAPID_PUBLIC_KEY` | Sim | Chave pública de notificações push (mesma do servidor) |
| `VITE_API_BASE` | Sim | Endereço do servidor API (https://...) |

---

## Personalização

### SOUL (Persona de IA)

Você pode personalizar a personalidade, estilo de fala e comportamento da IA editando `context/personas/default/SOUL.md`.

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

A configuração padrão inclui instruções mínimas, permitindo que o Claude responda naturalmente. Você pode adicionar personas detalhadas conforme necessário.

### Idioma (i18n)

Você pode alterar o idioma no menu de configurações do app. Atualmente suporta coreano e inglês.

**Adicionando um novo idioma:**

1. Crie `{código_idioma}.json` na pasta `web/src/locales/` (ex: `ja.json`)
2. Copie a estrutura do `ko.json` e traduza
3. Edite `web/src/lib/i18n.ts`:
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. Atualize a UI de troca de idioma (SessionListScreen.svelte)

---

## Solução de problemas

### Notificações push não funcionam

- Verifique se as chaves VAPID estão configuradas corretamente
- Confirme que as **chaves públicas são idênticas** em `server/.env` e `web/.env`
- Verifique se as permissões de notificação estão permitidas no navegador

### Conexão HTTPS não funciona

- Verifique se o Tailscale está conectado tanto no PC quanto no celular
- Confirme se os caminhos dos arquivos de certificado SSL estão corretos
- Verifique se o certificado não expirou (precisa de renovação a cada 3 meses)
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # re-emitir
  ```

### Erro "Não é possível conectar ao site"

- Verifique se o servidor está em execução
- Confirme se o app Tailscale está conectado no celular
- Verifique se o endereço do domínio está correto

### Sem resposta do Claude Code

- Verifique se está logado com `claude login`
- Verifique os logs do servidor em busca de erros: `server/logs/`
- Teste se o Claude Code CLI funciona corretamente:
  ```bash
  claude "Hello"
  ```

---

## Desenvolvimento

### Desenvolvimento local (HTTP)

Para testar localmente sem certificados SSL:

**Terminal 1 - Servidor:**
```bash
cd server
# Comente SSL_CERT_FILE, SSL_KEY_FILE no .env
go run main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
npm run dev
```

> Nota: Em modo HTTP, notificações push, instalação PWA, etc. não funcionarão.

---

## Nota

Esta é uma ferramenta auto-hospedada que roda no seu desktop pessoal. Não inclui um sistema de autenticação embutido, então por favor gerencie a segurança através do controle de acesso à rede (ex: Tailscale VPN).

---

## Contribuindo

Issues e Pull Requests são bem-vindos.

### Configuração do ambiente de desenvolvimento

1. Fork & Clone deste repositório
2. Siga os passos de [Instalação](#instalação) para configurar o ambiente
3. Consulte a seção de [Desenvolvimento](#desenvolvimento) para executar o servidor de desenvolvimento local

### Diretrizes para PR

- Escreva mensagens de commit em inglês
- Siga o estilo de código existente
- Inclua uma descrição das suas alterações no corpo do PR quando possível

---

## Licença

MIT License - veja [LICENSE](LICENSE)
