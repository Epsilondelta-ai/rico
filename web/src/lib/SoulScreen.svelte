<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { API_BASE } from './config';

  export let onBack: () => void;

  let currentSoul = '';
  let isLoading = true;
  let isGenerating = false;
  let mode: 'view' | 'character' | 'custom' = 'view';

  // 캐릭터 검색 모드
  let characterName = '';

  // 직접 입력 모드
  let customName = '';
  let customPersonality = '';
  let customGender = '';

  let generatedContent = '';
  let error = '';

  // 수정 요청 관련
  let modifyRequest = '';
  let isModifying = false;


  // 복사 상태
  let copied = false;

  async function copyGeneratedContent() {
    try {
      await navigator.clipboard.writeText(generatedContent);
      copied = true;
      setTimeout(() => { copied = false; }, 2000);
    } catch (err) {
      console.error('복사 실패:', err);
    }
  }

  // SOUL에서 이름 추출
  $: soulName = extractName(currentSoul);

  function extractName(soul: string): string {
    const match = soul.match(/이름:\s*\*?\*?([^*\n]+)/);
    return match ? match[1].trim() : 'EB';
  }

  onMount(async () => {
    await loadSoul();
    await checkPendingGeneration();
  });

  async function checkPendingGeneration() {
    try {
      const res = await fetch(`${API_BASE}/api/soul/generate/status`);
      if (res.ok) {
        const data = await res.json();
        if (data.status === 'generating') {
          // 생성 중이면 폴링 시작
          isGenerating = true;
          mode = 'character'; // 또는 적절한 모드
          await pollGenerateStatus();
        } else if (data.status === 'done' && data.content) {
          // 완료된 결과가 있으면 표시
          generatedContent = data.content;
        }
      }
    } catch (err) {
      // 무시
    }
  }

  async function loadSoul() {
    try {
      isLoading = true;
      const res = await fetch(`${API_BASE}/api/soul`);
      if (res.ok) {
        const data = await res.json();
        currentSoul = data.content;
      }
    } catch (err) {
      console.error('SOUL 로드 실패:', err);
    } finally {
      isLoading = false;
    }
  }

  async function backupSoul() {
    try {
      const res = await fetch(`${API_BASE}/api/soul/backup`, { method: 'POST' });
      if (res.ok) {
        alert($_<string>('soul.backup_done'));
      }
    } catch (err) {
      console.error('backup failed:', err);
    }
  }

  async function restoreSoul() {
    if (!confirm($_<string>('soul.restore_confirm'))) return;
    try {
      const res = await fetch(`${API_BASE}/api/soul/restore`, { method: 'POST' });
      if (res.ok) {
        await loadSoul();
        alert($_<string>('soul.restore_done'));
      }
    } catch (err) {
      console.error('restore failed:', err);
    }
  }

  async function generateFromCharacter() {
    if (!characterName.trim()) {
      error = $_<string>('character.error_empty');
      return;
    }
    error = '';
    isGenerating = true;
    try {
      // 생성 시작 요청
      const res = await fetch(`${API_BASE}/api/soul/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ characterName: characterName.trim() })
      });
      if (res.ok) {
        // 폴링으로 완료 대기
        await pollGenerateStatus();
      } else {
        error = $_<string>('generate.error_start');
        isGenerating = false;
      }
    } catch (err) {
      error = $_<string>('generate.error');
      isGenerating = false;
    }
  }

  async function pollGenerateStatus() {
    const maxAttempts = 300; // 최대 5분
    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(r => setTimeout(r, 1000)); // 1초 대기
      try {
        const res = await fetch(`${API_BASE}/api/soul/generate/status`);
        if (res.ok) {
          const data = await res.json();
          if (data.status === 'done') {
            generatedContent = data.content;
            isGenerating = false;
            return;
          } else if (data.status === 'error') {
            error = data.error || '생성 실패';
            isGenerating = false;
            return;
          }
          // generating이면 계속 폴링
        }
      } catch (err) {
        // 네트워크 오류 시 계속 시도
      }
    }
    error = $_<string>('generate.error_timeout');
    isGenerating = false;
  }

  async function generateFromCustom() {
    if (!customName.trim()) {
      error = $_<string>('custom.error_empty');
      return;
    }
    error = '';
    isGenerating = true;
    try {
      const res = await fetch(`${API_BASE}/api/soul/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: customName.trim(),
          personality: customPersonality.trim() || 'friendly',
          gender: customGender.trim() || 'neutral'
        })
      });
      if (res.ok) {
        // 폴링으로 완료 대기
        await pollGenerateStatus();
      } else {
        error = $_<string>('generate.error_start');
        isGenerating = false;
      }
    } catch (err) {
      error = $_<string>('generate.error');
      isGenerating = false;
    }
  }

  async function applyGenerated() {
    if (!generatedContent) return;
    try {
      const res = await fetch(`${API_BASE}/api/soul`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: generatedContent })
      });
      if (res.ok) {
        currentSoul = generatedContent;
        generatedContent = '';
        mode = 'view';
        alert($_<string>('soul.applied'));
      }
    } catch (err) {
      console.error('apply failed:', err);
    }
  }

  async function cancelGenerate() {
    // 서버 상태 초기화
    try {
      await fetch(`${API_BASE}/api/soul/generate/reset`, { method: 'POST' });
    } catch (err) {
      // 무시
    }

    generatedContent = '';
    characterName = '';
    customName = '';
    customPersonality = '';
    customGender = '';
    modifyRequest = '';
    error = '';
    mode = 'view';
  }

  async function requestModify() {
    if (!modifyRequest.trim() || !generatedContent) return;

    error = '';
    isModifying = true;

    try {
      const res = await fetch(`${API_BASE}/api/soul/modify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          currentSoul: generatedContent,
          request: modifyRequest.trim()
        })
      });

      if (res.ok) {
        const data = await res.json();
        if (data.status === 'generating') {
          // 폴링으로 완료 대기
          await pollModifyStatus();
        }
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
    const maxAttempts = 300; // 최대 5분
    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(r => setTimeout(r, 1000));
      try {
        const res = await fetch(`${API_BASE}/api/soul/generate/status`);
        if (res.ok) {
          const data = await res.json();
          if (data.status === 'done') {
            generatedContent = data.content;
            isModifying = false;
            return;
          } else if (data.status === 'error') {
            error = data.error || '수정 실패';
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

<!-- Rico 스타일 SOUL 설정 화면 -->
<div class="flex flex-col h-[100dvh] bg-[#1e2626]">
  <!-- 헤더 -->
  <div class="flex items-center justify-between px-4 py-3 pt-[calc(1rem+env(safe-area-inset-top))] bg-[#2a3636] border-b border-[#3d4f4f]">
    <div class="flex items-center gap-3">
      <button
        class="text-[#7eb8b8] hover:text-[#a8e6e6] p-1 transition-colors"
        on:click={() => generatedContent ? cancelGenerate() : onBack()}
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
    <!-- 초기 로딩 -->
    <div class="flex-1 flex flex-col items-center justify-center">
      <div class="w-16 h-16 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center mb-4 animate-pulse">
        <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
        </svg>
      </div>
      <p class="text-[#5a7a7a]">{$_('soul.loading')}</p>
    </div>

  {:else if isGenerating || isModifying}
    <!-- SOUL 생성/수정 전용 로딩 화면 -->
    <div class="flex-1 flex flex-col items-center justify-center p-6 relative overflow-hidden">
      <!-- 배경 글로우 효과 -->
      <div class="absolute inset-0 flex items-center justify-center pointer-events-none">
        <div class="w-64 h-64 bg-[#a78bfa]/20 rounded-full blur-3xl animate-pulse-slow"></div>
      </div>

      <!-- 메인 애니메이션 -->
      <div class="relative mb-10">
        <!-- 외곽 회전 링들 -->
        <div class="absolute inset-[-16px] w-[calc(100%+32px)] h-[calc(100%+32px)] border-2 border-[#a78bfa]/20 rounded-full animate-spin-slow"></div>
        <div class="absolute inset-[-8px] w-[calc(100%+16px)] h-[calc(100%+16px)] border-2 border-dashed border-[#7c3aed]/30 rounded-full animate-spin-reverse"></div>

        <!-- 중앙 원 -->
        <div class="relative w-28 h-28 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center shadow-2xl shadow-[#a78bfa]/30">
          <!-- 펄스 이펙트 -->
          <div class="absolute inset-0 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] animate-ping opacity-20"></div>

          <!-- 아이콘 -->
          <svg class="w-12 h-12 text-white relative z-10 animate-float" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
          </svg>
        </div>

        <!-- 플로팅 파티클 -->
        <div class="absolute -top-2 -right-2 w-3 h-3 bg-[#4fd1c5] rounded-full animate-float-delayed"></div>
        <div class="absolute -bottom-1 -left-3 w-2 h-2 bg-[#f6ad55] rounded-full animate-float"></div>
        <div class="absolute top-1/2 -right-4 w-2 h-2 bg-[#a78bfa] rounded-full animate-float-delayed-2"></div>
      </div>

      <!-- 텍스트 -->
      <h2 class="text-[#e0f0f0] text-2xl font-bold mb-3 relative z-10">
        {#if isModifying}
          {$_('soul.modifying')}
        {:else}
          {$_('soul.generating')}
        {/if}
      </h2>
      <div class="text-center mb-8 relative z-10">
        {#if characterName}
          <p class="text-[#e0f0f0] text-lg font-medium mb-1">"{characterName}"</p>
          <p class="text-[#7eb8b8]">{$_('character.analyzing')}</p>
        {:else if customName}
          <p class="text-[#e0f0f0] text-lg font-medium mb-1">"{customName}"</p>
          <p class="text-[#7eb8b8]">{$_('character.creating')}</p>
        {:else}
          <p class="text-[#7eb8b8]">{$_('soul.wait')}</p>
        {/if}
      </div>

      <!-- 진행 상태 표시 -->
      <div class="flex items-center px-4 py-2 rounded-full bg-[#2a3636]/80 border border-[#3d4f4f] relative z-10">
        <div class="flex gap-1">
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce-1"></div>
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce-2"></div>
          <div class="w-2 h-2 rounded-full bg-[#a78bfa] animate-bounce-3"></div>
        </div>
      </div>

      <!-- 취소 버튼 -->
      <button
        class="mt-10 px-8 py-3 rounded-2xl bg-[#2a3636] text-[#7eb8b8] font-medium border border-[#3d4f4f] transition-all active:scale-[0.98] hover:border-[#5a7a7a] relative z-10"
        on:click={cancelGenerate}
      >
        {$_('action.cancel')}
      </button>
    </div>

  {:else if generatedContent}
    <!-- 생성된 SOUL 결과 확인 화면 -->
    <div class="flex-1 overflow-y-auto p-4">
      <!-- 성공 배너 -->
      <div class="flex items-center gap-3 p-4 rounded-2xl bg-gradient-to-r from-[#4fd1c5]/20 to-[#38b2ac]/20 border border-[#4fd1c5]/30 mb-4">
        <div class="w-10 h-10 rounded-full bg-[#4fd1c5]/20 flex items-center justify-center flex-shrink-0">
          <svg class="w-5 h-5 text-[#4fd1c5]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
          </svg>
        </div>
        <div>
          <p class="text-[#4fd1c5] font-medium">{$_('soul.complete')}</p>
          <p class="text-[#7eb8b8] text-sm">{$_('soul.complete_hint')}</p>
        </div>
      </div>

      <!-- SOUL 미리보기 -->
      <div class="relative">
        <pre class="bg-[#0f1515] p-4 rounded-2xl text-[#c8e0e0] text-xs leading-relaxed whitespace-pre-wrap break-words overflow-y-auto border border-[#3d4f4f]" style="max-height: calc(100vh - 380px);">{generatedContent}</pre>
        <button
          class="absolute top-2 right-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all {copied ? 'bg-[#4fd1c5] text-[#0f1515]' : 'bg-[#2a3636] text-[#7eb8b8] hover:bg-[#3d4f4f]'}"
          on:click={copyGeneratedContent}
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

    <!-- 하단 버튼: 취소 / 수정 / 사용 -->
    <div class="p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] bg-gradient-to-t from-[#1a2020] via-[#1a2020] to-transparent">
      <!-- 메인 액션 버튼들 -->
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
          on:click={applyGenerated}
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
          </svg>
          {$_('soul.apply')}
        </button>
      </div>
    </div>

  {:else if mode === 'view'}
    <!-- 현재 SOUL 보기 -->
    <div class="flex-1 overflow-y-auto">
      <!-- 프로필 카드 -->
      <div class="p-6">
        <div class="relative bg-gradient-to-br from-[#2a3636] to-[#1a2020] rounded-3xl p-6 border border-[#3d4f4f] overflow-hidden">
          <!-- 배경 장식 -->
          <div class="absolute top-0 right-0 w-32 h-32 bg-gradient-to-br from-[#a78bfa]/10 to-transparent rounded-full -translate-y-1/2 translate-x-1/2"></div>
          <div class="absolute bottom-0 left-0 w-24 h-24 bg-gradient-to-tr from-[#4fd1c5]/10 to-transparent rounded-full translate-y-1/2 -translate-x-1/2"></div>

          <!-- 프로필 -->
          <div class="relative flex items-center gap-4 mb-6">
            <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center shadow-lg shadow-[#a78bfa]/20">
              <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
              </svg>
            </div>
            <div>
              <h2 class="text-[#e0f0f0] text-xl font-bold">{soulName}</h2>
              <p class="text-[#7eb8b8] text-sm">{$_('soul.active')}</p>
            </div>
          </div>

          <!-- SOUL 내용 미리보기 -->
          <div class="bg-[#0f1515]/50 rounded-2xl p-4 max-h-[200px] overflow-y-auto">
            <pre class="text-[#c8e0e0] text-xs leading-relaxed whitespace-pre-wrap break-words">{currentSoul}</pre>
          </div>
        </div>
      </div>

      <!-- 액션 버튼들 -->
      <div class="px-6 space-y-3">
        <p class="text-[#5a7a7a] text-xs uppercase tracking-wider font-medium mb-2">{$_('soul.change')}</p>

        <button
          class="w-full flex items-center gap-4 p-4 rounded-2xl bg-[#2a3636] border border-[#3d4f4f] transition-all active:scale-[0.98] hover:border-[#a78bfa]/50 group"
          on:click={() => mode = 'character'}
        >
          <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-[#a78bfa]/20 to-[#7c3aed]/20 flex items-center justify-center group-hover:from-[#a78bfa]/30 group-hover:to-[#7c3aed]/30 transition-colors">
            <svg class="w-6 h-6 text-[#a78bfa]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
            </svg>
          </div>
          <div class="flex-1 text-left">
            <p class="text-[#e0f0f0] font-semibold">{$_('soul.search')}</p>
            <p class="text-[#5a7a7a] text-sm">{$_('soul.search_hint')}</p>
          </div>
          <svg class="w-5 h-5 text-[#5a7a7a] group-hover:text-[#a78bfa] transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
          </svg>
        </button>

        <button
          class="w-full flex items-center gap-4 p-4 rounded-2xl bg-[#2a3636] border border-[#3d4f4f] transition-all active:scale-[0.98] hover:border-[#4fd1c5]/50 group"
          on:click={() => mode = 'custom'}
        >
          <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-[#4fd1c5]/20 to-[#38b2ac]/20 flex items-center justify-center group-hover:from-[#4fd1c5]/30 group-hover:to-[#38b2ac]/30 transition-colors">
            <svg class="w-6 h-6 text-[#4fd1c5]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
            </svg>
          </div>
          <div class="flex-1 text-left">
            <p class="text-[#e0f0f0] font-semibold">{$_('soul.custom')}</p>
            <p class="text-[#5a7a7a] text-sm">{$_('soul.custom_hint')}</p>
          </div>
          <svg class="w-5 h-5 text-[#5a7a7a] group-hover:text-[#4fd1c5] transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
          </svg>
        </button>

        <p class="text-[#5a7a7a] text-xs uppercase tracking-wider font-medium mt-6 mb-2">{$_('soul.manage')}</p>

        <div class="flex gap-3">
          <button
            class="flex-1 flex items-center justify-center gap-2 py-3 rounded-xl bg-[#2a3636] text-[#7eb8b8] text-sm font-medium border border-[#3d4f4f] transition-all active:scale-[0.98]"
            on:click={backupSoul}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"/>
            </svg>
            {$_('soul.backup')}
          </button>
          <button
            class="flex-1 flex items-center justify-center gap-2 py-3 rounded-xl bg-[#2a3636] text-[#f6ad55] text-sm font-medium border border-[#3d4f4f] transition-all active:scale-[0.98]"
            on:click={restoreSoul}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
            </svg>
            {$_('soul.restore')}
          </button>
        </div>
      </div>

      <!-- 하단 여백 -->
      <div class="h-[calc(1rem+env(safe-area-inset-bottom))]"></div>
    </div>

  {:else if mode === 'character'}
    <!-- 캐릭터로 SOUL 생성 -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- 헤더 카드 -->
      <div class="flex items-center gap-4 p-4 rounded-2xl bg-gradient-to-r from-[#a78bfa]/10 to-[#7c3aed]/10 border border-[#a78bfa]/20 mb-6">
        <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center">
          <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
          </svg>
        </div>
        <div>
          <p class="text-[#e0f0f0] font-semibold">{$_('character.title')}</p>
          <p class="text-[#7eb8b8] text-sm">{$_('character.hint')}</p>
        </div>
      </div>

      <div class="space-y-4">
        <div>
          <label class="block text-[#7eb8b8] text-sm font-medium mb-2">{$_('character.name')}</label>
          <input
            type="text"
            placeholder={$_('character.placeholder')}
            bind:value={characterName}
            class="w-full px-4 py-4 rounded-xl bg-[#0f1515] border-2 border-[#3d4f4f] text-[#e0f0f0] placeholder-[#5a7a7a] focus:outline-none focus:border-[#a78bfa] transition-colors text-base"
          />
        </div>

        {#if error}
          <div class="flex items-center gap-2 p-3 rounded-xl bg-[#f56565]/10 border border-[#f56565]/20">
            <svg class="w-5 h-5 text-[#f56565] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
            <p class="text-[#f56565] text-sm">{error}</p>
          </div>
        {/if}
      </div>
    </div>

    <!-- 하단 버튼 -->
    <div class="p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] bg-[#1a2020] border-t border-[#3d4f4f]">
      <div class="flex gap-3">
        <button
          class="flex-1 flex items-center justify-center gap-2 py-4 rounded-2xl bg-gradient-to-r from-[#a78bfa] to-[#7c3aed] text-white font-bold text-base shadow-lg shadow-[#a78bfa]/20 transition-all active:scale-[0.98] disabled:opacity-50"
          on:click={generateFromCharacter}
          disabled={isGenerating}
        >
          {#if isGenerating}
            <svg class="w-5 h-5 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="10" stroke-width="2" stroke-dasharray="32" stroke-dashoffset="12"/>
            </svg>
            {$_('generate.loading')}
          {:else}
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            {$_('generate.button')}
          {/if}
        </button>
        <button
          class="px-6 py-4 rounded-2xl bg-[#2a3636] text-[#7eb8b8] font-medium border border-[#3d4f4f] transition-all active:scale-[0.98]"
          on:click={cancelGenerate}
        >
          {$_('action.cancel')}
        </button>
      </div>
    </div>

  {:else if mode === 'custom'}
    <!-- 직접 SOUL 설정 -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- 헤더 카드 -->
      <div class="flex items-center gap-4 p-4 rounded-2xl bg-gradient-to-r from-[#4fd1c5]/10 to-[#38b2ac]/10 border border-[#4fd1c5]/20 mb-6">
        <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-[#4fd1c5] to-[#38b2ac] flex items-center justify-center">
          <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
          </svg>
        </div>
        <div>
          <p class="text-[#e0f0f0] font-semibold">{$_('custom.title')}</p>
          <p class="text-[#7eb8b8] text-sm">{$_('custom.hint')}</p>
        </div>
      </div>

      <div class="space-y-5">
        <div>
          <label class="block text-[#7eb8b8] text-sm font-medium mb-2">{$_('custom.name')} <span class="text-[#f56565]">*</span></label>
          <input
            type="text"
            placeholder={$_('custom.name_placeholder')}
            bind:value={customName}
            class="w-full px-4 py-4 rounded-xl bg-[#0f1515] border-2 border-[#3d4f4f] text-[#e0f0f0] placeholder-[#5a7a7a] focus:outline-none focus:border-[#4fd1c5] transition-colors text-base"
          />
        </div>

        <div>
          <label class="block text-[#7eb8b8] text-sm font-medium mb-2">{$_('custom.personality')}</label>
          <input
            type="text"
            placeholder={$_('custom.personality_placeholder')}
            bind:value={customPersonality}
            class="w-full px-4 py-4 rounded-xl bg-[#0f1515] border-2 border-[#3d4f4f] text-[#e0f0f0] placeholder-[#5a7a7a] focus:outline-none focus:border-[#4fd1c5] transition-colors text-base"
          />
        </div>

        <div>
          <label class="block text-[#7eb8b8] text-sm font-medium mb-2">{$_('custom.gender')}</label>
          <div class="flex gap-3">
            {#each [{ key: 'female', label: $_('gender.female') }, { key: 'male', label: $_('gender.male') }, { key: 'neutral', label: $_('gender.neutral') }] as gender}
              <button
                class="flex-1 py-3 rounded-xl border-2 transition-all text-sm font-medium {customGender === gender.key ? 'bg-[#4fd1c5]/20 border-[#4fd1c5] text-[#4fd1c5]' : 'bg-[#0f1515] border-[#3d4f4f] text-[#7eb8b8]'}"
                on:click={() => customGender = gender.key}
              >
                {gender.label}
              </button>
            {/each}
          </div>
        </div>

        {#if error}
          <div class="flex items-center gap-2 p-3 rounded-xl bg-[#f56565]/10 border border-[#f56565]/20">
            <svg class="w-5 h-5 text-[#f56565] flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
            </svg>
            <p class="text-[#f56565] text-sm">{error}</p>
          </div>
        {/if}
      </div>
    </div>

    <!-- 하단 버튼 -->
    <div class="p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] bg-[#1a2020] border-t border-[#3d4f4f]">
      <div class="flex gap-3">
        <button
          class="flex-1 flex items-center justify-center gap-2 py-4 rounded-2xl bg-gradient-to-r from-[#4fd1c5] to-[#38b2ac] text-[#1e2626] font-bold text-base shadow-lg shadow-[#4fd1c5]/20 transition-all active:scale-[0.98] disabled:opacity-50"
          on:click={generateFromCustom}
          disabled={isGenerating}
        >
          {#if isGenerating}
            <svg class="w-5 h-5 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="10" stroke-width="2" stroke-dasharray="32" stroke-dashoffset="12"/>
            </svg>
            {$_('generate.loading')}
          {:else}
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
            </svg>
            {$_('generate.button')}
          {/if}
        </button>
        <button
          class="px-6 py-4 rounded-2xl bg-[#2a3636] text-[#7eb8b8] font-medium border border-[#3d4f4f] transition-all active:scale-[0.98]"
          on:click={cancelGenerate}
        >
          {$_('action.cancel')}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  /* 로딩 애니메이션 */
  @keyframes spin-slow {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  @keyframes spin-reverse {
    from { transform: rotate(360deg); }
    to { transform: rotate(0deg); }
  }

  @keyframes float {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-8px); }
  }

  @keyframes float-delayed {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-6px); }
  }

  @keyframes float-delayed-2 {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-10px); }
  }

  @keyframes pulse-slow {
    0%, 100% { opacity: 0.3; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(1.1); }
  }

  @keyframes bounce-1 {
    0%, 80%, 100% { transform: translateY(0); }
    40% { transform: translateY(-6px); }
  }

  @keyframes bounce-2 {
    0%, 80%, 100% { transform: translateY(0); }
    40% { transform: translateY(-6px); }
  }

  @keyframes bounce-3 {
    0%, 80%, 100% { transform: translateY(0); }
    40% { transform: translateY(-6px); }
  }

  :global(.animate-spin-slow) {
    animation: spin-slow 8s linear infinite;
  }

  :global(.animate-spin-reverse) {
    animation: spin-reverse 6s linear infinite;
  }

  :global(.animate-float) {
    animation: float 3s ease-in-out infinite;
  }

  :global(.animate-float-delayed) {
    animation: float-delayed 2.5s ease-in-out infinite;
    animation-delay: 0.5s;
  }

  :global(.animate-float-delayed-2) {
    animation: float-delayed-2 3.5s ease-in-out infinite;
    animation-delay: 1s;
  }

  :global(.animate-pulse-slow) {
    animation: pulse-slow 4s ease-in-out infinite;
  }

  :global(.animate-bounce-1) {
    animation: bounce-1 1.4s ease-in-out infinite;
  }

  :global(.animate-bounce-2) {
    animation: bounce-2 1.4s ease-in-out infinite;
    animation-delay: 0.2s;
  }

  :global(.animate-bounce-3) {
    animation: bounce-3 1.4s ease-in-out infinite;
    animation-delay: 0.4s;
  }
</style>
