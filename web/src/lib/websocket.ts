// WebSocket 클라이언트

type MessageHandler = (data: any) => void;

interface WebSocketClient {
  connect: (sessionId?: string) => void;
  disconnect: () => void;
  send: (type: string, payload: any) => void;
  onMessage: (handler: MessageHandler) => void;
  onStatusChange: (handler: (connected: boolean) => void) => void;
  refreshSession: (sessionId: string) => void; // 세션 상태 재질의
  isConnected: () => boolean;
}

export function createWebSocketClient(url: string): WebSocketClient {
  let ws: WebSocket | null = null;
  let messageHandler: MessageHandler | null = null;
  let statusHandler: ((connected: boolean) => void) | null = null;
  let reconnectTimer: number | null = null;
  let currentSessionId: string | undefined = undefined;

  function connect(sessionId?: string) {
    // 세션 ID 저장 (재연결 시에도 사용)
    if (sessionId) {
      currentSessionId = sessionId;
    }

    // 이미 연결 중이거나 연결됨이면 무시
    if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) return;

    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log('WebSocket 연결됨');
      statusHandler?.(true);

      // 연결 메시지 전송 (세션 ID 포함)
      send('connect', { sessionId: currentSessionId });
    };

    ws.onclose = () => {
      console.log('WebSocket 연결 끊김');
      statusHandler?.(false);

      // 3초 후 재연결 시도
      reconnectTimer = window.setTimeout(() => {
        console.log('재연결 시도...');
        connect();
      }, 3000);
    };

    ws.onerror = (error) => {
      console.error('WebSocket 에러:', error);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log('수신:', data);
        messageHandler?.(data);
      } catch (e) {
        console.error('메시지 파싱 실패:', e);
      }
    };
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    ws?.close();
    ws = null;
  }

  function send(type: string, payload: any) {
    if (ws?.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket이 연결되지 않음');
      return;
    }

    const message = {
      type,
      payload,
      timestamp: Date.now(),
    };

    ws.send(JSON.stringify(message));
    console.log('전송:', message);
  }

  function onMessage(handler: MessageHandler) {
    messageHandler = handler;
  }

  function onStatusChange(handler: (connected: boolean) => void) {
    statusHandler = handler;
  }

  // 세션 상태 재질의 (백그라운드 복귀 시 사용)
  function refreshSession(sessionId: string) {
    currentSessionId = sessionId;
    if (ws?.readyState === WebSocket.OPEN) {
      // 이미 연결된 상태면 connect 메시지로 세션 상태 재질의
      send('connect', { sessionId });
      console.log('세션 상태 재질의:', sessionId);
    } else {
      // 연결 안 되어 있으면 재연결
      connect(sessionId);
    }
  }

  function isConnected() {
    return ws?.readyState === WebSocket.OPEN;
  }

  return {
    connect,
    disconnect,
    send,
    onMessage,
    onStatusChange,
    refreshSession,
    isConnected,
  };
}
