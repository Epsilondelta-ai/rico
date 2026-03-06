# Rico

**Remote Claude Code Operator**

[English](README-en.md) | [한국어](../README.md) | [简体中文](README-zh.md) | 日本語 | [Español](README-es.md) | [Português (BR)](README-pt-br.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Deutsch](README-de.md)

### Q. これは何？

**A. 寝転がりながら Claude Code で作業したくて作りました。**

モバイルで Claude Code を使えるようにする PWA クライアントです。Go ブリッジサーバーを通じて Claude Code CLI と通信し、リアルタイム WebSocket 接続で高速なレスポンスを提供します。

![Rico Screenshots](../screenshots/rico-mockup.png)

## 機能

- **Mobile-First PWA**：ホーム画面にインストールしてネイティブアプリのように使用
- **リアルタイムチャット**：WebSocket ベースのリアルタイム会話
- **プッシュ通知**：新しいメッセージ到着時にプッシュ通知
- **ファイルブラウザ**：サーバーファイルシステムの閲覧とファイル表示
- **セッション管理**：会話セッションの保存と管理
- **SOUL システム**：カスタマイズ可能な AI ペルソナ
- **Skills**：スラッシュコマンドで拡張機能を使用
- **i18n**：多言語対応（韓国語/英語）

## 技術スタック

| コンポーネント | 技術 |
|--------------|------|
| フロントエンド | Svelte 5, Vite 7, TypeScript, Tailwind CSS |
| バックエンド | Go (gorilla/websocket) |
| 通信 | WebSocket + REST API |
| デプロイ | PWA (Progressive Web App) |

---

## 要件

### 前提条件

| 要件 | バージョン | 用途 |
|------|----------|------|
| Node.js | 18+ | フロントエンドビルド |
| Go | 1.21+ | サーバービルド |
| Claude Code CLI | latest | AI 通信 |
| HTTPS + ドメイン | - | PWA 必須要件（下記参照） |

### HTTPS とドメインが必要な理由

PWA のコア機能（Service Worker、プッシュ通知、ホーム画面インストール）はセキュリティ上の理由から **HTTPS でのみ動作**します。また、SSL 証明書の取得には**ドメイン**が必要です。

ドメインと SSL 証明書を取得する方法はいくつかありますが、このプロジェクトでは Tailscale を使用しています。他の方法（Cloudflare Tunnel、ngrok、独自ドメインなど）を使用することもできます。

---

## Tailscale 設定（このプロジェクトで使用した方法）

Tailscale は VPN サービスで、無料でドメインと SSL 証明書も提供しています。個人プロジェクトで HTTPS の設定を簡単に済ませたい時に便利です。

### 1. Tailscale のインストール

**PC（サーバーを実行するコンピューター）：**
- https://tailscale.com/download から OS に合ったバージョンをインストール
- インストール後にログイン（Google、GitHub など）

**モバイル：**
- App Store / Play Store から "Tailscale" をインストール
- **同じアカウント**でログイン

### 2. ドメインの確認

Tailscale インストール後、ターミナルで：

```bash
tailscale status
```

出力例：
```
100.94.195.110  your-machine    your-email@...
```

ドメイン形式：`your-machine.tail1234.ts.net`

> Tailscale Admin Console（https://login.tailscale.com/admin）でも確認可能

### 3. SSL 証明書の取得

```bash
# 証明書取得（無料）
tailscale cert your-machine.tail1234.ts.net
```

生成されるファイル：
- `your-machine.tail1234.ts.net.crt`（証明書）
- `your-machine.tail1234.ts.net.key`（秘密鍵）

これらのファイルを `server/certs/` フォルダにコピーしてください。

---

## Claude Code CLI のインストール

```bash
# npm でインストール
npm install -g @anthropic-ai/claude-code

# ログイン
claude login
```

> Claude Code CLI にログイン後、使用可能です。別途 API キーの設定は不要です。

---

## インストール

### 1. クローン

```bash
git clone https://github.com/Epsilondelta-ai/rico.git
cd rico
```

### 2. VAPID キーの生成

まず、プッシュ通知用の VAPID キーを生成します：

```bash
npx web-push generate-vapid-keys
```

出力例：
```
=======================================

Public Key:
BNlx...your_public_key...

Private Key:
abc1...your_private_key...

=======================================
```

これらのキーをメモしてください。以下の設定で使用します。

### 3. サーバー設定

```bash
cd server
go mod download
cp .env.example .env
```

`server/.env` を編集：
```env
# VAPID キー（上記で生成したキー）
VAPID_PUBLIC_KEY=BNlx...your_public_key...
VAPID_PRIVATE_KEY=abc1...your_private_key...

# サーバーポート
SERVER_PORT=8080

# SSL 証明書（Tailscale で発行されたファイルのパス）
SSL_CERT_FILE=./certs/your-machine.tail1234.ts.net.crt
SSL_KEY_FILE=./certs/your-machine.tail1234.ts.net.key
```

### 4. フロントエンド設定

```bash
cd ../web
npm install
cp .env.example .env
```

`web/.env` を編集：
```env
# API サーバーアドレス（Tailscale ドメインを使用）
VITE_API_BASE=https://your-machine.tail1234.ts.net:8080

# VAPID 公開鍵（server/.env と同じ公開鍵）
VITE_VAPID_PUBLIC_KEY=BNlx...your_public_key...
```

### 5. ビルドと実行

プロジェクトルート（`rico/`）で実行します。

**Windows：**
```bash
scripts\run-windows.bat
```

**Linux/macOS：**
```bash
chmod +x scripts/run-linux.sh   # 初回のみ
./scripts/run-linux.sh
```

スクリプトが順番に処理します：依存関係インストール → フロントエンドビルド → サーバービルド → サーバー起動。

