<script lang="ts">
  import { _ } from 'svelte-i18n';

  export let onLogin: (email: string, password: string) => void = () => {};

  let email = '';
  let password = '';
  let isLoading = false;
  let error = '';

  function handleSubmit() {
    if (!email.trim() || !password.trim()) {
      error = $_<string>('login.error');
      return;
    }
    error = '';
    isLoading = true;
    onLogin(email, password);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      handleSubmit();
    }
  }
</script>

<div class="flex flex-col min-h-screen bg-[#121212] p-6 justify-center">
  <div class="mb-12 text-center">
    <h1 class="text-3xl font-bold text-white mb-2">{$_('app.name')}</h1>
    <p class="text-gray-500">{$_('login.subtitle')}</p>
  </div>

  <div class="space-y-4">
    <div>
      <input
        type="email"
        bind:value={email}
        on:keydown={handleKeydown}
        placeholder={$_('login.email')}
        class="w-full bg-[#1E1E1E] rounded-xl px-4 py-3.5 text-white placeholder-gray-500 outline-none focus:ring-2 focus:ring-green-500"
      />
    </div>

    <div>
      <input
        type="password"
        bind:value={password}
        on:keydown={handleKeydown}
        placeholder={$_('login.password')}
        class="w-full bg-[#1E1E1E] rounded-xl px-4 py-3.5 text-white placeholder-gray-500 outline-none focus:ring-2 focus:ring-green-500"
      />
    </div>

    {#if error}
      <p class="text-red-500 text-sm">{error}</p>
    {/if}

    <button
      class="w-full bg-green-500 py-4 rounded-xl text-white text-lg font-semibold disabled:opacity-50 disabled:cursor-not-allowed"
      on:click={handleSubmit}
      disabled={isLoading}
    >
      {isLoading ? $_('login.loading') : $_('login.button')}
    </button>
  </div>
</div>
