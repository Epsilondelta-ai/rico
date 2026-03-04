<script lang="ts">
  import { onMount } from 'svelte';
  import { _, locale } from 'svelte-i18n';
  import { setLocale } from './i18n';

  interface Session {
    id: string;
    title: string;
    lastMessage?: string;
    updatedAt: number;
    messageCount?: number;
  }

  export let sessions: Session[] = [];
  export let onSelectSession: (sessionId: string) => void = () => {};
  export let onNewSession: () => void = () => {};
  export let onRefresh: () => void = () => {};
  export let onDeleteSession: (sessionId: string) => void = () => {};
  export let onEnablePush: () => Promise<void> = async () => {};
  export let onSoulSettings: () => void = () => {};
  export let onToggleTheme: () => void = () => {};
  export let theme: 'dark' | 'light' = 'dark';

  let editMode = false;
  let pushEnabled = false;
  let pushLoading = false;
  let showSettings = false;

  const READ_COUNT_KEY = 'rico_read_counts';

  // 안 읽은 메시지 수 계산
  function getUnreadCount(session: Session): number {
    if (!session.messageCount) return 0;
    const readCounts = JSON.parse(localStorage.getItem(READ_COUNT_KEY) || '{}');
    const lastRead = readCounts[session.id] || 0;
    return Math.max(0, session.messageCount - lastRead);
  }

  // 세션 선택 시 읽음 처리
  function handleSelect(sessionId: string) {
    const session = sessions.find(s => s.id === sessionId);
    if (session && session.messageCount) {
      const readCounts = JSON.parse(localStorage.getItem(READ_COUNT_KEY) || '{}');
      readCounts[sessionId] = session.messageCount;
      localStorage.setItem(READ_COUNT_KEY, JSON.stringify(readCounts));
    }
    onSelectSession(sessionId);
  }

  onMount(() => {
    // 이미 알림 권한이 있는지 확인
    if ('Notification' in window && Notification.permission === 'granted') {
      pushEnabled = true;
    }
  });

  async function handleEnablePush() {
    pushLoading = true;
    try {
      await onEnablePush();
      if ('Notification' in window && Notification.permission === 'granted') {
        pushEnabled = true;
      }
    } finally {
      pushLoading = false;
    }
  }

  function formatDate(timestamp: number): string {
    const date = new Date(timestamp);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));
    const currentLocale = $locale || 'en';
    const localeCode = currentLocale.startsWith('ko') ? 'ko-KR' : 'en-US';

    if (diffDays === 0) {
      return date.toLocaleTimeString(localeCode, { hour: '2-digit', minute: '2-digit' });
    } else if (diffDays === 1) {
      return $_('time.yesterday');
    } else if (diffDays < 7) {
      return $_('time.days_ago', { values: { days: diffDays } });
    } else {
      return date.toLocaleDateString(localeCode, { month: 'short', day: 'numeric' });
    }
  }

  // 설정 메뉴 외부 클릭 시 닫기
  function handleOutsideClick() {
    if (showSettings) {
      showSettings = false;
    }
  }
</script>