成功時のログ：
```
Rico 설정 로드 완료:
  - RICO_BASE_PATH: /path/to/rico
  - SERVER_PORT: 8080
Rico 브릿지 서버 시작 (HTTPS): :8080
```

### モバイルからのアクセス

1. モバイルで Tailscale アプリを実行（接続確認）
2. ブラウザで `https://your-machine.tail1234.ts.net:8080` にアクセス

**iOS (Safari)：**
3. 下部の共有ボタンをタップ → **「ホーム画面に追加」**を選択
4. PWA インストール完了

**Android (Chrome)：**
3. メニュー(⋮) → **「ホーム画面に追加」**または**「アプリをインストール」**を選択
4. PWA インストール完了

> このプロジェクトは iOS Safari でテスト済みです。Android でも動作するはずですが、テストされていません。

---

## プロジェクト構成

```
rico/
├── scripts/
│   ├── run-windows.bat     # Windows ビルド & 実行
│   └── run-linux.sh        # Linux/macOS ビルド & 実行
├── web/                    # Svelte PWA フロントエンド
│   ├── src/
│   │   ├── App.svelte
│   │   └── lib/
│   │       ├── ChatScreen.svelte
│   │       └── websocket.ts
│   ├── .env.example
│   └── package.json
├── server/                 # Go ブリッジサーバー
│   ├── main.go
│   ├── certs/              # SSL 証明書フォルダ
│   └── .env.example
├── context/                # コンテキストシステム
│   ├── personas/           # ペルソナ設定
│   │   ├── active.json     # 現在アクティブなペルソナ
│   │   └── default/        # デフォルトペルソナ
│   │       ├── SOUL.md
│   │       └── config.json
│   └── ...
├── CLAUDE.md               # Agent ルール
└── README.md
```

---

## 設定

### 環境変数まとめ

#### サーバー (`server/.env`)

| 変数 | 必須 | 説明 |
|------|------|------|
| `VAPID_PUBLIC_KEY` | はい | プッシュ通知公開鍵 |
| `VAPID_PRIVATE_KEY` | はい | プッシュ通知秘密鍵 |
| `SERVER_PORT` | いいえ | サーバーポート（デフォルト：8080） |
| `SSL_CERT_FILE` | はい | SSL 証明書ファイルパス |
| `SSL_KEY_FILE` | はい | SSL キーファイルパス |

#### Web (`web/.env`)

| 変数 | 必須 | 説明 |
|------|------|------|
| `VITE_VAPID_PUBLIC_KEY` | はい | プッシュ通知公開鍵（サーバーと同じ） |
| `VITE_API_BASE` | はい | API サーバーアドレス (https://...) |

---

## カスタマイズ

### SOUL（AI ペルソナ）

`context/personas/default/SOUL.md` を編集して、AI の性格、話し方、行動方式をカスタマイズできます。

```markdown
# SOUL

You are a Claude Code agent. Respond naturally.
```

デフォルト設定は最小限の指示のみ含まれており、Claude が自然に応答します。必要に応じて詳細なペルソナを追加できます。

### 言語 (i18n)

アプリ内の設定メニューで言語を変更できます。現在、韓国語と英語をサポートしています。

**新しい言語の追加方法：**

1. `web/src/locales/` フォルダに `{言語コード}.json` を作成（例：`ja.json`）
2. `ko.json` の構造をコピーして翻訳
3. `web/src/lib/i18n.ts` を編集：
   ```typescript
   import ja from '../locales/ja.json';
   addMessages('ja', ja);
   ```
4. 言語切替 UI を更新（SessionListScreen.svelte）

---

## トラブルシューティング

### プッシュ通知が来ない

- VAPID キーが正しく設定されているか確認
- `server/.env` と `web/.env` の**公開鍵が同一**か確認
- ブラウザで通知権限が許可されているか確認

### HTTPS 接続ができない

- Tailscale が PC とモバイルの両方で接続されているか確認
- SSL 証明書ファイルのパスが正しいか確認
- 証明書が期限切れでないか確認（3ヶ月ごとに更新が必要）
  ```bash
  tailscale cert your-machine.tail1234.ts.net  # 再発行
  ```

### 「サイトに接続できません」エラー

- サーバーが実行中か確認
- モバイルで Tailscale アプリが接続されているか確認
- ドメインアドレスが正しいか確認

### Claude Code の応答がない

- `claude login` でログインしているか確認
- サーバーログでエラーを確認：`server/logs/`
- Claude Code CLI が正常に動作するかテスト：
  ```bash
  claude "Hello"
  ```

---

## 開発

### ローカル開発（HTTP）

SSL 証明書なしでローカルテストする場合：

**ターミナル 1 - サーバー：**
```bash
cd server
# .env で SSL_CERT_FILE、SSL_KEY_FILE をコメントアウト
go run main.go
```

**ターミナル 2 - フロントエンド：**
```bash
cd web
npm run dev
```

> 注意：HTTP モードでは、プッシュ通知、PWA インストールなどは動作しません。

---

## 注意事項

これは個人のデスクトップで実行するセルフホスティングツールです。組み込みの認証システムは含まれていないため、ネットワークアクセス制御（Tailscale VPN など）でセキュリティを管理してください。

---

## コントリビューション

Issues と Pull Requests を歓迎します。

### 開発環境のセットアップ

1. このリポジトリを Fork & Clone
2. [インストール](#インストール) の手順に従って環境を設定
3. [開発](#開発) セクションを参考にローカル開発サーバーを実行

### PR ガイドライン

- コミットメッセージは英語で記述
- 既存のコードスタイルに従ってください
- 可能であれば PR 本文に変更内容の説明を含めてください

---

## ライセンス

MIT License - [LICENSE](LICENSE) を参照
