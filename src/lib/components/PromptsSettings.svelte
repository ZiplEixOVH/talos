<script lang="ts">
  import { onMount } from 'svelte';
  import { Sparkles, Save, FileText, CheckCircle2, AlertCircle, RotateCcw } from 'lucide-svelte';

  let prompts = $state<string[]>([]);
  let selectedPrompt = $state<string | null>(null);
  let promptContent = $state('');
  let isLoading = $state(true);
  let isSaving = $state(false);
  let notification = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  onMount(async () => {
    await loadPrompts();
  });

  async function loadPrompts() {
    isLoading = true;
    if (window.talosAPI) {
      try {
        prompts = await window.talosAPI.getPrompts();
        if (prompts.length > 0) {
          await selectPrompt(prompts[0]);
        }
      } catch (err) {
        console.error('Failed to load prompts:', err);
      }
    } else {
      // Fallback local storage mock prompts for browser testing
      prompts = ['system.md', 'agent.md', 'plan.md', 'ask.md'];
      await selectPrompt(prompts[0]);
    }
    isLoading = false;
  }

  async function selectPrompt(name: string) {
    selectedPrompt = name;
    if (window.talosAPI) {
      try {
        promptContent = await window.talosAPI.readPrompt(name);
      } catch (err) {
        console.error(`Failed to read prompt ${name}:`, err);
      }
    } else {
      promptContent = localStorage.getItem(`talos_mock_prompt_${name}`) || `# Mode: ${name.replace('.md', '').toUpperCase()}\n\nContenu par défaut simulé pour le navigateur.`;
    }
  }

  async function savePrompt() {
    if (!selectedPrompt) return;
    isSaving = true;
    showNotification(null);

    if (window.talosAPI) {
      try {
        await window.talosAPI.savePrompt(selectedPrompt, promptContent);
        showNotification({ type: 'success', message: `Prompt '${selectedPrompt}' enregistré avec succès.` });
      } catch (err: any) {
        showNotification({ type: 'error', message: `Erreur lors de l'enregistrement : ${err.message}` });
      }
    } else {
      localStorage.setItem(`talos_mock_prompt_${selectedPrompt}`, promptContent);
      showNotification({ type: 'success', message: `Prompt '${selectedPrompt}' enregistré en simulation.` });
    }
    isSaving = false;
  }

  async function resetPrompt() {
    if (!selectedPrompt) return;
    if (!confirm(`Êtes-vous sûr de vouloir réinitialiser le prompt '${selectedPrompt}' à sa version par défaut ? Vos modifications locales seront écrasées.`)) {
      return;
    }
    isSaving = true;
    showNotification(null);

    if (window.talosAPI && window.talosAPI.resetPrompt) {
      try {
        const defaultContent = await window.talosAPI.resetPrompt(selectedPrompt);
        promptContent = defaultContent;
        showNotification({ type: 'success', message: `Prompt '${selectedPrompt}' réinitialisé avec succès.` });
      } catch (err: any) {
        showNotification({ type: 'error', message: `Erreur lors de la réinitialisation : ${err.message}` });
      }
    } else {
      // Mock reset for browser testing
      promptContent = `# Mode: ${selectedPrompt.replace('.md', '').toUpperCase()}\n\nContenu par défaut restauré en simulation.`;
      localStorage.setItem(`talos_mock_prompt_${selectedPrompt}`, promptContent);
      showNotification({ type: 'success', message: `Prompt '${selectedPrompt}' réinitialisé en simulation.` });
    }
    isSaving = false;
  }

  function showNotification(notif: typeof notification) {
    notification = notif;
    if (notif?.type === 'success') {
      setTimeout(() => {
        if (notification?.message === notif.message) {
          notification = null;
        }
      }, 3000);
    }
  }
</script>

