// API 설정
// VITE_API_PORT 환경변수로 API 서버 포트를 지정할 수 있습니다.
// 기본값: 8081 (Rico)

const API_PORT = import.meta.env.VITE_API_PORT || '8081';
const API_HOST = window.location.hostname;

export const API_BASE = import.meta.env.VITE_API_BASE || `https://${API_HOST}:${API_PORT}`;
export const WS_URL = `wss://${API_HOST}:${API_PORT}/ws`;
