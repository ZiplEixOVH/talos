<script lang="ts">
  import { onMount } from 'svelte';
  import { Sparkles, Save, FileText, CheckCircle2, AlertCircle, RotateCcw, Code, ChevronDown, ChevronRight, Variable, Info, List, ToggleRight, Braces, Layers } from 'lucide-svelte';

  let prompts = $state<string[]>([]);
  let selectedPrompt = $state<string | null>(null);
  let promptContent = $state('');
  let isLoading = $state(true);
  let isSaving = $state(false);
  let showVariables = $state(true);
  let notification = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  // Template variables state
  let templateVariables = $state<Array<{
    name: string;
    description: string;
    type: 'string' | 'boolean' | 'array';
    children?: Array<{ name: string; description: string; type: string; usage: string }>;
    usage: string;
  }>>([]);

  let templateSyntax = $state<Array<{ syntax: string; description: string }>>([]);
  let expandedVars = $state<Set<string>>(new Set());

  type VariableIconMap = typeof Variable;
  const TYPE_ICONS: Record<string, VariableIconMap> = {
    string: Braces,
    boolean: ToggleRight,
    array: Layers
  };

  const TYPE_COLORS: Record<string, string> = {
    string: 'text-emerald-400',
    boolean: 'text-amber-400',
    array: 'text-sky-400'
  };

  onMount(async () => {
    await Promise.all([loadPrompts(), loadTemplateVariables()]);
  });

  async function loadTemplateVariables() {
    if (window.talosAPI) {
      try {
        const data = await window.talosAPI.getTemplateVariables();
        templateVariables = data.variables;
        templateSyntax = data.syntax;
      } catch (err) {
        console.error('Failed to load template variables:', err);
      }
    } else {
      // Fallback mock data for browser dev
      templateVariables = [
        { name: 'currentCwd', description: 'Le dossier de travail actuel (Current Working Directory)', type: 'string', usage: '{{currentCwd}}' },
        { name: 'chatFolder', description: 'Le dossier des artifacts du chat en cours', type: 'string', usage: '{{chatFolder}}' },
        { name: 'hasTools', description: 'Booléen indiquant si des outils sont disponibles dans ce mode', type: 'boolean', usage: '{{#if hasTools}}...{{/if}}' },
        {
          name: 'tools',
          description: 'Liste des outils disponibles dans le mode actuel',
          type: 'array',
          usage: '{{#each tools}}...{{/each}}',
          children: [
            { name: 'name', description: "Nom de l'outil", type: 'string', usage: '{{name}}' },
            { name: 'description', description: "Description de l'outil", type: 'string', usage: '{{description}}' }
          ]
        }
      ];
      templateSyntax = [
        { syntax: '{{variable}}', description: "Affiche la valeur d'une variable simple" },
        { syntax: '{{#if variable}}...{{/if}}', description: 'Affiche le contenu si la variable est définie et non-vide' },
        { syntax: '{{#each list}}...{{/each}}', description: "Itère sur un tableau d'éléments" },
        { syntax: '{{this}}', description: "Référence à l'élément courant dans une boucle #each" }
      ];
    }
  }

  function toggleExpand(name: string) {
    if (expandedVars.has(name)) {
      expandedVars.delete(name);
    } else {
      expandedVars.add(name);
    }
    expandedVars = new Set(expandedVars);
  }

  function insertAtCursor(text: string) {
    promptContent += text;
  }

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

  <!-- Right area: Editor + Variables panel -->
  <div class="col-span-3 flex gap-6 h-full">
    <!-- Editor column -->
    <div class="flex flex-col h-full bg-[#070b15]/60 border border-slate-900/80 rounded-2xl p-6 flex-1 min-w-0 relative">
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
              onclick={() => showVariables = !showVariables}
              class="flex items-center gap-1.5 px-3.5 py-2 bg-slate-900 border hover:bg-slate-800/70 text-slate-450 hover:text-sky-400 disabled:opacity-50 font-bold text-xs rounded-xl transition-all cursor-pointer {showVariables ? 'text-sky-400 border-sky-800/40' : 'border-slate-800/80'}"
              title="Afficher/Masquer les variables disponibles"
            >
              <Code size={13} />
              Variables
            </button>

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

    <!-- Variables panel (collapsible) -->
    {#if showVariables}
      <div class="w-64 shrink-0 bg-[#070b15]/60 border border-slate-900/80 rounded-2xl p-4 flex flex-col gap-3 overflow-y-auto select-none">
        <div class="flex items-center gap-2 px-1">
          <Variable size={14} class="text-sky-400" />
          <h3 class="text-xs font-bold text-slate-300 tracking-wider uppercase">Variables</h3>
        </div>

        <!-- Syntax reference -->
        <div class="bg-slate-950/50 border border-slate-900/80 rounded-xl p-3 space-y-1.5">
          <h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
            <Info size={11} />
            Syntaxe
          </h4>
          <div class="space-y-1">
            {#each templateSyntax as s}
              <div class="flex items-start gap-2">
                <code class="text-[10px] font-mono text-amber-300 bg-amber-950/30 px-1.5 py-0.5 rounded-md shrink-0 leading-relaxed">{s.syntax}</code>
                <span class="text-[10px] text-slate-400 leading-relaxed">{s.description}</span>
              </div>
            {/each}
          </div>
        </div>

        <!-- Variables list -->
        <div class="space-y-1">
          <h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-wider px-1 flex items-center gap-1.5">
            <List size={11} />
            Disponibles
          </h4>
          
          {#each templateVariables as v}
            <div class="bg-slate-950/30 border border-slate-900/60 rounded-xl overflow-hidden">
              <button
                onclick={() => toggleExpand(v.name)}
                class="w-full flex items-center gap-2 px-3 py-2 text-xs transition-all hover:bg-slate-900/40 cursor-pointer text-left"
              >
                {#if v.children}
                  {#if expandedVars.has(v.name)}
                    <ChevronDown size={12} class="text-slate-500 shrink-0" />
                  {:else}
                    <ChevronRight size={12} class="text-slate-500 shrink-0" />
                  {/if}
                {:else}
                  <span class="w-3 shrink-0"></span>
                {/if}

                {#if TYPE_ICONS[v.type]}
                  {@const Icon = TYPE_ICONS[v.type]}
                  <Icon size={12} class={`${TYPE_COLORS[v.type] || 'text-slate-400'} shrink-0`} />
                {/if}

                <span class="font-bold text-slate-200 truncate">{v.name}</span>
                <span class="text-[10px] uppercase tracking-wider text-slate-600 ml-auto">{v.type}</span>
              </button>

              <div class="px-3 pb-2">
                <p class="text-[10px] text-slate-400 leading-relaxed mb-1.5">{v.description}</p>
                <button
                  onclick={() => insertAtCursor(v.usage)}
                  class="group flex items-center gap-1 px-2 py-1 bg-slate-900/60 border border-slate-800/60 hover:border-sky-800/40 hover:bg-sky-950/20 rounded-lg transition-all cursor-pointer"
                  title="Insérer dans l'éditeur"
                >
                  <code class="text-[10px] font-mono text-amber-300/80 group-hover:text-amber-300 transition-colors">{v.usage}</code>
                  <span class="text-[9px] text-slate-500 group-hover:text-sky-400 ml-auto">insérer</span>
                </button>

                <!-- Children (for arrays/objects) -->
                {#if v.children && expandedVars.has(v.name)}
                  <div class="mt-2 space-y-1 pl-2 border-l border-slate-800/60">
                    {#each v.children as child}
                      <div class="flex items-start gap-2 py-1">
                        <span class="text-[10px] font-bold text-slate-300 shrink-0 w-16 truncate">{child.name}</span>
                        <span class="text-[10px] text-slate-500 leading-relaxed">{child.description}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>