<div class="grid grid-cols-4 gap-6 w-full h-[550px]">
  <!-- Left list: available prompts -->
  <div class="col-span-1 bg-[#070b15]/60 border border-slate-900/80 rounded-2xl p-4 flex flex-col gap-2 select-none">
    <h3 class="text-xs font-bold text-slate-400 tracking-wider uppercase mb-2 px-1">Fichiers Prompts</h3>
    
    {#if isLoading}
      <div class="flex-1 flex items-center justify-center">
        <div class="w-5 h-5 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
      </div>
    {:else}
      <div class="flex-col gap-1 overflow-y-auto pr-1">
        {#each prompts as p}
          <button
            onclick={() => selectPrompt(p)}
            class="w-full flex items-center gap-2.5 px-3 py-2 text-xs font-bold rounded-xl transition-all text-left cursor-pointer border
              {selectedPrompt === p 
                ? 'bg-indigo-600/10 border-indigo-500/20 text-indigo-400 font-extrabold' 
                : 'bg-transparent border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-900/30'
              }"
          >
            <FileText size={14} class={selectedPrompt === p ? 'text-indigo-400' : 'text-slate-500'} />
            <span class="truncate">{p}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Right area: Code Editor -->
  <div class="col-span-3 flex flex-col h-full bg-[#070b15]/60 border border-slate-900/80 rounded-2xl p-6 relative">
    {#if selectedPrompt}
      <div class="flex items-center justify-between mb-4 select-none shrink-0">
        <div>
          <h3 class="text-sm font-bold text-slate-200 flex items-center gap-2">
            Édition de <span class="text-indigo-400 font-mono">{selectedPrompt}</span>
          </h3>
          <p class="text-[10px] text-slate-500 mt-0.5">Ces templates régissent le comportement de Talos dans ses différents modes.</p>
        </div>
        
        <div class="flex items-center gap-2">
          <button
            onclick={resetPrompt}
            disabled={isSaving}
            class="flex items-center gap-1.5 px-3.5 py-2 bg-slate-900 border border-slate-800/80 hover:bg-slate-800/70 text-slate-450 hover:text-slate-200 disabled:opacity-50 font-bold text-xs rounded-xl transition-all cursor-pointer"
            title="Restaurer le template par défaut"
          >
            <RotateCcw size={13} />
            Réinitialiser
          </button>

          <button
            onclick={savePrompt}
            disabled={isSaving}
            class="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:bg-indigo-800 text-white font-bold text-xs rounded-xl transition-colors cursor-pointer"
          >
            {#if isSaving}
              <div class="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
            {:else}
              <Save size={14} />
            {/if}
            Enregistrer
          </button>
        </div>
      </div>

      <!-- Editor textarea -->
      <div class="flex-1 min-h-0 relative">
        <textarea
          bind:value={promptContent}
          class="w-full h-full bg-slate-950/80 text-slate-200 border border-slate-900/80 focus:border-indigo-500/30 focus:outline-none rounded-xl p-4 font-mono text-xs leading-relaxed resize-none scrollbar-thin scrollbar-thumb-slate-900 scrollbar-track-transparent focus:ring-1 focus:ring-indigo-500/25"
          placeholder="# Écrivez votre prompt ici..."
        ></textarea>
      </div>

      <!-- Floating notification -->
      {#if notification}
        <div class="absolute bottom-4 left-6 right-6 flex items-center gap-2 px-4 py-3 rounded-xl border select-none animate-in fade-in slide-in-from-bottom-2 duration-200
          {notification.type === 'success' 
            ? 'bg-emerald-600/10 border-emerald-500/20 text-emerald-400' 
            : 'bg-rose-600/10 border-rose-500/20 text-rose-400'
          }"
        >
          {#if notification.type === 'success'}
            <CheckCircle2 size={16} class="shrink-0" />
          {:else}
            <AlertCircle size={16} class="shrink-0" />
          {/if}
          <span class="text-xs font-semibold leading-none">{notification.message}</span>
        </div>
      {/if}
    {:else}
      <div class="flex-1 flex flex-col items-center justify-center text-center text-slate-500 space-y-2">
        <div class="p-3 bg-slate-900/30 rounded-full border border-slate-900/60 text-slate-400">
          <Sparkles size={24} />
        </div>
        <div class="max-w-xs">
          <p class="text-xs font-bold text-slate-350">Aucun prompt sélectionné</p>
          <p class="text-[10px] text-slate-500 mt-1">Sélectionnez un fichier de prompt à gauche pour commencer son édition.</p>
        </div>
      </div>
    {/if}
  </div>
</div>
