<script lang="ts">
  import { onMount, afterUpdate } from 'svelte';
  import { marked } from 'marked';
  import { _ } from 'svelte-i18n';
  import { API_BASE } from './config';

  // 도구 상세 정보 인터페이스
  interface ToolDetail {
    name: string;      // "Read", "Edit", "Grep" 등
    file?: string;     // 파일 경로
    offset?: number;   // 시작 줄 (Read)
    limit?: number;    // 줄 수 (Read)
    line?: number;     // 수정 줄 번호 (Edit)
    oldString?: string;  // 변경 전 문자열 (Edit)
    newString?: string;  // 변경 후 문자열 (Edit)
    pattern?: string;    // 검색 패턴 (Grep, Glob)
    command?: string;    // 실행 명령어 (Bash)
    suggestions?: string[]; // 제안 (StructuredOutput)
  }

  // 토큰 사용량 정보
  interface TokenUsage {
    inputTokens: number;
    outputTokens: number;
    cacheCreationInputTokens?: number;
    cacheReadInputTokens?: number;
    totalCostUsd?: number;
  }

  interface ChatMessage {
    id: string;
    text: string;
    isUser: boolean;
    timestamp: number;
    imageUrl?: string; // 첨부 이미지 URL (유저)
    imageUrls?: string[]; // 이미지 URL들 (Claude 응답)
    isSystem?: boolean; // 시스템 메시지 (페르소나 충전 등) - 세션 복원 시 필터링
    toolsUsed?: string[]; // 사용된 도구 목록 (예: "Read: main.go") - 호환용
    toolDetails?: ToolDetail[]; // 도구 상세 정보
    outputTokens?: number; // 출력 토큰 수
  }

  export let lastResponse: string | null = null;
  export let lastSuggestions: string[] = [];
  export let lastToolsUsed: string[] = [];
  export let lastToolDetails: ToolDetail[] = [];
  export let lastTokenUsage: TokenUsage | null = null;
  export let isConnected: boolean = false;
  export let claudeState: string = 'idle';
  export let claudeTask: string = ''; // 현재 수행 중인 작업 상태
  export let initialPendingTools: string[] = []; // 재연결 시 복원할 도구 목록
  export let sessionId: string | null = null;
  export let initialMessages: ChatMessage[] = [];
  export let onMessagesChange: (messages: ChatMessage[]) => void = () => {};
  export let onSendMessage: (text: string) => void = () => {};
  export let onCancel: () => void = () => {};
  export let onBack: () => void = () => {};
  export let onLogs: () => void = () => {};

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

  export let personaConfig: PersonaConfig = {
    name: 'Assistant',
    avatar: 'A',
    accentColor: '#4fd1c5',
    typingMessages: {
      default: ['Thinking...', 'Processing...', 'Working on it...'],
      ko: ['생각하는 중...', '처리 중...', '작업 중...'],
      en: ['Thinking...', 'Processing...', 'Working on it...']
    }
  };

  // 편의 변수
  $: activeSoulName = personaConfig.name;
  $: avatarChar = personaConfig.avatar;

  // 현재 표시할 suggestions (응답 완료 후 표시, 사용자 입력 시 숨김)
  let currentSuggestions: string[] = [];

  // lastSuggestions가 변경되면 currentSuggestions 업데이트
  $: if (lastSuggestions.length > 0) {
    currentSuggestions = lastSuggestions;
  }

  // 로컬 메시지 상태
  let localMessages: ChatMessage[] = [];
  let lastInitialMessagesLength = 0;

  // initialMessages가 변경될 때 localMessages 업데이트 (시스템 메시지 필터링)
  $: if (initialMessages.length !== lastInitialMessagesLength) {
    localMessages = processMessages([...initialMessages].filter(m => !m.isSystem));
    lastInitialMessagesLength = initialMessages.length;
  }

  onMount(async () => {
    // 메시지 불러올 때 이미지 경로 파싱 + 시스템 메시지 필터링
    localMessages = processMessages([...initialMessages].filter(m => !m.isSystem));
    lastInitialMessagesLength = initialMessages.length;

    // 퀵 경로 로드 (서버에서 동적으로)
    const quickPaths = await loadQuickPaths();
    if (quickPaths.length > 0 && recentPaths.length === 0) {
      recentPaths = quickPaths;
      // 첫 번째 퀵 경로를 currentPath로 설정
      if (!currentPath && quickPaths[0]) {
        currentPath = quickPaths[0].path;
      }
    }
  });

  // 스크롤이 맨 아래인지 확인 (여유값 50px)
  function isScrolledToBottom(): boolean {
    if (!messageListEl) return true;
    const { scrollTop, scrollHeight, clientHeight } = messageListEl;
    return scrollHeight - scrollTop - clientHeight < 50;
  }

  // 스크롤 이벤트 핸들러
  let dividerRemoveTimer: ReturnType<typeof setTimeout> | null = null;

  function handleScroll() {
    // 맨 아래로 스크롤하면 2초 후 구분선 제거
    if (isScrolledToBottom() && newMessageDividerIndex !== null) {
      if (!dividerRemoveTimer) {
        dividerRemoveTimer = setTimeout(() => {
          newMessageDividerIndex = null;
          dividerRemoveTimer = null;
        }, 2000);
      }
    } else {
      // 다시 위로 스크롤하면 타이머 취소
      if (dividerRemoveTimer) {
        clearTimeout(dividerRemoveTimer);
        dividerRemoveTimer = null;
      }
    }
  }

  let inputText = '';
  let messageListEl: HTMLDivElement;
  let textareaEl: HTMLTextAreaElement;
  let copiedId: string | null = null;
  let prevMessageCount = 0;
  let isTyping = false;
  let typingMessage = '';
  let lastProcessedResponse: string | null = null; // 마지막 처리한 응답 (중복 방지)

  // 실시간 도구 사용 표시 (작업 중일 때)
  let pendingTools: string[] = [];
  let pendingToolsSet = new Set<string>();

  // initialPendingTools가 오면 복원 (재연결 시)
  // initialPendingTools가 오면 복원 (재연결 시, working 상태일 때만)
  $: if (initialPendingTools.length > 0 && pendingTools.length === 0 && claudeState === 'working') {
    pendingTools = [...initialPendingTools];
    pendingToolsSet = new Set(initialPendingTools);
  }

  // 새 메시지 구분선 (재접속 시 마지막으로 본 메시지 인덱스)
  let newMessageDividerIndex: number | null = null;

  // + 메뉴
  let showPlusMenu = false;

  // 파일 브라우저
  let showFileBrowser = false;
  let currentPath = ''; // 서버에서 동적으로 받아옴
  let folderContents: { name: string; isDir: boolean }[] = [];
  let workingPath: string | null = null; // 선택된 작업 폴더 (메시지에 숨겨서 전송)
  let isLoadingFolder = false;

  // 파일 뷰어
  let showFileViewer = false;
  let viewingFile: { name: string; ext: string; content: string } | null = null;
  let isLoadingFile = false;

  // Skills
  let showSkillsModal = false;
  let skills: { name: string; path: string; description: string }[] = [];
  let isLoadingSkills = false;

  // 이미지 업로드
  let fileInputEl: HTMLInputElement;
  let uploadedImagePath: string | null = null;
  let isUploading = false;

  // 메시지 액션 시트
  let showMessageActions = false;
  let selectedMessage: ChatMessage | null = null;
  let longPressTimer: ReturnType<typeof setTimeout> | null = null;
  let pressingMessageId: string | null = null; // 현재 누르고 있는 메시지 ID
  let pressingActionButton: string | null = null; // 액션 버튼 누르는 중

  // 이미지 뷰어
  let showImageViewer = false;
  let viewingImageUrl: string | null = null;

  // 도구 인스펙터 (펼침/접힘 상태 및 파일 내용 캐시)
  let expandedToolKey: string | null = null; // "messageId:toolIndex" 형식
  let toolContentCache: Record<string, { content: string; loading: boolean; error?: string }> = {};

  // 설정 메뉴 바텀시트
  let showSettingsMenu = false;
  let isRestarting = false;

  // 최근 이동 경로 (localStorage에서 로드)
  const RECENT_PATHS_KEY = 'rico_recent_paths';
  const MAX_RECENT_PATHS = 4;

  function loadRecentPaths(): { name: string; path: string }[] {
    try {
      const saved = localStorage.getItem(RECENT_PATHS_KEY);
      if (saved) {
        return JSON.parse(saved);
      }
    } catch (e) {
      console.error('recent path load failed:', e);
    }
    // 기본값은 빈 배열 (서버에서 퀵 경로 로드)
    return [];
  }

  // 서버에서 퀵 경로 (홈, 바탕화면 등) 가져오기
  async function loadQuickPaths(): Promise<{ name: string; path: string }[]> {
    try {
      const res = await fetch(`${API_BASE}/api/quick-paths`);
      if (res.ok) {
        return await res.json();
      }
    } catch (e) {
      console.error('quick paths load failed:', e);
    }
    return [];
  }

  function saveRecentPath(path: string) {
    // 경로에서 폴더명 추출
    const parts = path.split('\\');
    const name = parts[parts.length - 1] || parts[parts.length - 2] || path;

    // 기존 목록에서 같은 경로 제거
    let paths = recentPaths.filter(p => p.path !== path);

    // 맨 앞에 추가
    paths.unshift({ name, path });

    // 최대 개수 유지
    paths = paths.slice(0, MAX_RECENT_PATHS);

    // 저장
    recentPaths = paths;
    localStorage.setItem(RECENT_PATHS_KEY, JSON.stringify(paths));
  }

  let recentPaths = loadRecentPaths();

  // API 서버 주소 (config에서 가져옴)
  const API_HOST = API_BASE;

  async function openFileBrowser() {
    showFileBrowser = true;
    await loadFolder(currentPath);
  }

  async function loadFolder(path: string) {
    isLoadingFolder = true;
    try {
      const res = await fetch(`${API_HOST}/api/files?path=${encodeURIComponent(path)}`);
      if (res.ok) {
        const data = await res.json();
        currentPath = data.path;
        folderContents = data.files;
      }
    } catch (err) {
      console.error($_<string>('error.folder_load'), err);
    }
    isLoadingFolder = false;
  }

  function navigateToFolder(name: string) {
    const newPath = currentPath + '\\' + name;
    loadFolder(newPath);
  }

  function goUp() {
    const parts = currentPath.split('\\');
    if (parts.length > 1) {
      parts.pop();
      loadFolder(parts.join('\\'));
    }
  }

  function selectPath(path: string) {
    workingPath = path;
    saveRecentPath(path); // 최근 경로에 저장
    showFileBrowser = false;
  }

  function clearWorkingPath() {
    workingPath = null;
  }

  function selectQuickPath(path: string) {
    loadFolder(path);
  }

  // 이미지 선택 트리거
  function triggerImageUpload() {
    fileInputEl?.click();
  }

  // 이미지 업로드 처리
  async function handleImageSelect(event: Event) {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    isUploading = true;
    const formData = new FormData();
    formData.append('image', file);

    try {
      const uploadSessionId = sessionId || 'default';
      const uploadUrl = `${API_HOST}/api/upload?sessionId=${encodeURIComponent(uploadSessionId)}`;
      console.log('[업로드] 시작:', uploadUrl, file.name, file.size, 'bytes');

      // AbortController로 타임아웃 설정 (30초)
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 30000);

      const res = await fetch(uploadUrl, {
        method: 'POST',
        body: formData,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);
      console.log('[업로드] 응답 상태:', res.status);

      if (res.ok) {
        const data = await res.json();
        console.log('[업로드] 성공:', data.path);
        uploadedImagePath = data.path;
      } else {
        const errorText = await res.text();
        console.error('[업로드] 실패:', res.status, errorText);
      }
    } catch (err: any) {
      if (err.name === 'AbortError') {
        console.error('[업로드] 타임아웃 (30초 초과)');
      } else {
        console.error('[업로드] 에러:', err);
      }
    } finally {
      isUploading = false;
      // input 초기화 (같은 파일 다시 선택 가능하도록)
      target.value = '';
    }
  }

  // 첨부된 이미지 제거
  function clearUploadedImage() {
    uploadedImagePath = null;
  }

  // 메시지 롱프레스 핸들러
  function handleMessageTouchStart(event: TouchEvent, message: ChatMessage) {
    // 코드블록 내부에서 터치 시작하면 롱프레스 무시 (스크롤 허용)
    const target = event.target as HTMLElement;
    if (target.closest('pre') || target.closest('code')) {
      return;
    }

    pressingMessageId = message.id; // 누르고 있는 메시지 표시
    longPressTimer = setTimeout(() => {
      selectedMessage = message;
      showMessageActions = true;
      pressingMessageId = null;
      // 햅틱 피드백 (지원되는 경우)
      if (navigator.vibrate) {
        navigator.vibrate(50);
      }
    }, 500); // 500ms 길게 누르기
  }

  function handleMessageTouchEnd() {
    pressingMessageId = null; // 누르기 해제
    if (longPressTimer) {
      clearTimeout(longPressTimer);
      longPressTimer = null;
    }
  }

  // 메시지 복사 (액션 시트용)
  async function copySelectedMessage() {
    if (!selectedMessage) return;
    await copyMessage(selectedMessage.text, selectedMessage.id);
    showMessageActions = false;
    selectedMessage = null;
  }

  // 메시지 삭제
  function deleteSelectedMessage() {
    if (!selectedMessage) return;
    localMessages = localMessages.filter(m => m.id !== selectedMessage!.id);
    onMessagesChange(localMessages);
    showMessageActions = false;
    selectedMessage = null;
  }

  // 액션 시트 닫기
  function closeMessageActions() {
    showMessageActions = false;
    selectedMessage = null;
  }

  // 이미지 뷰어 열기
  function openImageViewer(url: string) {
    viewingImageUrl = url;
    showImageViewer = true;
  }

  // 이미지 뷰어 닫기
  function closeImageViewer() {
    showImageViewer = false;
    viewingImageUrl = null;
  }

  // ============ 도구 인스펙터 함수들 ============

  // 도구 상세 정보 토글
  function toggleToolDetail(messageId: string, toolIndex: number) {
    const key = `${messageId}:${toolIndex}`;
    if (expandedToolKey === key) {
      expandedToolKey = null;
    } else {
      expandedToolKey = key;
    }
  }

  // 도구 파일 내용 조회 (토글 방식)
  async function loadToolContent(cacheKey: string, detail: ToolDetail) {
    // 이미 로드되어 있으면 토글 (숨기기)
    if (toolContentCache[cacheKey]) {
      delete toolContentCache[cacheKey];
      toolContentCache = { ...toolContentCache };
      return;
    }

    toolContentCache[cacheKey] = { content: '', loading: true };
    toolContentCache = { ...toolContentCache };

    try {
      let url = `${API_HOST}/api/file?path=${encodeURIComponent(detail.file || '')}`;
      if (detail.offset) url += `&offset=${detail.offset}`;
      if (detail.limit) url += `&limit=${detail.limit}`;

      const res = await fetch(url);
      if (res.ok) {
        const data = await res.json();
        // 줄 번호 추가 (offset이 있으면 해당 줄부터 시작)
        const startLine = detail.offset || 1;
        const contentWithLineNumbers = addLineNumbers(data.content, startLine);
        toolContentCache[cacheKey] = { content: contentWithLineNumbers, loading: false };
      } else {
        toolContentCache[cacheKey] = { content: '', loading: false, error: '파일을 읽을 수 없습니다' };
      }
    } catch (err) {
      toolContentCache[cacheKey] = { content: '', loading: false, error: '파일을 읽을 수 없습니다' };
    }
    toolContentCache = { ...toolContentCache };
  }

  // 줄 번호 추가 함수
  function addLineNumbers(content: string, startLine: number = 1): string {
    const lines = content.split('\n');
    const maxLineNum = startLine + lines.length - 1;
    const padWidth = String(maxLineNum).length;

    return lines.map((line, index) => {
      const lineNum = String(startLine + index).padStart(padWidth, ' ');
      return `${lineNum} │ ${line}`;
    }).join('\n');
  }

  // 도구 상세 정보에서 범위 텍스트 생성
  function getToolRangeText(detail: ToolDetail): string {
    if (detail.name === 'Read' && detail.offset && detail.limit) {
      return `${detail.offset}-${detail.offset + detail.limit - 1}줄`;
    } else if (detail.name === 'Read' && detail.limit) {
      return `1-${detail.limit}줄`;
    } else if (detail.line) {
      return `${detail.line}줄`;
    }
    return '';
  }

  // 도구 태그 라벨 생성
  function getToolLabel(detail: ToolDetail): string {
    const name = detail.name;

    // 파일 경로가 있는 도구
    if (detail.file) {
      const fileName = detail.file.split(/[/\\]/).pop() || detail.file;
      return `${name}: ${fileName}`;
    }

    // Bash: 명령어 표시 (첫 30자)
    if (name === 'Bash' && detail.command) {
      const cmd = detail.command.length > 30
        ? detail.command.substring(0, 30) + '...'
        : detail.command;
      return `${name}: ${cmd}`;
    }

    // Grep/Glob: 패턴 표시
    if ((name === 'Grep' || name === 'Glob') && detail.pattern) {
      const pattern = detail.pattern.length > 20
        ? detail.pattern.substring(0, 20) + '...'
        : detail.pattern;
      return `${name}: ${pattern}`;
    }

    // 기본: 도구 이름만
    return name;
  }

  // ============ 끝: 도구 인스펙터 함수들 ============

  // 메시지 텍스트에서 이미지 경로 파싱 → imageUrl 변환
  function parseImageFromText(text: string): { imageUrl?: string; cleanText: string } {
    const imagePattern = /\n?\n?\(첨부 이미지: ([^)]+)\)$/;
    const match = text.match(imagePattern);

    if (match) {
      const imagePath = match[1];
      // 절대 경로를 쿼리 파라미터로 전달
      const imageUrl = `${API_HOST}/api/image?path=${encodeURIComponent(imagePath)}`;
      const cleanText = text.replace(imagePattern, '').trim();
      return { imageUrl, cleanText };
    }

    return { cleanText: text };
  }

  // Claude 응답에서 파일 경로 파싱 (이미지 파일인 경우 URL 반환)
  function parseFilePathsFromResponse(text: string): { imageUrls: string[]; cleanText: string } {
    const imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp'];
    // Windows 절대 경로 패턴: C:\...\filename.ext
    const pathPattern = /([A-Za-z]:\\[^\s\n]+\.(png|jpg|jpeg|gif|webp|bmp))/gi;

    const matches = text.match(pathPattern) || [];
    const imageUrls: string[] = [];
    let cleanText = text;

    for (const match of matches) {
      const ext = match.substring(match.lastIndexOf('.')).toLowerCase();
      if (imageExts.includes(ext)) {
        // 절대 경로를 쿼리 파라미터로 전달
        imageUrls.push(`${API_HOST}/api/image?path=${encodeURIComponent(match)}`);
        // 경로를 텍스트에서 제거
        cleanText = cleanText.replace(match, '').trim();
      }
    }

    return { imageUrls, cleanText };
  }

  // initialMessages에서 이미지 경로 파싱
  function processMessages(messages: ChatMessage[]): ChatMessage[] {
    return messages.map(msg => {
      if (msg.isUser && !msg.imageUrl) {
        // 유저 메시지: [첨부 이미지: 경로] 파싱
        const { imageUrl, cleanText } = parseImageFromText(msg.text);
        if (imageUrl) {
          return { ...msg, imageUrl, text: cleanText };
        }
      } else if (!msg.isUser && !msg.imageUrls) {
        // Claude 메시지: 파일 경로 파싱
        const { imageUrls, cleanText } = parseFilePathsFromResponse(msg.text);
        if (imageUrls.length > 0) {
          return { ...msg, imageUrls, text: cleanText };
        }
      }
      return msg;
    });
  }

  // 파일 보기 허용 확장자
  const viewableExts = ['.md', '.txt', '.go', '.js', '.ts', '.jsx', '.tsx', '.json', '.html', '.css', '.svelte', '.vue', '.py', '.rs', '.yaml', '.yml', '.toml', '.sh', '.bat', '.sql', '.log'];

  function isViewable(name: string): boolean {
    const ext = name.substring(name.lastIndexOf('.')).toLowerCase();
    return viewableExts.includes(ext);
  }

  async function viewFile(name: string) {
    const filePath = currentPath + '\\' + name;
    isLoadingFile = true;
    showFileViewer = true;

    try {
      const res = await fetch(`${API_HOST}/api/file?path=${encodeURIComponent(filePath)}`);
      if (res.ok) {
        viewingFile = await res.json();
      } else {
        viewingFile = { name, ext: '', content: $_<string>('file.unreadable') };
      }
    } catch (err) {
      console.error($_<string>('error.file_load'), err);
      viewingFile = { name, ext: '', content: $_<string>('file.unreadable') };
    }
    isLoadingFile = false;
  }

  function closeFileViewer() {
    showFileViewer = false;
    viewingFile = null;
  }

  async function openSkillsModal() {
    showSkillsModal = true;
    isLoadingSkills = true;
    try {
      const res = await fetch(`${API_HOST}/api/skills`);
      if (res.ok) {
        const data = await res.json();
        skills = data.skills;
      }
    } catch (err) {
      console.error($_<string>('error.skills_load'), err);
    }
    isLoadingSkills = false;
  }

  function useSkill(skillName: string) {
    showSkillsModal = false;

    // 유저 메시지로 표시
    const userMessage: ChatMessage = {
      id: Date.now().toString(),
      text: `/${skillName}`,
      isUser: true,
      timestamp: Date.now(),
    };
    localMessages = [...localMessages, userMessage];
    onMessagesChange(localMessages);

    // 타이핑 표시 시작
    typingMessage = $_<string>('typing.skill', { values: { skill: skillName } });
    isTyping = true;

    // 실제 전송 (작업 폴더 정보 포함)
    let messageToSend = `/${skillName}`;
    if (workingPath) {
      messageToSend = `${messageToSend} (참고: 내가 보고 있는 폴더는 ${workingPath})`;
    }
    onSendMessage(messageToSend);
  }

  async function viewSkillMd(skill: { name: string; path: string }) {
    const skillMdPath = skill.path + '\\SKILL.md';
    isLoadingFile = true;
    showFileViewer = true;
    showSkillsModal = false;

    try {
      const res = await fetch(`${API_HOST}/api/file?path=${encodeURIComponent(skillMdPath)}`);
      if (res.ok) {
        viewingFile = await res.json();
      } else {
        viewingFile = { name: 'SKILL.md', ext: '.md', content: '파일을 읽을 수 없습니다.' };
      }
    } catch (err) {
      console.error('SKILL.md load failed:', err);
      viewingFile = { name: 'SKILL.md', ext: '.md', content: '파일을 읽을 수 없습니다.' };
    }
    isLoadingFile = false;
  }

  // 현재 locale 기반 타이핑 메시지 가져오기
  function getRandomTypingMessage() {
    // 현재 locale 판단
    const lang = navigator.language || 'en';
    const locale = lang.startsWith('ko') ? 'ko' : lang.startsWith('en') ? 'en' : 'default';

    // personaConfig의 typingMessages에서 가져오기
    const messages = personaConfig.typingMessages[locale] || personaConfig.typingMessages.default || ['Thinking...'];
    return messages[Math.floor(Math.random() * messages.length)];
  }

  // textarea 높이 자동 조절
  function autoResize() {
    if (textareaEl) {
      textareaEl.style.height = 'auto';
      textareaEl.style.height = Math.min(textareaEl.scrollHeight, 192) + 'px'; // max-h-48 = 192px
    }
  }

  // 마크다운 설정
  marked.setOptions({
    breaks: true,
    gfm: true,
  });

  // claudeState 변화에 따른 타이핑 표시 및 실시간 도구 목록 처리
  $: if (claudeState === 'idle') {
    isTyping = false;
    // idle로 돌아오면 pendingTools 초기화 (응답 완료 후)
    pendingTools = [];
    pendingToolsSet = new Set<string>();
  } else if (claudeState === 'working' && claudeTask) {
    // 서버에서 task 정보가 오면 타이핑 메시지 업데이트
    typingMessage = claudeTask;
    isTyping = true;

    // 도구 사용인 경우 pendingTools에 추가 (Read:, Edit:, Grep: 등으로 시작)
    if (claudeTask.match(/^(Read|Edit|Grep|Glob|Write|Bash|Task|WebFetch|WebSearch|TodoWrite|StructuredOutput):/)) {
      if (!pendingToolsSet.has(claudeTask)) {
        pendingToolsSet.add(claudeTask);
        pendingTools = [...pendingTools, claudeTask];
      }
    }
  }

  // 토큰 사용량 캡처 (props 전달 타이밍 문제 해결)
  let capturedTokenUsage: typeof lastTokenUsage = null;
  $: if (lastTokenUsage) {
    capturedTokenUsage = lastTokenUsage;
  }

  // 새 응답 처리 (중복 방지)
  $: if (lastResponse && lastResponse !== lastProcessedResponse) {
    lastProcessedResponse = lastResponse; // 처리 완료 표시
    isTyping = false; // 응답 오면 타이핑 표시 끄기

    // 스크롤이 맨 아래가 아니면 구분선 표시
    if (!isScrolledToBottom() && newMessageDividerIndex === null) {
      newMessageDividerIndex = localMessages.length;
    }

    // Claude 응답에서 이미지 경로 파싱
    const { imageUrls, cleanText } = parseFilePathsFromResponse(lastResponse);

    // 도구 정보 저장 (현재 값을 캡처)
    const toolsUsedToSave = lastToolsUsed.length > 0 ? [...lastToolsUsed] : undefined;
    const toolDetailsToSave = lastToolDetails.length > 0 ? [...lastToolDetails] : undefined;
    // 출력 토큰 저장 (캡처된 값 우선 사용)
    const tokenUsageToUse = capturedTokenUsage || lastTokenUsage;
    const outputTokensToSave = tokenUsageToUse?.outputTokens;

      const newMessage: ChatMessage = {
        id: Date.now().toString(),
        text: cleanText || lastResponse,
        isUser: false,
        timestamp: Date.now(),
        imageUrls: imageUrls.length > 0 ? imageUrls : undefined,
        toolsUsed: toolsUsedToSave,
        toolDetails: toolDetailsToSave,
        outputTokens: outputTokensToSave,
      };
      localMessages = [...localMessages, newMessage];
      onMessagesChange(localMessages);

      // 캡처 초기화
      capturedTokenUsage = null;

      // suggestions 업데이트
      currentSuggestions = lastSuggestions || [];
  }

  afterUpdate(() => {
    if (messageListEl) {
      // 메시지가 추가됐을 때 자동 스크롤
      if (localMessages.length > prevMessageCount) {
        // 새 메시지가 봇 응답이면 항상 스크롤 (구분선 있어도)
        const lastMsg = localMessages[localMessages.length - 1];
        if (lastMsg && !lastMsg.isUser) {
          messageListEl.scrollTop = messageListEl.scrollHeight;
          // 스크롤 후 구분선 제거
          if (newMessageDividerIndex !== null) {
            newMessageDividerIndex = null;
          }
        } else if (newMessageDividerIndex === null) {
          // 유저 메시지는 기존 로직 (구분선 없을 때만 스크롤)
          messageListEl.scrollTop = messageListEl.scrollHeight;
        }
        prevMessageCount = localMessages.length;
      }
      addCopyButtons();
    }
  });

  function addCopyButtons() {
    const codeBlocks = messageListEl.querySelectorAll('pre:not([data-copy-added])');
    codeBlocks.forEach((pre) => {
      pre.setAttribute('data-copy-added', 'true');

      const wrapper = document.createElement('div');
      wrapper.className = 'code-block-wrapper relative';

      const button = document.createElement('button');
      button.className = 'copy-btn';
      button.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>`;
      button.onclick = () => {
        const code = pre.querySelector('code')?.textContent || pre.textContent || '';
        navigator.clipboard.writeText(code);
        button.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;
        button.style.color = '#4fd1c5';
        setTimeout(() => {
          button.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>`;
          button.style.color = '';
        }, 1500);
      };

      pre.parentNode?.insertBefore(wrapper, pre);
      wrapper.appendChild(pre);
      wrapper.appendChild(button);
    });
  }

  function parseMarkdown(text: string): string {
    return marked.parse(text) as string;
  }

  function handleCancel() {
    isTyping = false;
    onCancel();
  }

  function handleSend() {
    if (!inputText.trim() && !uploadedImagePath) return;

    // suggestions 숨기기
    currentSuggestions = [];

    // 타이핑 표시 시작
    typingMessage = getRandomTypingMessage();
    isTyping = true;

    const userText = inputText.trim();

    // 이미지 URL 생성 (있으면)
    let imageUrl: string | undefined;
    if (uploadedImagePath) {
      // 절대 경로를 쿼리 파라미터로 전달
      imageUrl = `${API_HOST}/api/image?path=${encodeURIComponent(uploadedImagePath)}`;
    }

    const userMessage: ChatMessage = {
      id: Date.now().toString(),
      text: userText,
      isUser: true,
      timestamp: Date.now(),
      imageUrl,
    };
    localMessages = [...localMessages, userMessage];
    onMessagesChange(localMessages);

    // 실제 전송할 메시지 (작업 폴더를 메시지 내에 자연스럽게 포함)
    let messageToSend = userText;
    if (workingPath) {
      // "여기", "이 폴더" 등을 실제 경로로 치환
      messageToSend = `${messageToSend} (참고: 내가 보고 있는 폴더는 ${workingPath})`;
    }
    if (uploadedImagePath) {
      messageToSend = `${messageToSend}\n\n(첨부 이미지: ${uploadedImagePath})`;
    }

    onSendMessage(messageToSend);
    inputText = '';
    uploadedImagePath = null; // 전송 후 이미지 초기화
    // 높이 리셋
    if (textareaEl) {
      textareaEl.style.height = 'auto';
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    // Ctrl+Enter 또는 Cmd+Enter로 전송 (데스크탑용)
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      handleSend();
    }
    // 일반 Enter는 줄바꿈 (기본 동작 유지)
  }

  // suggestion 버튼 클릭 시
  function handleSuggestionClick(suggestion: string) {
    currentSuggestions = []; // suggestions 숨기기
    inputText = suggestion; // 입력창에 텍스트 채우기
    handleSend(); // 바로 전송
  }

  function formatTime(timestamp: number): string {
    return new Date(timestamp).toLocaleTimeString('ko-KR', {
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  // 전체 대화 복사
  async function copyAllMessages() {
    const allText = localMessages.map(msg => {
      const sender = msg.isUser ? 'You' : 'Assistant';
      return `[${sender}]\n${msg.text}`;
    }).join('\n\n---\n\n');

    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(allText);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = allText;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '0';
        textarea.style.opacity = '0';
        textarea.setAttribute('readonly', '');
        document.body.appendChild(textarea);
        textarea.select();
        textarea.setSelectionRange(0, allText.length);
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      // TODO: 복사 완료 피드백 (토스트 등)
    } catch (err) {
      console.error($_<string>('error.copy_all'), err);
    }
  }

  // 서버 재시작
  async function restartServer() {
    if (isRestarting) return;

    if (!confirm($_('settings.restart_confirm'))) {
      return;
    }

    isRestarting = true;
    try {
      const res = await fetch(`${API_BASE}/api/restart`, {
        method: 'POST',
      });
      if (res.ok) {
        console.log('[Restart] Request successful');
      }
    } catch (err) {
      console.error('[Restart] Request failed:', err);
    }
    // 재시작 중 상태는 서버가 다시 올라올 때까지 유지
    setTimeout(() => { isRestarting = false; }, 10000);
  }

  async function copyMessage(text: string, id: string) {
    // 현재 스크롤 위치 저장
    const scrollTop = messageListEl?.scrollTop || 0;

    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '0';
        textarea.style.opacity = '0';
        textarea.setAttribute('readonly', '');
        document.body.appendChild(textarea);
        textarea.select();
        textarea.setSelectionRange(0, text.length);
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }

      // 스크롤 위치 복원
      if (messageListEl) {
        messageListEl.scrollTop = scrollTop;
      }
      copiedId = id;
      setTimeout(() => { copiedId = null; }, 1500);
    } catch (err) {
      console.error($_<string>('error.copy'), err);
    }
  }
