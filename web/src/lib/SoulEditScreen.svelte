<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { API_BASE } from './config';

  export let fileName: string; // 실제로는 folderName으로 사용됨
  export let soulName: string;
  export let onBack: () => void;
  export let onApply: () => void;

  // fileName을 folderName으로 사용
  $: folderName = fileName;

  let content = '';
  let originalContent = '';
  let isLoading = true;
  let isSaving = false;
  let isModifying = false;
  let copied = false;
  let modifyRequest = '';
  let error = '';

  $: hasChanges = content !== originalContent;

  onMount(async () => {
    await loadSoul();
  });

  async function loadSoul() {
    try {
      isLoading = true;
      // 새 personas 구조: /api/soul을 사용하면 활성 페르소나의 SOUL.md를 가져옴
      // 특정 페르소나의 SOUL을 편집하려면 해당 페르소나를 먼저 활성화하거나 직접 경로 접근 필요
      // 여기서는 일단 현재 활성 페르소나의 SOUL을 로드
      const res = await fetch(`${API_BASE}/api/soul`);
      if (res.ok) {
        const data = await res.json();
        content = data.content;
        originalContent = data.content;
      }
    } catch (err) {
      console.error('SOUL 로드 실패:', err);
    } finally {
      isLoading = false;
    }
  }

  async function applySoul() {
    try {
      // 페르소나 변경
      const res = await fetch(`${API_BASE}/api/persona/active`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ persona: folderName })
      });
      if (res.ok) {
        alert($_<string>('soul.applied'));
        onApply();
      }
    } catch (err) {
      console.error('apply failed:', err);
    }
  }

  async function copyContent() {
    try {
      await navigator.clipboard.writeText(content);
      copied = true;
      setTimeout(() => { copied = false; }, 2000);
    } catch (err) {
      console.error('복사 실패:', err);
    }
  }

  async function requestModify() {
    if (!modifyRequest.trim() || !content) return;

    error = '';
    isModifying = true;

    try {
      const res = await fetch(`${API_BASE}/api/soul/modify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          currentSoul: content,
          request: modifyRequest.trim()
        })
      });

      if (res.ok) {
        await pollModifyStatus();
      } else {
        error = $_<string>('generate.error_modify');
      }
    } catch (err) {
      error = $_<string>('generate.error_modify_process');
    } finally {
      isModifying = false;
      modifyRequest = '';
    }
  }

  async function pollModifyStatus() {
    const maxAttempts = 300;
    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(r => setTimeout(r, 1000));
      try {
        const res = await fetch(`${API_BASE}/api/soul/generate/status`);
        if (res.ok) {
          const data = await res.json();
          if (data.status === 'done') {
            content = data.content;
            // 파일에도 저장
            await fetch(`${API_BASE}/api/souls/${encodeURIComponent(fileName)}`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ content: data.content })
            });
            originalContent = data.content;
            isModifying = false;
            return;
          } else if (data.status === 'error') {
            error = data.error || $_<string>('generate.error_modify');
            isModifying = false;
            return;
          }
        }
      } catch (err) {
        // 네트워크 오류 시 계속 시도
      }
    }
    error = $_<string>('generate.error_modify_timeout');
    isModifying = false;
  }
</script>

<div class="flex flex-col h-[100dvh] bg-[#1e2626]">
  <!-- 헤더 -->
  <div class="flex items-center justify-between px-4 py-3 pt-[calc(1rem+env(safe-area-inset-top))] bg-[#2a3636] border-b border-[#3d4f4f]">
    <div class="flex items-center gap-3">
      <button
        class="text-[#7eb8b8] hover:text-[#a8e6e6] p-1 transition-colors"
        on:click={onBack}
        title="뒤로가기"
      >
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <h1 class="text-[#e0f0f0] font-semibold text-lg">{$_('soul.title')}</h1>
    </div>
  </div>

  {#if isLoading}
    <div class="flex-1 flex items-center justify-center">
      <div class="w-12 h-12 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center animate-pulse">
        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
        </svg>
      </div>
    </div>

  {:else if isModifying}
    <!-- 수정 중 로딩 화면 -->
    <div class="flex-1 flex flex-col items-center justify-center p-6 relative overflow-hidden">
      <div class="absolute inset-0 flex items-center justify-center pointer-events-none">
        <div class="w-64 h-64 bg-[#a78bfa]/20 rounded-full blur-3xl animate-pulse"></div>
      </div>

      <div class="relative mb-10">
        <div class="absolute inset-[-16px] w-[calc(100%+32px)] h-[calc(100%+32px)] border-2 border-[#a78bfa]/20 rounded-full animate-spin" style="animation-duration: 8s;"></div>
        <div class="absolute inset-[-8px] w-[calc(100%+16px)] h-[calc(100%+16px)] border-2 border-dashed border-[#7c3aed]/30 rounded-full animate-spin" style="animation-duration: 6s; animation-direction: reverse;"></div>

        <div class="relative w-28 h-28 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center shadow-2xl shadow-[#a78bfa]/30">
          <div class="absolute inset-0 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] animate-ping opacity-20"></div>
          <svg class="w-12 h-12 text-white relative z-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
          </svg>
        </div>
      </div>

      <h2 class="text-[#e0f0f0] text-2xl font-bold mb-3">{$_('soul.modifying')}</h2>
      <p class="text-[#7eb8b8]">{$_('soul.wait')}</p>

      <div class="flex items-center mt-8 px-4 py-2 rounded-full bg-[#2a3636]/80 border border-[#3d4f4f]">
        <div class="flex gap-1">
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce" style="animation-delay: 0s;"></div>
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce" style="animation-delay: 0.2s;"></div>
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce" style="animation-delay: 0.4s;"></div>
        </div>
      </div>
    </div>

  {:else}
    <!-- SOUL 결과 화면 (SoulScreen의 generatedContent 화면과 동일) -->
    <div class="flex-1 overflow-y-auto p-4">
      <!-- 성공 배너 -->
      <div class="flex items-center gap-3 p-4 rounded-2xl bg-gradient-to-r from-[#4fd1c5]/20 to-[#38b2ac]/20 border border-[#4fd1c5]/30 mb-4">
        <div class="w-10 h-10 rounded-full bg-[#4fd1c5]/20 flex items-center justify-center flex-shrink-0">
          <svg class="w-5 h-5 text-[#4fd1c5]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
          </svg>
        </div>
        <div>
          <p class="text-[#4fd1c5] font-medium">{soulName}</p>
          <p class="text-[#7eb8b8] text-sm">{$_('soul.complete_hint')}</p>
        </div>
      </div>

      <!-- SOUL 미리보기 -->
      <div class="relative">
        <pre class="bg-[#0f1515] p-4 rounded-2xl text-[#c8e0e0] text-xs leading-relaxed whitespace-pre-wrap break-words overflow-y-auto border border-[#3d4f4f]" style="max-height: calc(100vh - 380px);">{content}</pre>
        <button
          class="absolute top-2 right-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all {copied ? 'bg-[#4fd1c5] text-[#0f1515]' : 'bg-[#2a3636] text-[#7eb8b8] hover:bg-[#3d4f4f]'}"
          on:click={copyContent}
        >
          {copied ? $_('action.copied') : $_('action.copy')}
        </button>
      </div>

      <!-- 수정 요청 입력창 -->
      <div class="mt-4 p-4 rounded-2xl bg-[#2a3636] border border-[#3d4f4f]">
        <label class="block text-[#e0f0f0] text-sm font-medium mb-3">{$_('soul.modify_hint')}</label>
        <input
          type="text"
          placeholder={$_('soul.modify_placeholder')}
          bind:value={modifyRequest}
          class="w-full px-4 py-3 rounded-xl bg-[#0f1515] border-2 border-[#3d4f4f] text-[#e0f0f0] placeholder-[#5a7a7a] focus:outline-none focus:border-[#a78bfa] transition-colors text-sm"
          on:keypress={(e) => e.key === 'Enter' && modifyRequest.trim() && requestModify()}
        />
      </div>

      {#if error}
        <div class="flex items-center gap-2 p-3 rounded-xl bg-[#f56565]/10 border border-[#f56565]/20 mt-4">
          <svg class="w-5 h-5 text-[#f56565] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
          </svg>
          <p class="text-[#f56565] text-sm">{error}</p>
        </div>
      {/if}
    </div>

    <!-- 하단 버튼: 수정 / 사용 -->
    <div class="p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] bg-gradient-to-t from-[#1a2020] via-[#1a2020] to-transparent">
      <div class="flex gap-3">
        <!-- 수정 요청 버튼 -->
        <button
          class="flex-1 py-4 rounded-2xl bg-gradient-to-r from-[#a78bfa]/90 to-[#7c3aed]/90 text-white font-bold text-base shadow-lg shadow-[#a78bfa]/30 transition-all active:scale-[0.98] disabled:opacity-40 disabled:shadow-none flex items-center justify-center gap-2 border border-[#a78bfa]/30"
          on:click={requestModify}
          disabled={!modifyRequest.trim()}
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
          </svg>
          {$_('soul.request_modify')}
        </button>
        <!-- 사용하기 버튼 -->
        <button
          class="flex-1 py-4 rounded-2xl bg-gradient-to-r from-[#4fd1c5] to-[#38b2ac] text-[#0f1515] font-bold text-base shadow-lg shadow-[#4fd1c5]/30 transition-all active:scale-[0.98] flex items-center justify-center gap-2 border border-[#4fd1c5]/30"
          on:click={applySoul}
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
          </svg>
          {$_('soul.apply')}
        </button>
      </div>
    </div>
  {/if}
</div>
