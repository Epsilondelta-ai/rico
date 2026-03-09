<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { API_BASE } from './config';

  export let onBack: () => void;

  interface LogEntry {
    timestamp: number;
    level: string;
    message: string;
    source: string;
  }

  const WS_URL = API_BASE.replace('https://', 'wss://').replace('http://', 'ws://') + '/ws/logs';

  let logs: LogEntry[] = [];
  let ws: WebSocket | null = null;
  let autoScroll = true;
  let logContainer: HTMLDivElement;
  let filterSource = 'all';
  let filterLevel = 'all';
  let isConnected = false;

  $: filteredLogs = logs.filter(log => {
    if (filterSource !== 'all' && log.source !== filterSource) return false;
    if (filterLevel !== 'all' && log.level !== filterLevel) return false;
    return true;
  });

  function formatTime(timestamp: number): string {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false });
  }

  function getLevelClass(level: string): string {
    switch (level) {
      case 'error': return 'log-error';
      case 'warn': return 'log-warn';
      default: return 'log-info';
    }
  }

  function getSourceIcon(source: string): string {
    switch (source) {
      case 'go': return '🔵';
      case 'vite': return '⚡';
      case 'stt': return '🎤';
      default: return '📋';
    }
  }

  let isMounted = true;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  // 로그 배치 처리 (UI blocking 방지)
  let pendingLogs: LogEntry[] = [];
  let flushTimer: ReturnType<typeof setTimeout> | null = null;

  function flushLogs() {
    if (pendingLogs.length === 0) return;

    logs = [...logs, ...pendingLogs];
    pendingLogs = [];

    // 최대 500개 유지
    if (logs.length > 500) {
      logs = logs.slice(-500);
    }

    // 자동 스크롤
    if (autoScroll && logContainer) {
      requestAnimationFrame(() => {
        if (logContainer) {
          logContainer.scrollTop = logContainer.scrollHeight;
        }
      });
    }
  }

  function queueLog(log: LogEntry) {
    pendingLogs.push(log);
    // 100ms마다 배치 처리 (초당 최대 10번 UI 업데이트)
    if (!flushTimer) {
      flushTimer = setTimeout(() => {
        flushTimer = null;
        flushLogs();
      }, 100);
    }
  }

  function connectWebSocket() {
    if (!isMounted) return;

    try {
      ws = new WebSocket(WS_URL);
    } catch (err) {
      console.error('Log WebSocket creation failed:', err);
      return;
    }

    ws.onopen = () => {
      isConnected = true;
      console.log('Log WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const log: LogEntry = JSON.parse(event.data);
        queueLog(log); // 배치 처리로 UI blocking 방지
      } catch (err) {
        console.error('Log parsing failed:', err);
      }
    };

    ws.onclose = () => {
      isConnected = false;
      console.log('Log WebSocket disconnected');
      // Reconnect only if mounted
      if (isMounted) {
        reconnectTimer = setTimeout(connectWebSocket, 3000);
      }
    };

    ws.onerror = (err) => {
      console.error('Log WebSocket error:', err);
    };
  }

  function clearLogs() {
    logs = [];
  }

  function handleScroll() {
    if (!logContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainer;
    // Enable auto scroll if within 50px of bottom
    autoScroll = scrollHeight - scrollTop - clientHeight < 50;
  }

  onMount(() => {
    isMounted = true;
    connectWebSocket();
  });

  onDestroy(() => {
    isMounted = false;
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
    }
    if (flushTimer) {
      clearTimeout(flushTimer);
    }
    if (ws) {
      ws.close();
    }
  });
</script>

