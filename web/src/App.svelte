<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import LoginScreen from './lib/LoginScreen.svelte';
  import SessionListScreen from './lib/SessionListScreen.svelte';
  import ChatScreen from './lib/ChatScreen.svelte';
  import SoulScreen from './lib/SoulScreen.svelte';
  import SoulListScreen from './lib/SoulListScreen.svelte';
  import SoulEditScreen from './lib/SoulEditScreen.svelte';
  import LogViewerScreen from './lib/LogViewerScreen.svelte';
  import { createWebSocketClient } from './lib/websocket';
  import { API_BASE, WS_URL } from './lib/config';

  type Screen = 'login' | 'sessions' | 'chat' | 'soul' | 'soulList' | 'soulEdit' | 'logs';

  interface ChatMessage {
    id: string;
    text: string;
    isUser: boolean;
    timestamp: number;
  }

  interface Session {
    id: string;
    title: string;
    lastMessage?: string;
    updatedAt: number;
  }

  const STORAGE_KEY = 'rico_current_session';
  const THEME_KEY = 'rico_theme';
  const VAPID_PUBLIC_KEY = import.meta.env.VITE_VAPID_PUBLIC_KEY || '';

  // 테마 관리
  type Theme = 'dark' | 'light';
  let currentTheme: Theme = 'dark';

  function initTheme() {
    const saved = localStorage.getItem(THEME_KEY) as Theme | null;
    if (saved) {
      currentTheme = saved;
    } else {
      // 시스템 설정 따르기
      currentTheme = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
    }
    applyTheme(currentTheme);
  }

  function applyTheme(theme: Theme) {
    const root = document.documentElement;
    root.classList.add('theme-transition');

    if (theme === 'light') {
      root.classList.add('light');
    } else {
      root.classList.remove('light');
    }

    // 애니메이션 후 클래스 제거
    setTimeout(() => {
      root.classList.remove('theme-transition');
    }, 300);
  }

  function toggleTheme() {
    currentTheme = currentTheme === 'dark' ? 'light' : 'dark';
    localStorage.setItem(THEME_KEY, currentTheme);
    applyTheme(currentTheme);
  }

  let currentScreen: Screen = 'sessions'; // 로그인 스킵
  let pushSubscription: PushSubscription | null = null;
  let isLoggedIn = true;
  let currentSessionId: string | null = null;
  let lastResponse: string | null = null;
  let lastSuggestions: string[] = [];
  let lastToolsUsed: string[] = [];
  let lastToolDetails: any[] = [];
  let lastTokenUsage: any = null;
  let isConnected = false;
  let claudeState = 'idle';
  let claudeTask = ''; // 현재 수행 중인 작업 (도구 사용 중 등)

  // 세션별 메시지 저장소 (서버에서 로드)
  let sessionMessages: Record<string, ChatMessage[]> = {};

  // 현재 세션 메시지 (반응성)
  $: currentMessages = currentSessionId ? (sessionMessages[currentSessionId] || []) : [];

  // WebSocket 클라이언트
  const ws = createWebSocketClient(WS_URL);

  let sessions: Session[] = [];

  // Persona 설정
  interface PersonaConfig {
    name: string;
    avatar: string;
    accentColor: string;
    typingMessages: {
      default: string[];
      ko: string[];
      en: string[];
    };
  }

  let personaConfig: PersonaConfig = {
    name: 'Assistant',
    avatar: 'A',
    accentColor: '#4fd1c5',
    typingMessages: {
      default: ['Thinking...', 'Processing...', 'Working on it...'],
      ko: ['생각하는 중...', '처리 중...', '작업 중...'],
      en: ['Thinking...', 'Processing...', 'Working on it...']
    }
  };

  // Persona 설정 로드
  async function loadPersonaConfig() {
    try {
      const res = await fetch(`${API_BASE}/api/persona`);
      if (res.ok) {
        personaConfig = await res.json();
      }
    } catch (err) {
      console.error('Persona 설정 로드 실패:', err);
    }
  }

  // 세션 목록 로드
  async function loadSessions() {
    try {
      const res = await fetch(`${API_BASE}/api/sessions`);
      if (res.ok) {
        sessions = await res.json();
      }
    } catch (err) {
      console.error('세션 목록 로드 실패:', err);
    }
  }

  const READ_COUNT_KEY = 'rico_read_counts';

  // 특정 세션의 메시지 로드
  async function loadSessionMessages(sessionId: string) {
    try {
      const res = await fetch(`${API_BASE}/api/session/${sessionId}`);
      if (res.ok) {
        const session = await res.json();
        sessionMessages = { ...sessionMessages, [sessionId]: session.messages || [] };

        // 저장된 suggestions 복원 (없으면 초기화)
        if (session.lastSuggestions && session.lastSuggestions.length > 0) {
          lastSuggestions = session.lastSuggestions;
        } else {
          lastSuggestions = [];
        }

        // 읽음 처리: Claude 응답 수를 저장 (시스템 메시지 제외)
        const claudeMessageCount = (session.messages || []).filter((m: any) => !m.isUser && !m.isSystem).length;
        if (claudeMessageCount > 0) {
          const readCounts = JSON.parse(localStorage.getItem(READ_COUNT_KEY) || '{}');
          readCounts[sessionId] = claudeMessageCount;
          localStorage.setItem(READ_COUNT_KEY, JSON.stringify(readCounts));
        }
      }
    } catch (err) {
      console.error('세션 메시지 로드 실패:', err);
    }
  }

  // VAPID 키를 Uint8Array로 변환
  function urlBase64ToUint8Array(base64String: string): Uint8Array {
    const padding = '='.repeat((4 - base64String.length % 4) % 4);
    const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
      outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
  }

  // 푸시 알림 구독
  async function subscribeToPush() {
    try {
      // 지원 여부 체크
      if (!('serviceWorker' in navigator)) {
        console.log('Service Worker 미지원');
        return;
      }
      if (!('PushManager' in window)) {
        console.log('Push API 미지원');
        return;
      }
      if (!('Notification' in window)) {
        console.log('Notification API 미지원');
        return;
      }

      // 서비스 워커 등록
      const registration = await navigator.serviceWorker.register('/sw.js', {
        scope: '/'
      });
      await navigator.serviceWorker.ready;
      console.log('Service Worker 등록 완료');

      // 알림 권한 요청
      const permission = await Notification.requestPermission();
      console.log('알림 권한 결과:', permission);
      if (permission !== 'granted') {
        console.log('알림 권한 거부됨:', permission);
        return;
      }

      // 푸시 구독
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY)
      });

      pushSubscription = subscription;
      console.log('푸시 구독 완료:', JSON.stringify(subscription));

      // 서버에 구독 정보 전송
      await fetch(`${API_BASE}/api/push/subscribe`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(subscription)
      });

      console.log('푸시 알림 설정 완료');
      console.log('푸시 구독 정보 서버 전송 완료');
    } catch (err: any) {
      // iOS PWA에서 푸시가 불안정할 수 있음 - 조용히 실패
      console.error('푸시 구독 실패:', err);
    }
  }

  onMount(async () => {
    // 테마 초기화
    initTheme();

    ws.onStatusChange((connected) => {
      isConnected = connected;
    });

    ws.onMessage((data) => {
      switch (data.type) {
        case 'status':
          claudeState = data.payload.state;
          claudeTask = data.payload.task || '';
          break;
        case 'response':
          if (data.payload.text) {
            lastResponse = data.payload.text;
            lastSuggestions = data.payload.suggestions || [];
            lastToolsUsed = data.payload.toolsUsed || [];
            lastToolDetails = data.payload.toolDetails || [];
            lastTokenUsage = data.payload.tokenUsage || null;
            setTimeout(() => { lastResponse = null; lastSuggestions = []; lastToolsUsed = []; lastToolDetails = []; }, 100);

            // 응답 받으면 읽음 처리 (현재 세션의 읽음 카운트 +1)
            if (currentSessionId) {
              const readCounts = JSON.parse(localStorage.getItem(READ_COUNT_KEY) || '{}');
              readCounts[currentSessionId] = (readCounts[currentSessionId] || 0) + 1;
              localStorage.setItem(READ_COUNT_KEY, JSON.stringify(readCounts));
            }
          }
          break;
        case 'error':
          console.error('서버 에러:', data.payload);
          break;
      }
    });

    // 저장된 세션 ID 확인
    const savedSessionId = localStorage.getItem(STORAGE_KEY);

    // WebSocket 연결 (세션 ID 포함)
    ws.connect(savedSessionId || undefined);
    await loadSessions();
    await loadPersonaConfig();

    // 푸시 알림 구독 (서비스 워커 지원 시)
    if ('serviceWorker' in navigator && 'PushManager' in window) {
      subscribeToPush();
    }

    // 저장된 세션 ID가 있으면 자동으로 해당 세션으로 이동
    if (savedSessionId) {
      await loadSessionMessages(savedSessionId);
      currentSessionId = savedSessionId;
      currentScreen = 'chat';
    }

    // 앱 전환 후 돌아왔을 때 메시지 다시 로드 + WebSocket 재연결/세션 상태 재질의
    document.addEventListener('visibilitychange', async () => {
      if (document.visibilityState === 'visible') {
        console.log('앱 포커스 복귀 - 리로드');

        // WebSocket 연결 상태 확인 및 세션 상태 재질의
        if (currentSessionId) {
          // refreshSession: 연결되어 있으면 세션 상태 재질의, 아니면 재연결
          ws.refreshSession(currentSessionId);
        } else if (!ws.isConnected()) {
          ws.connect();
        }

        // 세션 목록 화면이면 세션 목록 새로고침
        if (currentScreen === 'sessions') {
          await loadSessions();
        }
        // 채팅 화면이면 메시지 다시 로드
        if (currentSessionId && currentScreen === 'chat') {
          await loadSessionMessages(currentSessionId);
        }
      }
    });
  });

  onDestroy(() => {
    ws.disconnect();
  });

  function handleLogin(email: string, password: string) {
    console.log('Login:', email);
    isLoggedIn = true;
    loadSessions();
    currentScreen = sessions.length === 0 ? 'chat' : 'sessions';
  }


  async function handleSelectSession(sessionId: string) {
    currentSessionId = sessionId;
    localStorage.setItem(STORAGE_KEY, sessionId); // 세션 ID 저장
    await loadSessionMessages(sessionId);
    currentScreen = 'chat';
  }

  function handleNewSession() {
    // 새 세션 ID 생성
    const newId = Date.now().toString(36) + Math.random().toString(36).substr(2);
    currentSessionId = newId;
    localStorage.setItem(STORAGE_KEY, newId); // 세션 ID 저장
    sessionMessages = { [newId]: [] };
    lastSuggestions = []; // 새 세션이므로 suggestions 초기화
    currentScreen = 'chat';
  }

  async function handleRefresh() {
    await loadSessions();
  }

  async function handleDeleteSession(sessionId: string) {
    try {
      await fetch(`${API_BASE}/api/session/${sessionId}`, { method: 'DELETE' });
      await loadSessions();
    } catch (err) {
      console.error('세션 삭제 실패:', err);
    }
  }

  async function handleBackToSessions() {
    localStorage.removeItem(STORAGE_KEY); // 세션 목록으로 가면 저장된 세션 삭제
    currentScreen = 'sessions';
    await loadSessions(); // 세션 목록 새로고침
  }

  function handleSendMessage(text: string) {
    console.log('Send message:', text, 'sessionId:', currentSessionId);
    ws.send('message', { text, sessionId: currentSessionId });
  }

  function handleCancel() {
    console.log('Cancel request');
    ws.send('action', { action: 'cancel' });
  }

  function updateMessages(newMessages: ChatMessage[]) {
    if (!currentSessionId) return;
    sessionMessages = { ...sessionMessages, [currentSessionId]: newMessages };
  }

  function handleSoulSettings() {
    currentScreen = 'soulList';
  }

  function handleBackFromSoul() {
    currentScreen = 'soulList';
  }

  function handleBackFromSoulList() {
    currentScreen = 'sessions';
  }

  let selectedSoulFileName = '';
  let selectedSoulName = '';

  function handleSelectSoul(fileName: string, name: string) {
    selectedSoulFileName = fileName;
    selectedSoulName = name;
    currentScreen = 'soulEdit';
  }

  function handleCreateNewSoul() {
    currentScreen = 'soul';
  }

  function handleBackFromSoulEdit() {
    currentScreen = 'soulList';
  }

  async function handleSoulApplied() {
    await loadPersonaConfig();
    currentScreen = 'soulList';
  }

  function handleLogs() {
    currentScreen = 'logs';
  }

  function handleBackFromLogs() {
    currentScreen = 'chat';
  }