<!-- Rico 스타일 세션 목록 - CSS 변수 적용 -->
<div class="flex flex-col h-[100dvh] bg-[var(--bg-primary)]" on:click={handleOutsideClick}>
  <!-- 헤더 -->
  <div class="flex items-center justify-between px-3 py-2.5 pt-[calc(0.625rem+env(safe-area-inset-top))] bg-[var(--bg-primary)] border-b border-[var(--border-primary)]/50">
    <div class="flex flex-col">
      <h1 class="text-[var(--text-primary)] font-semibold text-base leading-tight">{$_('app.name')}</h1>
      <span class="text-[var(--accent-primary)] text-xs">{$_('session.count', { values: { count: sessions.length } })}</span>
    </div>
    <button
      class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all {editMode ? 'bg-[var(--accent-primary)] text-[var(--bg-primary)]' : 'text-[var(--text-dimmed)] hover:text-[var(--text-muted)] hover:bg-[var(--border-primary)]'}"
      on:click|stopPropagation={() => editMode = !editMode}
    >
      {editMode ? $_('session.done') : $_('session.edit')}
    </button>
  </div>

  <!-- 새 세션 버튼 영역 -->
  <div class="px-3 py-3">
    <div class="flex gap-2">
      <button
        class="flex-1 flex items-center justify-center gap-2 bg-[var(--accent-primary)] hover:bg-[var(--accent-primary-hover)] py-3.5 rounded-xl text-[var(--bg-primary)] font-semibold transition-all shadow-lg" style="box-shadow: 0 4px 12px var(--accent-primary-shadow);"
        on:click={onNewSession}
        type="button"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4"/>
        </svg>
        {$_('session.new')}
      </button>
      <button
        class="w-12 h-12 flex items-center justify-center bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] rounded-xl text-[var(--text-dimmed)] hover:text-[var(--text-muted)] transition-all border border-[var(--border-primary)]"
        on:click={onRefresh}
        type="button"
        title={$_('session.refresh')}
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- 세션 목록 -->
  <div class="flex-1 overflow-y-auto px-3">
    {#if sessions.length === 0}
      <div class="flex flex-col items-center justify-center h-64 text-[var(--text-faint)]">
        <div class="w-16 h-16 rounded-2xl bg-[var(--bg-tertiary)] flex items-center justify-center mb-4">
          <svg class="w-8 h-8 text-[var(--text-faint)]" fill="currentColor" viewBox="0 0 24 24">
            <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z"/>
          </svg>
        </div>
        <p class="font-medium text-[var(--text-dimmed)]">{$_('session.empty')}</p>
        <p class="text-sm mt-1 text-[var(--text-faint)]">{$_('session.empty_hint')}</p>
      </div>
    {:else}
      <div class="space-y-1.5">
        {#each sessions as session (session.id)}
          <div class="flex items-center rounded-2xl hover:bg-[var(--bg-hover)] group transition-all {getUnreadCount(session) > 0 ? 'bg-[var(--bg-secondary)]' : ''}">
            {#if editMode}
              <button
                class="pl-3 py-3 text-[var(--red-primary)] hover:opacity-80 transition-colors"
                on:click|stopPropagation={() => onDeleteSession(session.id)}
              >
                <div class="w-7 h-7 rounded-full bg-[var(--red-bg)] flex items-center justify-center">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4"/>
                  </svg>
                </div>
              </button>
            {/if}
            <button
              class="flex-1 flex items-center gap-3 px-3 py-3 text-left min-w-0 overflow-hidden"
              on:click={() => !editMode && handleSelect(session.id)}
            >
              <!-- 세션 아이콘 -->
              <div class="relative flex-shrink-0">
                <div class="w-11 h-11 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-primary)] flex items-center justify-center {getUnreadCount(session) > 0 ? 'border-[var(--accent-primary)]/50' : ''}">
                  <svg class="w-5 h-5 text-[var(--accent-primary)]" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z"/>
                  </svg>
                </div>
                {#if getUnreadCount(session) > 0}
                  <div class="absolute -top-1 -right-1 min-w-[20px] h-[20px] rounded-full bg-[var(--accent-primary)] flex items-center justify-center px-1.5 shadow-lg" style="box-shadow: 0 2px 8px var(--accent-primary-shadow);">
                    <span class="text-[var(--bg-primary)] text-[10px] font-bold">{getUnreadCount(session) > 99 ? '99+' : getUnreadCount(session)}</span>
                  </div>
                {/if}
              </div>

              <!-- 세션 정보 -->
              <div class="flex-1 min-w-0 overflow-hidden">
                <div class="flex items-center justify-between gap-2">
                  <span class="text-[15px] truncate flex-1 min-w-0 transition-colors {getUnreadCount(session) > 0 ? 'font-semibold text-[var(--text-primary)]' : 'text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]'}">
                    {session.title}
                  </span>
                  <span class="text-[var(--text-faint)] text-xs flex-shrink-0">
                    {formatDate(session.updatedAt)}
                  </span>
                </div>
                {#if session.lastMessage}
                  <p class="text-[13px] truncate mt-0.5 {getUnreadCount(session) > 0 ? 'text-[var(--text-dimmed)]' : 'text-[var(--text-faint)]'}">{session.lastMessage}</p>
                {/if}
              </div>

              <!-- 화살표 -->
              {#if !editMode}
                <svg class="w-4 h-4 text-[var(--text-faint)] group-hover:text-[var(--text-dimmed)] flex-shrink-0 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
                </svg>
              {/if}
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- 하단 사용자 영역 -->
  <div class="flex items-center gap-3 px-3 py-3 pb-[calc(1.5rem+env(safe-area-inset-bottom))] bg-[var(--bg-primary)] border-t border-[var(--border-primary)]/50">
    <div class="w-10 h-10 rounded-full bg-gradient-to-br from-[var(--purple-secondary)] to-[var(--purple-dark)] flex items-center justify-center text-white text-sm font-bold shadow-lg" style="box-shadow: 0 4px 12px var(--purple-shadow);">
      U
    </div>
    <div class="flex-1 min-w-0">
      <p class="text-[var(--text-primary)] text-sm font-medium truncate">{$_('user.name')}</p>
      <div class="flex items-center gap-1.5">
        <span class="w-2 h-2 rounded-full bg-[var(--accent-primary)]" style="box-shadow: 0 0 4px var(--accent-primary-shadow);"></span>
        <p class="text-[var(--text-dimmed)] text-xs">{$_('user.online')}</p>
      </div>
    </div>

    <!-- 설정 버튼 -->
    <div class="relative">
      <button
        class="w-10 h-10 rounded-xl flex items-center justify-center text-[var(--text-dimmed)] hover:text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)] transition-all"
        on:click|stopPropagation={() => showSettings = !showSettings}
        title={$_('settings.title')}
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
        </svg>
      </button>

      <!-- 설정 메뉴 -->
      {#if showSettings}
        <div class="absolute bottom-full right-0 mb-2 bg-[var(--bg-tertiary)] rounded-2xl border border-[var(--border-primary)] shadow-2xl overflow-hidden min-w-[180px] animate-slide-up" style="box-shadow: var(--shadow-lg);">
          <!-- 테마 토글 -->
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 text-left hover:bg-[var(--bg-hover)] transition-colors"
            on:click|stopPropagation={() => { onToggleTheme(); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[var(--orange-bg)] flex items-center justify-center">
              {#if theme === 'dark'}
                <svg class="w-4 h-4 text-[var(--orange-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
                </svg>
              {:else}
                <svg class="w-4 h-4 text-[var(--orange-primary)]" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"/>
                </svg>
              {/if}
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{theme === 'dark' ? $_('settings.theme_light') : $_('settings.theme_dark')}</span>
          </button>

          <!-- SOUL 설정 -->
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 text-left hover:bg-[var(--bg-hover)] transition-colors border-t border-[var(--border-primary)]"
            on:click|stopPropagation={() => { showSettings = false; onSoulSettings(); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[var(--purple-primary)]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--purple-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{$_('settings.soul')}</span>
          </button>

          <!-- 알림 -->
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 text-left hover:bg-[var(--bg-hover)] transition-colors border-t border-[var(--border-primary)]"
            on:click|stopPropagation={() => { showSettings = false; handleEnablePush(); }}
            disabled={pushLoading || pushEnabled}
          >
            <div class="w-8 h-8 rounded-lg {pushEnabled ? 'bg-[var(--accent-primary)]/10' : 'bg-[var(--orange-bg)]'} flex items-center justify-center">
              <svg class="w-4 h-4 {pushEnabled ? 'text-[var(--accent-primary)]' : 'text-[var(--orange-primary)]'}" fill="{pushEnabled ? 'currentColor' : 'none'}" stroke="currentColor" viewBox="0 0 24 24">
                {#if pushEnabled}
                  <path d="M12 22c1.1 0 2-.9 2-2h-4c0 1.1.9 2 2 2zm6-6v-5c0-3.07-1.63-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.64 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z"/>
                {:else}
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
                {/if}
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{pushEnabled ? $_('settings.push_on') : $_('settings.push_off')}</span>
          </button>

          <!-- 언어 전환 -->
          <button
            class="w-full flex items-center gap-3 px-4 py-3.5 text-left hover:bg-[var(--bg-hover)] transition-colors border-t border-[var(--border-primary)]"
            on:click|stopPropagation={() => { setLocale($locale?.startsWith('ko') ? 'en' : 'ko'); }}
          >
            <div class="w-8 h-8 rounded-lg bg-[var(--accent-primary)]/10 flex items-center justify-center">
              <svg class="w-4 h-4 text-[var(--accent-primary)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 5h12M9 3v2m1.048 9.5A18.022 18.022 0 016.412 9m6.088 9h7M11 21l5-10 5 10M12.751 5C11.783 10.77 8.07 15.61 3 18.129"/>
              </svg>
            </div>
            <span class="text-[var(--text-secondary)] text-sm font-medium">{$locale?.startsWith('ko') ? 'English' : '한국어'}</span>
          </button>

        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  /* 슬라이드 업 애니메이션 */
  .animate-slide-up {
    animation: slideUp 0.15s ease-out;
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
