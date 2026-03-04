<script lang="ts">
  import { onMount } from 'svelte';
  import { _, locale } from 'svelte-i18n';
  import { API_BASE } from './config';

  export let onBack: () => void;
  export let onSelectSoul: (folderName: string, name: string) => void;
  export let onCreateNew: () => void;

  interface PersonaInfo {
    name: string;
    folderName: string;
    avatar: string;
    accentColor: string;
  }

  let personas: PersonaInfo[] = [];
  let isLoading = true;
  let activePersonaFolder = 'default';
  let isManageMode = false;

  onMount(async () => {
    await loadPersonas();
  });

  async function loadPersonas() {
    try {
      isLoading = true;
      const res = await fetch(`${API_BASE}/api/personas`);
      if (res.ok) {
        const data = await res.json();
        personas = data.personas || [];
        activePersonaFolder = data.active || 'default';
      }
    } catch (err) {
      console.error('Persona 목록 로드 실패:', err);
    } finally {
      isLoading = false;
    }
  }

  async function applyPersona(folderName: string, name: string) {
    try {
      const res = await fetch(`${API_BASE}/api/persona/active`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ persona: folderName })
      });
      if (res.ok) {
        activePersonaFolder = folderName;
        alert($_<string>('soul.applied'));
      }
    } catch (err) {
      console.error('Persona apply failed:', err);
    }
  }

  async function deletePersona(folderName: string, name: string) {
    if (folderName === 'default') {
      alert($_<string>('soulList.default_cannot_delete'));
      return;
    }
    if (!confirm($_<string>('soulList.delete_confirm', { values: { name } }))) return;
    // TODO: 삭제 API 구현 필요
    console.log('Delete persona:', folderName);
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
      <h1 class="text-[#e0f0f0] font-semibold text-lg">{$_('soulList.title')}</h1>
    </div>
    <!-- 관리 버튼 -->
    <button
      class="px-3 py-1.5 rounded-lg text-sm font-medium transition-colors {isManageMode ? 'bg-[#a78bfa] text-white' : 'text-[#7eb8b8] hover:bg-[#3d4f4f]'}"
      on:click={() => isManageMode = !isManageMode}
    >
      {isManageMode ? $_('soulList.done') : $_('soulList.manage')}
    </button>
  </div>

  <!-- 새 SOUL 생성 버튼 -->
  <div class="p-3">
    <button
      class="w-full flex items-center justify-center gap-2 bg-gradient-to-r from-[#a78bfa] to-[#7c3aed] hover:from-[#b49bfb] hover:to-[#8b4be8] py-3 rounded-xl text-white font-semibold transition-all shadow-md"
      on:click={onCreateNew}
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
      </svg>
      {$_('soulList.create')}
    </button>
  </div>

  <!-- Persona 목록 -->
  <div class="flex-1 overflow-y-auto px-3">
    {#if isLoading}
      <div class="flex flex-col items-center justify-center h-64 text-[#5a7a7a]">
        <div class="w-12 h-12 rounded-full bg-gradient-to-br from-[#a78bfa] to-[#7c3aed] flex items-center justify-center mb-4 animate-pulse">
          <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
          </svg>
        </div>
        <p class="text-[#7eb8b8]">{$_('soul.loading')}</p>
      </div>
    {:else if personas.length === 0}
      <div class="flex flex-col items-center justify-center h-64 text-[#5a7a7a]">
        <svg class="w-16 h-16 mb-4 opacity-50 text-[#a78bfa]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
        </svg>
        <p class="font-medium text-[#7eb8b8]">{$_('soulList.empty')}</p>
        <p class="text-sm mt-1">{$_('soulList.empty_hint')}</p>
      </div>
    {:else}
      <div class="space-y-2">
        {#each personas as persona (persona.folderName)}
          <div class="flex items-center rounded-xl bg-[#2a3636] border border-[#3d4f4f] overflow-hidden group transition-all hover:border-[#a78bfa]/50">
            <!-- 메인 영역 (클릭하면 편집) -->
            <button
              class="flex-1 flex items-center gap-3 px-4 py-3 text-left min-w-0"
              on:click={() => onSelectSoul(persona.folderName, persona.name)}
            >
              <!-- 아바타 -->
              <div class="relative w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 text-white font-bold" style="background: linear-gradient(135deg, {persona.accentColor}, {persona.accentColor}dd);">
                {persona.avatar}
                {#if persona.folderName === activePersonaFolder}
                  <div class="absolute -top-1 -right-1 w-4 h-4 rounded-full bg-[#4fd1c5] flex items-center justify-center">
                    <svg class="w-2.5 h-2.5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/>
                    </svg>
                  </div>
                {/if}
              </div>

              <!-- 정보 -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-[#e0f0f0] font-medium truncate">{persona.name}</span>
                  {#if persona.folderName === 'default'}
                    <span class="px-2 py-0.5 rounded-full bg-[#a78bfa]/20 text-[#a78bfa] text-xs font-medium">{$_('soulList.default')}</span>
                  {/if}
                  {#if persona.folderName === activePersonaFolder}
                    <span class="px-2 py-0.5 rounded-full bg-[#4fd1c5]/20 text-[#4fd1c5] text-xs font-medium">{$_('soulList.active')}</span>
                  {/if}
                </div>
                <span class="text-[#5a7a7a] text-xs">{persona.folderName}</span>
              </div>
            </button>

            <!-- 액션 버튼들 (관리 모드일 때만 표시) -->
            {#if isManageMode}
              <div class="flex items-center gap-1 pr-2">
                <!-- 적용 버튼 -->
                {#if persona.folderName !== activePersonaFolder}
                  <button
                    class="p-2 text-[#4fd1c5] hover:bg-[#4fd1c5]/10 rounded-lg transition-colors"
                    on:click|stopPropagation={() => applyPersona(persona.folderName, persona.name)}
                    title={$_('soulList.use')}
                  >
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                    </svg>
                  </button>
                {/if}
                <!-- 삭제 버튼 (기본 persona는 삭제 불가) -->
                {#if persona.folderName !== 'default'}
                  <button
                    class="p-2 text-[#f56565] hover:bg-[#f56565]/10 rounded-lg transition-colors"
                    on:click|stopPropagation={() => deletePersona(persona.folderName, persona.name)}
                    title={$_('action.delete')}
                  >
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                    </svg>
                  </button>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- 하단 여백 -->
  <div class="h-[calc(1rem+env(safe-area-inset-bottom))]"></div>
</div>