<div class="log-viewer">
  <header class="header">
    <button class="back-btn" on:click|stopPropagation={onBack} on:touchend|preventDefault={onBack}>
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M15 19l-7-7 7-7"/>
      </svg>
      {$_('chat.back')}
    </button>
    <h1>{$_('logs.title')}</h1>
    <div class="status" class:connected={isConnected}>
      {isConnected ? '🟢' : '🔴'}
    </div>
  </header>

  <div class="filters">
    <select bind:value={filterSource}>
      <option value="all">{$_('logs.all_sources')}</option>
      <option value="go">{$_('logs.source_go')}</option>
      <option value="vite">{$_('logs.source_vite')}</option>
      <option value="stt">{$_('logs.source_stt')}</option>
      <option value="system">{$_('logs.source_system')}</option>
    </select>
    <select bind:value={filterLevel}>
      <option value="all">{$_('logs.all_levels')}</option>
      <option value="info">Info</option>
      <option value="warn">Warning</option>
      <option value="error">Error</option>
    </select>
    <button class="clear-btn" on:click={clearLogs}>
      {$_('logs.clear')}
    </button>
  </div>

  <div class="log-container" bind:this={logContainer} on:scroll={handleScroll}>
    {#each filteredLogs as log, i (log.timestamp + '_' + i)}
      <div class="log-entry {getLevelClass(log.level)}">
        <span class="log-time">{formatTime(log.timestamp)}</span>
        <span class="log-source">{getSourceIcon(log.source)}</span>
        <span class="log-message">{log.message}</span>
      </div>
    {/each}
    {#if filteredLogs.length === 0}
      <div class="empty-state">
        {$_('logs.empty')}
      </div>
    {/if}
  </div>

  <div class="footer">
    <label class="auto-scroll">
      <input type="checkbox" bind:checked={autoScroll} />
      {$_('logs.auto_scroll')}
    </label>
    <span class="log-count">{filteredLogs.length} {$_('logs.entries')}</span>
  </div>
</div>

<style>
  .log-viewer {
    display: flex;
    flex-direction: column;
    height: 100vh;
    height: 100dvh;
    background: var(--bg-primary);
    color: var(--text-primary);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    padding-top: calc(12px + env(safe-area-inset-top));
    border-bottom: 1px solid var(--border-primary);
    background: var(--bg-secondary);
  }

  .header h1 {
    flex: 1;
    font-size: 1.1rem;
    font-weight: 600;
    margin: 0;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    background: none;
    border: none;
    color: var(--accent-primary);
    font-size: 1rem;
    cursor: pointer;
    padding: 8px 12px;
    margin: -8px;
    margin-right: 0;
    border-radius: 8px;
    -webkit-tap-highlight-color: transparent;
  }

  .back-btn:active {
    background: var(--bg-hover);
  }

  .back-btn svg {
    width: 20px;
    height: 20px;
  }

  .status {
    font-size: 0.8rem;
  }

  .filters {
    display: flex;
    gap: 8px;
    padding: 8px 16px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-primary);
  }

  .filters select {
    flex: 1;
    padding: 6px 8px;
    border: 1px solid var(--border-primary);
    border-radius: 6px;
    background: var(--bg-primary);
    color: var(--text-primary);
    font-size: 0.9rem;
  }

  .clear-btn {
    padding: 6px 12px;
    border: 1px solid var(--border-primary);
    border-radius: 6px;
    background: var(--bg-primary);
    color: var(--text-secondary);
    font-size: 0.9rem;
    cursor: pointer;
  }

  .clear-btn:active {
    background: var(--bg-secondary);
  }

  .log-container {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
    font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
    font-size: 0.75rem;
    line-height: 1.4;
  }

  .log-entry {
    display: flex;
    gap: 6px;
    padding: 4px 6px;
    border-radius: 4px;
    margin-bottom: 2px;
  }

  .log-entry:hover {
    background: var(--bg-secondary);
  }

  .log-time {
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .log-source {
    flex-shrink: 0;
  }

  .log-message {
    word-break: break-all;
    white-space: pre-wrap;
  }

  .log-info .log-message {
    color: var(--text-primary);
  }

  .log-warn {
    background: rgba(255, 193, 7, 0.1);
  }

  .log-warn .log-message {
    color: #ffc107;
  }

  .log-error {
    background: rgba(244, 67, 54, 0.1);
  }

  .log-error .log-message {
    color: #f44336;
  }

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary);
  }

  .footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 16px;
    border-top: 1px solid var(--border-primary);
    background: var(--bg-secondary);
    font-size: 0.85rem;
  }

  .auto-scroll {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
  }

  .auto-scroll input {
    accent-color: var(--accent-primary);
  }

  .log-count {
    color: var(--text-secondary);
  }
</style>
