import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import fs from 'fs'
import path from 'path'

// SSL 인증서 경로 (환경변수 또는 기본 경로)
const sslKeyPath = process.env.VITE_SSL_KEY_PATH || '../server/certs/server.key';
const sslCertPath = process.env.VITE_SSL_CERT_PATH || '../server/certs/server.crt';

// SSL 파일이 존재하는지 확인
const keyExists = fs.existsSync(path.resolve(__dirname, sslKeyPath));
const certExists = fs.existsSync(path.resolve(__dirname, sslCertPath));

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    // SSL 파일이 있을 때만 HTTPS 활성화
    ...(keyExists && certExists ? {
      https: {
        key: fs.readFileSync(path.resolve(__dirname, sslKeyPath)),
        cert: fs.readFileSync(path.resolve(__dirname, sslCertPath)),
      },
    } : {}),
    host: '0.0.0.0',
    port: 5173,
  },
})