</script>

{#if currentScreen === 'login'}
  <LoginScreen onLogin={handleLogin} />
{:else if currentScreen === 'sessions'}
  <SessionListScreen
    {sessions}
    onSelectSession={handleSelectSession}
    onNewSession={handleNewSession}
    onRefresh={handleRefresh}
    onDeleteSession={handleDeleteSession}
    onEnablePush={subscribeToPush}
    onSoulSettings={handleSoulSettings}
    onToggleTheme={toggleTheme}
    theme={currentTheme}
  />
{:else if currentScreen === 'soulList'}
  <SoulListScreen
    onBack={handleBackFromSoulList}
    onSelectSoul={handleSelectSoul}
    onCreateNew={handleCreateNewSoul}
  />
{:else if currentScreen === 'soulEdit'}
  <SoulEditScreen
    fileName={selectedSoulFileName}
    soulName={selectedSoulName}
    onBack={handleBackFromSoulEdit}
    onApply={handleSoulApplied}
  />
{:else if currentScreen === 'soul'}
  <SoulScreen onBack={handleBackFromSoul} />
{:else if currentScreen === 'logs'}
  <LogViewerScreen onBack={handleBackFromLogs} />
{:else}
  <ChatScreen
    {lastResponse}
    {lastSuggestions}
    {lastToolsUsed}
    {lastToolDetails}
    {lastTokenUsage}
    {isConnected}
    {claudeState}
    {claudeTask}
    sessionId={currentSessionId}
    initialMessages={currentMessages}
    onMessagesChange={updateMessages}
    onSendMessage={handleSendMessage}
    onCancel={handleCancel}
    onBack={handleBackToSessions}
    onLogs={handleLogs}
    {personaConfig}
  />
{/if}