</script>

<!-- Rico 스타일 채팅 화면 - CSS 변수 적용 -->
<div class="flex flex-col h-[100dvh] bg-[var(--bg-primary)]" on:click={() => showPlusMenu = false}>
  <!-- 헤더 -->
  <div class="flex items-center justify-between px-3 py-2.5 pt-[calc(0.625rem+env(safe-area-inset-top))] bg-[var(--bg-primary)] border-b border-[var(--border-primary)]/50">
    <div class="flex items-center gap-2">
      <button class="text-[var(--text-dimmed)] hover:text-[var(--text-muted)] p-1.5 -ml-1.5 rounded-lg hover:bg-[var(--border-primary)] transition-all" on:click={onBack} aria-label={$_('chat.back')}>
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div class="flex items-center gap-2">
        <div class="w-8 h-8 rounded-full bg-gradient-to-br from-[var(--accent-primary)] to-[var(--accent-secondary)] flex items-center justify-center shadow-lg" style="box-shadow: 0 4px 12px var(--accent-primary-shadow);">
          <span class="text-white text-sm font-bold">{avatarChar}</span>
        </div>
        <div class="flex flex-col">
          <span class="text-[var(--text-primary)] font-semibold text-sm leading-tight">{activeSoulName}</span>
          <span class="text-xs {isConnected ? 'text-[var(--accent-primary)]' : 'text-[var(--red-primary)]'}">{isConnected ? $_('chat.connected') : $_('chat.disconnected')}</span>
        </div>
      </div>
    </div>
    <button
      class="text-[var(--text-dimmed)] hover:text-[var(--text-muted)] p-2 rounded-lg hover:bg-[var(--border-primary)] transition-all"
      on:click|stopPropagation={() => showSettingsMenu = true}
      title={$_('settings.title')}
      aria-label={$_('settings.title')}
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
      </svg>
    </button>
  </div>

  <!-- 메시지 목록 - 더 넓은 영역, 개선된 가독성 -->
  <div
    bind:this={messageListEl}
    class="flex-1 overflow-y-auto overflow-x-hidden px-3 py-3"
    on:scroll={handleScroll}
  >
    {#each localMessages as message, index (message.id)}
      <!-- 새 메시지 구분선 -->
      {#if newMessageDividerIndex !== null && index === newMessageDividerIndex}
        <div class="new-message-divider flex items-center gap-3 my-4 px-2">
          <div class="flex-1 h-px bg-[var(--red-primary)]/40"></div>
          <span class="text-[var(--red-primary)] text-xs font-medium px-2">{$_('chat.new_message')}</span>
          <div class="flex-1 h-px bg-[var(--red-primary)]/40"></div>
        </div>
      {/if}
      <div
        class="flex gap-2.5 py-2 hover:bg-[var(--bg-hover)] rounded-2xl px-2.5 -mx-2.5 group select-none transition-all duration-150 touch-callout-none {pressingMessageId === message.id ? 'bg-[var(--bg-hover)] scale-[0.98] opacity-70' : ''}"
        on:touchstart={(e) => handleMessageTouchStart(e, message)}
        on:touchend={handleMessageTouchEnd}
        on:touchmove={handleMessageTouchEnd}
        on:contextmenu|preventDefault={() => { selectedMessage = message; showMessageActions = true; }}
      >
        <!-- 아바타 -->
        <div class="flex-shrink-0 pt-0.5">
          {#if message.isUser}
            <div class="w-8 h-8 rounded-full bg-gradient-to-br from-[var(--purple-secondary)] to-[var(--purple-dark)] flex items-center justify-center text-white text-xs font-bold shadow-md" style="box-shadow: 0 2px 8px var(--purple-shadow);">
              U
            </div>
          {:else}
            <div class="w-8 h-8 rounded-full bg-gradient-to-br from-[var(--accent-primary)] to-[var(--accent-secondary)] flex items-center justify-center text-white text-xs font-bold shadow-md" style="box-shadow: 0 2px 8px var(--accent-primary-shadow);">
              {avatarChar}
            </div>
          {/if}
        </div>

        <!-- 메시지 내용 -->
        <div class="flex-1 min-w-0 overflow-hidden">
          <div class="flex items-baseline gap-2">
            <span class="font-semibold text-sm {message.isUser ? 'text-[var(--purple-primary)]' : 'text-[var(--accent-primary)]'}">
              {message.isUser ? 'You' : activeSoulName}
            </span>
            <span class="text-[var(--text-faint)] text-[11px]">
              {formatTime(message.timestamp)}
            </span>
            {#if !message.isUser && message.outputTokens}
              <span class="text-[var(--text-faint)] text-[10px] font-mono">
                · {message.outputTokens.toLocaleString()}t
              </span>
            {/if}
          </div>

          {#if message.isUser}
            {#if message.imageUrl}
              <button
                class="mt-2 block"
                on:click|stopPropagation={() => openImageViewer(message.imageUrl!)}
              >
                <img
                  src={message.imageUrl}
                  alt={$_('image.attached')}
                  class="rounded-xl max-w-[180px] max-h-[180px] object-cover border border-[var(--border-primary)] shadow-lg"
                />
              </button>
            {/if}
            {#if message.text}
              <p class="text-[var(--text-secondary)] text-[15px] leading-[1.7] whitespace-pre-wrap mt-1">{message.text}</p>
            {/if}
          {:else}
            <!-- 사용된 도구 목록 (클릭하여 상세 정보 보기) -->
            {#if message.toolDetails && message.toolDetails.length > 0}
              <div
                class="flex flex-col gap-1 mt-1 mb-2"
                on:touchstart|stopPropagation
                on:touchend|stopPropagation
              >
                {#each message.toolDetails as detail, toolIndex}
                  {@const toolKey = `${message.id}:${toolIndex}`}
                  {@const isExpanded = expandedToolKey === toolKey}
                  <div class="flex flex-col">
                    <button
                      class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] rounded text-[11px] text-[var(--text-muted)] font-mono w-fit transition-colors cursor-pointer"
                      on:click|stopPropagation={() => toggleToolDetail(message.id, toolIndex)}
                    >
                      <svg class="w-3 h-3 text-[var(--accent-primary)] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                      </svg>
                      <span>{getToolLabel(detail)}</span>
                      <svg class="w-3 h-3 transition-transform {isExpanded ? 'rotate-180' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                      </svg>
                    </button>
                    {#if isExpanded}
                      <div class="ml-4 mt-2 p-3 bg-[var(--bg-secondary)] rounded border border-[var(--border-primary)] text-[11px] space-y-3">
                        <!-- 파일 경로 (있을 때만) -->
                        {#if detail.file}
                          <div class="text-[var(--text-muted)]">
                            <span class="text-[var(--text-secondary)]">경로:</span> {detail.file}
                          </div>
                        {/if}

                        <!-- Read: 범위 정보 + 내용 보기 버튼 -->
                        {#if detail.name === 'Read'}
                          {#if detail.offset !== undefined || detail.limit !== undefined}
                            <div class="text-[var(--text-muted)]">
                              <span class="text-[var(--text-secondary)]">범위:</span> {getToolRangeText(detail)}
                            </div>
                          {/if}
                          <button
                            class="px-3 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-secondary)] text-white rounded text-[10px] transition-colors"
                            on:click|stopPropagation={() => loadToolContent(toolKey, detail)}
                          >
                            {toolContentCache[toolKey] ? '내용 숨기기' : '내용 보기'}
                          </button>
                          {#if toolContentCache[toolKey]}
                            {#if toolContentCache[toolKey].loading}
                              <div class="text-[var(--text-muted)]">로딩 중...</div>
                            {:else if toolContentCache[toolKey].error}
                              <div class="text-[var(--red-primary)]">{toolContentCache[toolKey].error}</div>
                            {:else}
                              <pre data-tool-detail class="p-2 bg-[var(--bg-primary)] rounded overflow-x-auto text-[10px] text-[var(--text-secondary)] max-h-[200px] overflow-y-auto whitespace-pre-wrap">{toolContentCache[toolKey].content}</pre>
                            {/if}
                          {/if}
                        {/if}

                        <!-- Edit: 변경 전/후 표시 -->
                        {#if detail.name === 'Edit' && (detail.oldString || detail.newString)}
                          <div class="space-y-3">
                            {#if detail.oldString}
                              <div>
                                <span class="text-[var(--red-primary)] font-medium">- 변경 전:</span>
                                <pre data-tool-detail class="mt-2 p-2 bg-[#2a1a1a] border border-[var(--red-primary)]/30 rounded overflow-x-auto text-[10px] text-[var(--text-secondary)] max-h-[150px] overflow-y-auto whitespace-pre-wrap">{addLineNumbers(detail.oldString)}</pre>
                              </div>
                            {/if}
                            {#if detail.newString}
                              <div>
                                <span class="text-[var(--green-primary)] font-medium">+ 변경 후:</span>
                                <pre data-tool-detail class="mt-2 p-2 bg-[#1a2a1a] border border-[var(--green-primary)]/30 rounded overflow-x-auto text-[10px] text-[var(--text-secondary)] max-h-[150px] overflow-y-auto whitespace-pre-wrap">{addLineNumbers(detail.newString)}</pre>
                              </div>
                            {/if}
                          </div>
                        {/if}

                        <!-- Bash: 명령어 표시 -->
                        {#if detail.name === 'Bash' && detail.command}
                          <div class="text-[var(--text-muted)]">
                            <span class="text-[var(--text-secondary)]">명령어:</span>
                            <pre data-tool-detail class="mt-2 p-2 bg-[var(--bg-primary)] rounded overflow-x-auto text-[10px] text-[var(--text-secondary)] max-h-[100px] overflow-y-auto whitespace-pre-wrap">{detail.command}</pre>
                          </div>
                        {/if}

                        <!-- Grep/Glob: 검색 패턴 표시 -->
                        {#if (detail.name === 'Grep' || detail.name === 'Glob') && detail.pattern}
                          <div class="text-[var(--text-muted)]">
                            <span class="text-[var(--text-secondary)]">패턴:</span>
                            <code class="ml-1 px-1.5 py-0.5 bg-[var(--bg-tertiary)] rounded text-[var(--accent-primary)]">{detail.pattern}</code>
                          </div>
                        {/if}

                        <!-- StructuredOutput: suggestions 표시 -->
                        {#if detail.name === 'StructuredOutput' && detail.suggestions && detail.suggestions.length > 0}
                          <div class="text-[var(--text-muted)] mt-1">
                            <span class="text-[var(--text-secondary)]">제안:</span>
                            <div class="flex flex-col gap-1 mt-1">
                              {#each detail.suggestions as suggestion, i}
                                <span class="text-[11px] px-2 py-1 bg-[var(--bg-primary)] rounded text-[var(--text-tertiary)]">
                                  {i + 1}. {suggestion}
                                </span>
                              {/each}
                            </div>
                          </div>
                        {/if}

                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {:else if message.toolsUsed && message.toolsUsed.length > 0}
              <!-- toolDetails가 없으면 기존 toolsUsed 표시 (이전 메시지 호환) -->
              <div
                class="flex flex-col gap-1 mt-1 mb-2"
                on:touchstart|stopPropagation
                on:touchend|stopPropagation
              >
                {#each message.toolsUsed as tool}
                  <div class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-[var(--bg-tertiary)] rounded text-[11px] text-[var(--text-muted)] font-mono w-fit">
                    <svg class="w-3 h-3 text-[var(--accent-primary)] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                    </svg>
                    <span>{tool}</span>
                  </div>
                {/each}
              </div>
            {/if}
            <div class="prose prose-rico max-w-none mt-1">
              {@html parseMarkdown(message.text)}
            </div>
            {#if message.imageUrls && message.imageUrls.length > 0}
              <div class="flex flex-wrap gap-2 mt-3">
                {#each message.imageUrls as imgUrl}
                  <button
                    class="block"
                    on:click|stopPropagation={() => openImageViewer(imgUrl)}
                  >
                    <img
                      src={imgUrl}
                      alt="이미지"
                      class="rounded-xl max-w-[180px] max-h-[180px] object-cover border border-[var(--border-primary)] shadow-lg"
                    />
                  </button>
                {/each}
              </div>
            {/if}
          {/if}
        </div>
      </div>
    {/each}

    <!-- 타이핑 표시 - 실시간 도구 태그 포함 -->
    {#if isTyping}
      <div class="flex gap-2.5 py-2 px-2.5 -mx-2.5 items-start">
        <div class="flex-shrink-0">
          <div class="w-8 h-8 rounded-full bg-gradient-to-br from-[var(--accent-primary)] to-[var(--accent-secondary)] flex items-center justify-center text-white text-xs font-bold shadow-md" style="box-shadow: 0 2px 8px var(--accent-primary-shadow);">
            {avatarChar}
          </div>
        </div>
        <div class="flex-1 min-w-0">
          <!-- 실시간 도구 사용 목록 -->
          {#if pendingTools.length > 0}
            <div class="flex flex-col gap-1 mb-2">
              {#each pendingTools as tool}
                <div class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-[var(--bg-tertiary)] rounded text-[11px] text-[var(--text-muted)] font-mono w-fit animate-fade-in">
                  <svg class="w-3 h-3 text-[var(--accent-primary)] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                  </svg>
                  <span>{tool}</span>
                </div>
              {/each}
            </div>
          {/if}
          <!-- 현재 작업 상태 -->
          <div class="flex items-center gap-1">
            <span class="text-[var(--text-dimmed)] text-sm">{typingMessage}</span>
            <span class="typing-dots">
              <span class="dot">.</span><span class="dot">.</span><span class="dot">.</span>
            </span>
          </div>
        </div>
        <button
          class="px-3 py-1.5 bg-[#f87171]/10 hover:bg-[#f87171]/20 text-[var(--red-primary)] text-xs font-medium rounded-full transition-colors flex-shrink-0 self-start"
          on:click={handleCancel}
          aria-label={$_('chat.cancel')}
        >
          {$_('chat.cancel')}
        </button>
      </div>
    {/if}
  </div>

  <!-- Suggestions 버튼들 - 더 세련되게 -->
  {#if currentSuggestions.length > 0 && !isTyping}
    <div class="px-3 pt-3 pb-2">
      <div class="flex flex-wrap gap-1.5">
        {#each currentSuggestions as suggestion}
          <button
            class="px-3 py-1.5 bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] text-[var(--text-tertiary)] text-[13px] rounded-full border border-[var(--border-primary)] hover:border-[#3d4f4f] transition-all shadow-sm"
            on:click={() => handleSuggestionClick(suggestion)}
          >
            {suggestion}
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- 입력창 영역 - 전면 개선 -->
  <div class="px-3 pb-[calc(1rem+env(safe-area-inset-bottom))] pt-2 relative bg-[var(--bg-primary)]">
    <!-- 첨부된 이미지 표시 -->
    {#if uploadedImagePath || isUploading}
      <div class="flex items-center gap-2 mb-2 px-3 py-2.5 bg-[var(--bg-tertiary)] rounded-xl border border-[var(--border-primary)]">
        <div class="w-8 h-8 rounded-lg bg-[#f6ad55]/10 flex items-center justify-center flex-shrink-0">
          <svg class="w-4 h-4 text-[var(--orange-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/>
          </svg>
        </div>
        {#if isUploading}
          <span class="text-[var(--text-dimmed)] text-sm flex-1">{$_('chat.uploading')}</span>
        {:else}
          <span class="text-[var(--text-muted)] text-sm flex-1 truncate">
            {uploadedImagePath?.split('\\').pop() || uploadedImagePath?.split('/').pop() || '이미지'}
          </span>
          <button
            class="w-7 h-7 rounded-lg bg-[#f87171]/10 hover:bg-[#f87171]/20 flex items-center justify-center transition-colors"
            on:click={clearUploadedImage}
            title={$_('file.remove_image')}
          >
            <svg class="w-4 h-4 text-[var(--red-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        {/if}
      </div>
    {/if}

    <!-- 작업 폴더 표시 -->
    {#if workingPath}
      <div class="flex items-center gap-2 mb-2 px-3 py-2.5 bg-[var(--bg-tertiary)] rounded-xl border border-[var(--border-primary)]">
        <div class="w-8 h-8 rounded-lg bg-[var(--accent-primary)]/10 flex items-center justify-center flex-shrink-0">
          <svg class="w-4 h-4 text-[var(--accent-primary)]" fill="currentColor" viewBox="0 0 24 24">
            <path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
          </svg>
        </div>
        <span class="text-[var(--text-muted)] text-sm flex-1 truncate">{workingPath}</span>
        <button
          class="w-7 h-7 rounded-lg bg-[#f87171]/10 hover:bg-[#f87171]/20 flex items-center justify-center transition-colors"
          on:click={clearWorkingPath}
          title={$_('file.remove_folder')}
        >
          <svg class="w-4 h-4 text-[var(--red-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    {/if}

    <!-- 메인 입력창 -->
    <div class="flex items-end gap-2 bg-[var(--bg-tertiary)] rounded-2xl p-2 border border-[var(--border-primary)] shadow-xl shadow-black/20">
      <!-- + 버튼 -->
      <button
        class="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 transition-all {showPlusMenu ? 'bg-[var(--accent-primary)]/20 text-[var(--accent-primary)]' : 'text-[var(--text-faint)] hover:text-[var(--text-dimmed)] hover:bg-[var(--bg-hover)]'}"
        on:click|stopPropagation={() => showPlusMenu = !showPlusMenu}
        aria-label={$_('menu.open')}
        aria-expanded={showPlusMenu}
      >
        <svg class="w-5 h-5 transition-transform {showPlusMenu ? 'rotate-45' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4"/>
        </svg>
      </button>

      <!-- + 메뉴 팝업 -->
      {#if showPlusMenu}
        <div class="absolute bottom-20 left-3 bg-[var(--bg-tertiary)] rounded-2xl border border-[var(--border-primary)] shadow-2xl shadow-black/40 overflow-hidden z-10 min-w-[160px]">
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 hover:bg-[var(--bg-hover)] text-left transition-colors"
            on:click={() => { showPlusMenu = false; openFileBrowser(); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[var(--accent-primary)]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--accent-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{$_('menu.folder')}</span>
          </button>
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 hover:bg-[var(--bg-hover)] text-left transition-colors border-t border-[var(--border-primary)]"
            on:click={() => { showPlusMenu = false; triggerImageUpload(); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[#f6ad55]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--orange-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/>
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{$_('menu.image')}</span>
          </button>
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 hover:bg-[var(--bg-hover)] text-left transition-colors border-t border-[var(--border-primary)]"
            on:click={() => { showPlusMenu = false; openSkillsModal(); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[#a78bfa]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--purple-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{$_('menu.skills')}</span>
          </button>
        </div>
      {/if}

      <!-- 숨겨진 파일 input -->
      <input
        bind:this={fileInputEl}
        type="file"
        accept="image/*"
        class="hidden"
        on:change={handleImageSelect}
      />

      <!-- 텍스트 입력 영역 -->
      <textarea
        bind:this={textareaEl}
        bind:value={inputText}
        on:keydown={handleKeydown}
        on:input={autoResize}
        placeholder={$_('chat.placeholder')}
        class="flex-1 bg-transparent text-[var(--text-secondary)] text-[15px] resize-none max-h-32 placeholder-[var(--text-faint)] outline-none leading-6 overflow-y-auto py-2.5 px-1"
        rows="1"
      ></textarea>

      <!-- 전송 버튼 -->
      <button
        class="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 transition-all {inputText.trim() || uploadedImagePath ? 'bg-[var(--accent-primary)] text-[#1a2222] shadow-lg shadow-[#4fd1c5]/30 hover:bg-[var(--accent-primary-hover)]' : 'text-[var(--border-primary)] cursor-not-allowed'}"
        on:click={handleSend}
        disabled={!inputText.trim() && !uploadedImagePath}
        aria-label={$_('action.send')}
      >
        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- 파일 브라우저 모달 - 리디자인 -->
  {#if showFileBrowser}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-end justify-center backdrop-blur-sm" on:click={() => showFileBrowser = false}>
      <div
        class="bg-[var(--bg-primary)] w-full max-w-md rounded-t-3xl max-h-[80vh] flex flex-col shadow-2xl"
        on:click|stopPropagation
      >
        <!-- 핸들 바 -->
        <div class="flex justify-center pt-3 pb-1">
          <div class="w-10 h-1 bg-[#2d3a3a] rounded-full"></div>
        </div>

        <!-- 모달 헤더 -->
        <div class="flex items-center justify-between px-4 py-2 border-b border-[var(--border-primary)]">
          <h3 class="text-[var(--text-primary)] font-semibold text-base">{$_('file.title')}</h3>
          <button
            class="w-8 h-8 rounded-lg flex items-center justify-center text-[var(--text-faint)] hover:text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)] transition-all"
            on:click={() => showFileBrowser = false}
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>

        <!-- 최근 이동 -->
        <div class="px-4 py-2.5 border-b border-[var(--border-primary)]">
          <div class="flex flex-wrap gap-1.5">
            {#each recentPaths as rp}
              <button
                class="px-3 py-1.5 bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] text-[var(--text-tertiary)] text-[13px] rounded-full border border-[var(--border-primary)] hover:border-[#3d4f4f] transition-all"
                on:click={() => selectQuickPath(rp.path)}
              >
                {rp.name}
              </button>
            {/each}
          </div>
        </div>

        <!-- 현재 경로 + 상위 이동 -->
        <div class="flex items-center gap-2 px-4 py-2.5 bg-[var(--bg-secondary)] border-b border-[var(--border-primary)]">
          <button
            class="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-[var(--bg-tertiary)] text-[var(--text-dimmed)] hover:text-[var(--text-muted)] transition-all"
            on:click={goUp}
            title={$_('file.up')}
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7"/>
            </svg>
          </button>
          <span class="text-[var(--text-muted)] text-sm truncate flex-1 font-mono">{currentPath}</span>
          <button
            class="px-3 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-primary-hover)] text-[#1a2222] text-xs font-semibold rounded-lg shadow-lg shadow-[#4fd1c5]/20 transition-all"
            on:click={() => selectPath(currentPath)}
          >
            {$_('file.select')}
          </button>
        </div>

        <!-- 폴더/파일 목록 -->
        <div class="flex-1 overflow-y-auto pb-20">
          {#if isLoadingFolder}
            <div class="flex items-center justify-center py-10">
              <span class="text-[var(--text-faint)]">{$_('file.loading')}</span>
            </div>
          {:else if folderContents.length === 0}
            <div class="flex items-center justify-center py-10">
              <span class="text-[var(--text-faint)]">{$_('file.empty')}</span>
            </div>
          {:else}
            {#each folderContents as item}
              <button
                class="w-full flex items-center gap-3 px-4 py-3 hover:bg-[var(--bg-hover)] text-left transition-all {!item.isDir && !isViewable(item.name) ? 'opacity-40' : ''}"
                on:click={() => {
                  if (item.isDir) {
                    navigateToFolder(item.name);
                  } else if (isViewable(item.name)) {
                    viewFile(item.name);
                  }
                }}
                disabled={!item.isDir && !isViewable(item.name)}
              >
                <div class="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0 {item.isDir ? 'bg-[var(--accent-primary)]/10' : isViewable(item.name) ? 'bg-[#f6ad55]/10' : 'bg-[#2d3a3a]'}">
                  {#if item.isDir}
                    <svg class="w-5 h-5 text-[var(--accent-primary)]" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
                    </svg>
                  {:else}
                    <svg class="w-5 h-5 {isViewable(item.name) ? 'text-[var(--orange-primary)]' : 'text-[var(--text-faint)]'}" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zM6 20V4h7v5h5v11H6z"/>
                    </svg>
                  {/if}
                </div>
                <span class="{item.isDir ? 'text-[var(--text-secondary)]' : isViewable(item.name) ? 'text-[var(--orange-primary)]' : 'text-[var(--text-faint)]'} text-sm truncate">
                  {item.name}
                </span>
              </button>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- 파일 뷰어 모달 - 리디자인 -->
  {#if showFileViewer}
    <div class="fixed inset-0 bg-black/80 z-50 flex flex-col backdrop-blur-sm" on:click={closeFileViewer}>
      <div
        class="flex-1 flex flex-col bg-[var(--bg-primary)] m-2 rounded-2xl overflow-hidden shadow-2xl"
        on:click|stopPropagation
      >
        <!-- 헤더 -->
        <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--border-primary)] bg-[var(--bg-secondary)]">
          <div class="flex items-center gap-3 min-w-0">
            <div class="w-9 h-9 rounded-lg bg-[#f6ad55]/10 flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-[var(--orange-primary)]" fill="currentColor" viewBox="0 0 24 24">
                <path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zM6 20V4h7v5h5v11H6z"/>
              </svg>
            </div>
            <span class="text-[var(--text-primary)] font-medium truncate">{viewingFile?.name || '파일'}</span>
          </div>
          <button
            class="w-9 h-9 rounded-lg flex items-center justify-center text-[var(--text-faint)] hover:text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)] transition-all flex-shrink-0"
            on:click={closeFileViewer}
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>

        <!-- 파일 내용 -->
        <div class="flex-1 overflow-auto p-4">
          {#if isLoadingFile}
            <div class="flex items-center justify-center h-full">
              <span class="text-[var(--text-faint)]">{$_('file.loading')}</span>
            </div>
          {:else if viewingFile}
            {#if viewingFile.ext === '.md'}
              <div class="prose prose-rico max-w-none">
                {@html parseMarkdown(viewingFile.content)}
              </div>
            {:else}
              <pre class="text-[var(--text-secondary)] text-[13px] whitespace-pre-wrap break-all font-mono leading-relaxed">{viewingFile.content}</pre>
            {/if}
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- Skills 모달 - 리디자인 -->
  {#if showSkillsModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-end justify-center backdrop-blur-sm" on:click={() => showSkillsModal = false}>
      <div
        class="bg-[var(--bg-primary)] w-full max-w-md rounded-t-3xl max-h-[60vh] flex flex-col shadow-2xl"
        on:click|stopPropagation
      >
        <!-- 핸들 바 -->
        <div class="flex justify-center pt-3 pb-1">
          <div class="w-10 h-1 bg-[#2d3a3a] rounded-full"></div>
        </div>

        <!-- 모달 헤더 -->
        <div class="flex items-center justify-between px-4 py-2 border-b border-[var(--border-primary)]">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-[#a78bfa]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--purple-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
              </svg>
            </div>
            <h3 class="text-[var(--text-primary)] font-semibold text-base">{$_('skills.title')}</h3>
          </div>
          <button
            class="w-8 h-8 rounded-lg flex items-center justify-center text-[var(--text-faint)] hover:text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)] transition-all"
            on:click={() => showSkillsModal = false}
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>

        <!-- Skills 목록 -->
        <div class="flex-1 overflow-y-auto pb-20">
          {#if isLoadingSkills}
            <div class="flex items-center justify-center py-10">
              <span class="text-[var(--text-faint)]">{$_('file.loading')}</span>
            </div>
          {:else if skills.length === 0}
            <div class="flex items-center justify-center py-10">
              <span class="text-[var(--text-faint)]">{$_('skills.empty')}</span>
            </div>
          {:else}
            {#each skills as skill}
              <div class="flex items-center gap-3 px-4 py-3 border-b border-[var(--border-primary)]/50">
                <!-- 스킬 정보 -->
                <button
                  class="flex-1 min-w-0 flex items-center gap-3 hover:bg-[var(--bg-hover)] text-left transition-all rounded-xl p-2 -m-2"
                  on:click={() => viewSkillMd(skill)}
                >
                  <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center text-white text-sm font-bold flex-shrink-0 shadow-lg shadow-[#a78bfa]/20">
                    {skill.name.charAt(0).toUpperCase()}
                  </div>
                  <div class="flex-1 min-w-0 overflow-hidden">
                    <span class="text-[var(--text-primary)] font-medium text-sm block truncate">/{skill.name}</span>
                    {#if skill.description}
                      <p class="text-[var(--text-dimmed)] text-xs mt-0.5 truncate">{skill.description}</p>
                    {/if}
                  </div>
                </button>
                <!-- 실행 버튼 -->
                <button
                  class="w-10 h-10 rounded-xl bg-[var(--accent-primary)] hover:bg-[var(--accent-primary-hover)] flex items-center justify-center flex-shrink-0 transition-all shadow-lg shadow-[#4fd1c5]/20"
                  on:click={() => useSkill(skill.name)}
                  title={$_('skills.run')}
                >
                  <svg class="w-5 h-5 text-[#1a2222]" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M8 5v14l11-7z"/>
                  </svg>
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- 이미지 뷰어 - 리디자인 -->
  {#if showImageViewer && viewingImageUrl}
    <div
      class="fixed inset-0 bg-black/95 z-50 flex items-center justify-center backdrop-blur-md"
      on:click={closeImageViewer}
    >
      <!-- 닫기 버튼 -->
      <button
        class="absolute top-4 right-4 w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center text-white/80 hover:text-white z-10 transition-all"
        style="top: calc(1rem + env(safe-area-inset-top))"
        on:click={closeImageViewer}
      >
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>

      <!-- 이미지 -->
      <img
        src={viewingImageUrl}
        alt={$_('image.enlarged')}
        class="max-w-full max-h-full object-contain p-4"
        on:click|stopPropagation
      />
    </div>
  {/if}

  <!-- 메시지 액션 시트 - 리디자인 -->
  {#if showMessageActions}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-end justify-center backdrop-blur-sm" on:click={closeMessageActions}>
      <div
        class="bg-[var(--bg-primary)] w-full max-w-md rounded-t-3xl overflow-hidden animate-slide-up shadow-2xl"
        on:click|stopPropagation
      >
        <!-- 핸들 바 -->
        <div class="flex justify-center pt-3 pb-1">
          <div class="w-10 h-1 bg-[#2d3a3a] rounded-full"></div>
        </div>

        <!-- 선택된 메시지 미리보기 -->
        {#if selectedMessage}
          <div class="px-4 pb-3 border-b border-[var(--border-primary)]">
            <p class="text-[var(--text-dimmed)] text-sm line-clamp-2">{selectedMessage.text}</p>
          </div>
        {/if}

        <!-- 액션 버튼들 -->
        <div class="py-2">
          <button
            class="w-full flex items-center gap-4 px-5 py-4 text-left transition-all duration-150 hover:bg-[var(--bg-hover)] {pressingActionButton === 'copy' ? 'bg-[var(--bg-hover)] scale-[0.98] opacity-70' : ''}"
            on:click={copySelectedMessage}
            on:touchstart={() => pressingActionButton = 'copy'}
            on:touchend={() => pressingActionButton = null}
            on:touchcancel={() => pressingActionButton = null}
          >
            <div class="w-10 h-10 rounded-xl bg-[var(--accent-primary)]/10 flex items-center justify-center">
              <svg class="w-5 h-5 text-[var(--accent-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
              </svg>
            </div>
            <span class="text-[var(--text-primary)] text-base font-medium">{$_('action.copy')}</span>
          </button>
          <button
            class="w-full flex items-center gap-4 px-5 py-4 text-left transition-all duration-150 hover:bg-[var(--bg-hover)] {pressingActionButton === 'delete' ? 'bg-[var(--bg-hover)] scale-[0.98] opacity-70' : ''}"
            on:click={deleteSelectedMessage}
            on:touchstart={() => pressingActionButton = 'delete'}
            on:touchend={() => pressingActionButton = null}
            on:touchcancel={() => pressingActionButton = null}
          >
            <div class="w-10 h-10 rounded-xl bg-[#f87171]/10 flex items-center justify-center">
              <svg class="w-5 h-5 text-[var(--red-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </div>
            <span class="text-[var(--red-primary)] text-base font-medium">{$_('action.delete')}</span>
          </button>
        </div>

        <!-- 취소 버튼 -->
        <div class="px-4 pb-[calc(1rem+env(safe-area-inset-bottom))]">
          <button
            class="w-full py-3.5 bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] rounded-xl text-[var(--text-tertiary)] font-medium transition-all duration-150 {pressingActionButton === 'cancel' ? 'bg-[#2d3a3a] scale-[0.98] opacity-70' : ''}"
            on:click={closeMessageActions}
            on:touchstart={() => pressingActionButton = 'cancel'}
            on:touchend={() => pressingActionButton = null}
            on:touchcancel={() => pressingActionButton = null}
          >
            {$_('action.cancel')}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- 설정 메뉴 바텀시트 -->
  {#if showSettingsMenu}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-end justify-center backdrop-blur-sm" on:click={() => showSettingsMenu = false}>
      <div
        class="bg-[var(--bg-primary)] w-full max-w-md rounded-t-3xl overflow-hidden animate-slide-up shadow-2xl"
        on:click|stopPropagation
      >
        <!-- 핸들 바 -->
        <div class="flex justify-center pt-3 pb-1">
          <div class="w-10 h-1 bg-[#2d3a3a] rounded-full"></div>
        </div>

        <!-- 타이틀 -->
        <div class="px-5 pb-3 border-b border-[var(--border-primary)]">
          <h3 class="text-[var(--text-primary)] font-semibold">{$_('settings.title')}</h3>
        </div>

        <!-- 메뉴 항목들 -->
        <div class="py-2">
          <!-- 전체 대화 복사 -->
          <button
            class="w-full flex items-center gap-4 px-5 py-4 text-left transition-all duration-150 hover:bg-[var(--bg-hover)]"
            on:click={() => { copyAllMessages(); showSettingsMenu = false; }}
          >
            <div class="w-10 h-10 rounded-xl bg-[var(--accent-primary)]/10 flex items-center justify-center">
              <svg class="w-5 h-5 text-[var(--accent-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
              </svg>
            </div>
            <div class="flex flex-col">
              <span class="text-[var(--text-primary)] text-base font-medium">{$_('chat.copy_all')}</span>
              <span class="text-[var(--text-dimmed)] text-xs">{$_('settings.copy_all_desc')}</span>
            </div>
          </button>

          <!-- 서버 로그 -->
          <button
            class="w-full flex items-center gap-4 px-5 py-4 text-left transition-all duration-150 hover:bg-[var(--bg-hover)]"
            on:click={() => { onLogs(); showSettingsMenu = false; }}
          >
            <div class="w-10 h-10 rounded-xl bg-[var(--purple-primary)]/10 flex items-center justify-center">
              <svg class="w-5 h-5 text-[var(--purple-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
              </svg>
            </div>
            <div class="flex flex-col">
              <span class="text-[var(--text-primary)] text-base font-medium">{$_('settings.server_logs')}</span>
              <span class="text-[var(--text-dimmed)] text-xs">{$_('settings.server_logs_desc')}</span>
            </div>
          </button>

          <!-- 서버 재시작 -->
          <button
            class="w-full flex items-center gap-4 px-5 py-4 text-left transition-all duration-150 hover:bg-[var(--bg-hover)]"
            on:click={() => { restartServer(); showSettingsMenu = false; }}
            disabled={isRestarting}
          >
            <div class="w-10 h-10 rounded-xl bg-[var(--orange-primary)]/10 flex items-center justify-center">
              <svg class="w-5 h-5 text-[var(--orange-primary)] {isRestarting ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
              </svg>
            </div>
            <div class="flex flex-col">
              <span class="text-[var(--text-primary)] text-base font-medium">{isRestarting ? $_('settings.restarting') : $_('settings.restart_server')}</span>
              <span class="text-[var(--text-dimmed)] text-xs">{$_('settings.restart_server_desc')}</span>
            </div>
          </button>
        </div>

        <!-- 취소 버튼 -->
        <div class="px-4 pb-[calc(1rem+env(safe-area-inset-bottom))]">
          <button
            class="w-full py-3.5 bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] rounded-xl text-[var(--text-tertiary)] font-medium transition-all"
            on:click={() => showSettingsMenu = false}
          >
            {$_('action.cancel')}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* 슬라이드 업 애니메이션 */
  .animate-slide-up {
    animation: slideUp 0.2s ease-out;
  }

  @keyframes slideUp {
    from {
      transform: translateY(100%);
    }
    to {
      transform: translateY(0);
    }
  }

  /* 텍스트 줄 제한 */
  .line-clamp-2 {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  /* iOS Safari 기본 터치 동작 비활성화 */
  .touch-callout-none {
    -webkit-touch-callout: none;
    -webkit-user-select: none;
    user-select: none;
  }
  /* Rico 스타일 prose - CSS 변수 적용 */
  :global(.prose-rico) {
    color: var(--text-secondary);
    word-break: break-word;
    overflow-wrap: break-word;
    font-size: 15px;
    line-height: 1.7;
  }

  /* 코드블록 - CSS 변수 적용 */
  :global(.prose-rico pre) {
    background-color: var(--code-bg);
    border: 1px solid var(--border-primary);
    border-radius: 12px;
    padding: 0;
    overflow-x: auto;
    overflow-y: hidden;
    margin: 12px 0;
    max-width: 100%;
  }

  :global(.prose-rico pre code) {
    background: none;
    padding: 14px 16px;
    color: var(--code-text);
    white-space: pre;
    display: block;
    word-break: normal;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pan-y;
    font-size: 13px;
    line-height: 1.6;
    font-family: 'SF Mono', 'Fira Code', 'JetBrains Mono', Consolas, monospace;
  }

  /* 인라인 코드 */
  :global(.prose-rico code) {
    background-color: var(--code-inline-bg);
    padding: 2px 7px;
    border-radius: 6px;
    font-size: 13px;
    color: var(--code-inline-text);
    word-break: break-all;
    font-family: 'SF Mono', 'Fira Code', 'JetBrains Mono', Consolas, monospace;
  }

  :global(.prose-rico pre code) {
    background: none;
    padding: 14px 16px;
    color: var(--code-text);
    border-radius: 0;
  }

  /* 문단 */
  :global(.prose-rico p) {
    margin: 10px 0;
  }

  :global(.prose-rico p:first-child) {
    margin-top: 0;
  }

  :global(.prose-rico p:last-child) {
    margin-bottom: 0;
  }

  /* 리스트 */
  :global(.prose-rico ul, .prose-rico ol) {
    margin: 12px 0;
    padding-left: 24px;
  }

  :global(.prose-rico ul) {
    list-style-type: disc;
  }

  :global(.prose-rico ol) {
    list-style-type: decimal;
  }

  :global(.prose-rico li) {
    margin: 5px 0;
    display: list-item;
  }

  :global(.prose-rico li::marker) {
    color: var(--accent-primary);
  }

  /* 링크 */
  :global(.prose-rico a) {
    color: var(--accent-primary);
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: border-color 0.15s;
  }

  :global(.prose-rico a:hover) {
    border-bottom-color: var(--accent-primary);
  }

  /* 강조 */
  :global(.prose-rico strong) {
    color: var(--text-primary);
    font-weight: 600;
  }

  :global(.prose-rico em) {
    color: var(--text-tertiary);
  }

  /* 인용문 */
  :global(.prose-rico blockquote) {
    border-left: 3px solid var(--accent-primary);
    padding-left: 14px;
    margin: 14px 0;
    color: var(--text-muted);
    font-style: italic;
  }

  /* 제목 */
  :global(.prose-rico h1, .prose-rico h2, .prose-rico h3, .prose-rico h4) {
    color: var(--text-primary);
    margin: 18px 0 10px 0;
    font-weight: 600;
    line-height: 1.4;
  }

  :global(.prose-rico h1) {
    font-size: 1.4rem;
  }

  :global(.prose-rico h2) {
    font-size: 1.2rem;
  }

  :global(.prose-rico h3) {
    font-size: 1.05rem;
  }

  :global(.prose-rico h4) {
    font-size: 1rem;
  }

  /* 테이블 */
  :global(.prose-rico table) {
    width: 100%;
    margin: 14px 0;
    border-collapse: collapse;
    font-size: 14px;
  }

  :global(.prose-rico th, .prose-rico td) {
    border: 1px solid var(--border-primary);
    padding: 10px 12px;
    text-align: left;
  }

  :global(.prose-rico th) {
    background-color: var(--bg-tertiary);
    color: var(--text-primary);
    font-weight: 600;
  }

  :global(.prose-rico tr:nth-child(even)) {
    background-color: var(--bg-secondary);
  }

  /* 구분선 */
  :global(.prose-rico hr) {
    border: none;
    border-top: 1px solid var(--border-primary);
    margin: 18px 0;
  }

  /* 코드블록 복사 버튼 wrapper */
  :global(.prose-rico .code-block-wrapper) {
    position: relative;
  }

  :global(.prose-rico .code-block-wrapper .copy-btn) {
    position: absolute;
    top: 8px;
    right: 8px;
    padding: 6px 8px;
    background-color: var(--bg-tertiary);
    border: 1px solid var(--border-primary);
    border-radius: 6px;
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    opacity: 0.7;
    transition: all 0.15s;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  :global(.prose-rico .code-block-wrapper:hover .copy-btn) {
    opacity: 1;
  }

  /* 모바일에서는 항상 보이게 */
  @media (hover: none) {
    :global(.prose-rico .code-block-wrapper .copy-btn) {
      opacity: 1;
    }
  }

  :global(.prose-rico .code-block-wrapper .copy-btn:hover) {
    background-color: var(--border-secondary);
    color: var(--text-secondary);
  }

  :global(.prose-rico .code-block-wrapper .copy-btn:active) {
    transform: scale(0.95);
  }

  /* 타이핑 점 애니메이션 */
  .typing-dots {
    display: inline-flex;
    color: var(--accent-primary);
  }

  .typing-dots .dot {
    animation: typing-bounce 1.4s infinite ease-in-out;
  }

  .typing-dots .dot:nth-child(1) {
    animation-delay: 0s;
  }

  .typing-dots .dot:nth-child(2) {
    animation-delay: 0.2s;
  }

  .typing-dots .dot:nth-child(3) {
    animation-delay: 0.4s;
  }

  @keyframes typing-bounce {
    0%, 60%, 100% {
      opacity: 0.3;
    }
    30% {
      opacity: 1;
    }
  }
</style>
