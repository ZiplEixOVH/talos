<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    Clock, Plus, Trash2, Play, Power, PowerOff,
    ExternalLink, X, Save, AlertCircle,

    Edit

  } from 'lucide-svelte';

  interface ScheduledTask {
    id: string;
    name: string;
    description: string;
    schedule_type: 'cron' | 'once';
    schedule_value: string;
    instructions: string;
    provider_id: string;
    model: string;
    workspace?: string;
    internet_access: boolean;
    enabled: boolean;
    created_at: number;
    updated_at: number;
    last_run: number | null;
    last_result: string | null;
    next_run: number | null;
    total_runs: number;
    chat_id: string;
  }

  let tasks = $state<ScheduledTask[]>([]);
  let isLoading = $state(true);

  // Modal state
  let showModal = $state(false);
  let editingTask = $state<ScheduledTask | null>(null);
  let modalName = $state('');
  let modalDescription = $state('');
  let modalType = $state<'cron' | 'once'>('cron');
  let modalValue = $state('');
  let modalInstructions = $state('');
  let modalProvider = $state('');
  let modalModel = $state('');
  let modalWorkspace = $state('');
  let modalInternet = $state(false);
  let modalError = $state('');

  let providersList = $state<Array<{ id: string; name: string }>>([]);
  let modelsList = $state<Array<{ id: string; name: string }>>([]);

  // Stream cleanup
  let cleanups: (() => void)[] = [];

  onMount(async () => {
    await loadData();
    await loadProviders();

    // Écouter les mises à jour du scheduler
    if (window.talosAPI?.onSchedulerTaskExecuted) {
      const unsub = window.talosAPI.onSchedulerTaskExecuted(() => {
        loadData();
      });
      cleanups.push(unsub);
    }
  });

  onDestroy(() => {
    cleanups.forEach(fn => fn());
  });

  async function loadData() {
    if (window.talosAPI) {
      try {
        tasks = await window.talosAPI.getSchedules();
      } catch (err) {
        console.error('Failed to load schedules:', err);
      }
    }
    isLoading = false;
  }

  async function loadProviders() {
    if (window.talosAPI) {
      try {
        const provs = await window.talosAPI.getProviders();
        providersList = provs;
        if (provs.length > 0 && !modalProvider) {
          modalProvider = provs[0].id;
          loadModels(provs[0].id);
        }
      } catch (err) {
        console.error('Failed to load providers:', err);
      }
    }
  }

  async function loadModels(providerId: string, preserveSelection = false) {
    if (window.talosAPI) {
      try {
        modelsList = await window.talosAPI.getModels(providerId);
        if (modelsList.length > 0) {
          // Si on édite et que le modèle actuel existe dans la liste, on le garde
          if (preserveSelection && modalModel && modelsList.some(m => m.name === modalModel)) {
            return; // Garder la sélection actuelle
          }
          modalModel = modelsList[0].name;
        } else {
          modalModel = '';
        }
      } catch (err) {
        console.error('Failed to load models:', err);
      }
    }
  }

  function handleProviderChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    modalProvider = target.value;
    modalModel = '';
    loadModels(modalProvider);
  }

  function openCreateModal() {
    editingTask = null;
    modalName = '';
    modalDescription = '';
    modalType = 'cron';
    modalValue = '0 9 * * *';
    modalInstructions = '';
    modalWorkspace = '';
    modalInternet = false;
    modalProvider = providersList[0]?.id || '';
    modalModel = '';
    modalError = '';
    if (modalProvider) loadModels(modalProvider);
    showModal = true;
  }

  function openEditModal(task: ScheduledTask) {
    editingTask = task;
    modalName = task.name;
    modalDescription = task.description || '';
    modalType = task.schedule_type;
    modalValue = task.schedule_value;
    modalInstructions = task.instructions;
    modalWorkspace = task.workspace || '';
    modalInternet = task.internet_access ?? false;
    modalProvider = task.provider_id;
    modalModel = task.model;
    modalError = '';
    loadModels(task.provider_id, true);
    showModal = true;
  }

  function closeModal() {
    showModal = false;
    editingTask = null;
  }

  async function saveModal() {
    modalError = '';

    // Validation
    if (!modalName.trim()) {
      modalError = 'Le nom est requis.';
      return;
    }
    if (!modalValue.trim()) {
      modalError = 'La valeur de planification est requise.';
      return;
    }
    if (!modalInstructions.trim()) {
      modalError = 'Les instructions sont requises.';
      return;
    }
    if (!modalProvider) {
      modalError = 'Veuillez sélectionner un provider.';
      return;
    }
    if (!modalModel) {
      modalError = 'Veuillez sélectionner un modèle.';
      return;
    }

    const taskId = editingTask?.id || Math.random().toString(36).substring(2, 9);

    const task: ScheduledTask = {
      id: taskId,
      name: modalName.trim(),
      description: modalDescription.trim(),
      schedule_type: modalType,
      schedule_value: modalValue.trim(),
      instructions: modalInstructions.trim(),
      provider_id: modalProvider,
      model: modalModel,
      workspace: modalWorkspace.trim() || undefined,
      internet_access: modalInternet,
      enabled: editingTask?.enabled ?? true,
      created_at: editingTask?.created_at ?? Date.now(),
      updated_at: Date.now(),
      last_run: editingTask?.last_run ?? null,
      last_result: editingTask?.last_result ?? null,
      next_run: editingTask?.next_run ?? null,
      total_runs: editingTask?.total_runs ?? 0,
      chat_id: editingTask?.chat_id || `sched-${taskId}`,
    };

    if (window.talosAPI) {
      try {
        await window.talosAPI.saveSchedule(task);
        await loadData();
        closeModal();
      } catch (err: any) {
        modalError = err.message || 'Erreur lors de la sauvegarde.';
      }
    } else {
      closeModal();
    }
  }

  async function handleDelete(id: string) {
    if (window.talosAPI) {
      try {
        await window.talosAPI.deleteSchedule(id);
        await loadData();
      } catch (err) {
        console.error('Failed to delete schedule:', err);
      }
    }
  }

  async function handleToggle(task: ScheduledTask) {
    const updated = { ...task, enabled: !task.enabled };
    if (window.talosAPI) {
      try {
        await window.talosAPI.saveSchedule(updated);
        await loadData();
      } catch (err) {
        console.error('Failed to toggle schedule:', err);
      }
    }
  }

  async function handleRunNow(id: string) {
    if (window.talosAPI) {
      try {
        await window.talosAPI.runScheduleNow(id);
        await loadData();
      } catch (err) {
        console.error('Failed to run schedule now:', err);
      }
    }
  }

  function formatDate(ts: number | null): string {
    if (!ts) return '—';
    return new Date(ts).toLocaleString('fr-FR', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit'
    });
  }

  function formatSchedule(task: ScheduledTask): string {
    if (task.schedule_type === 'once') {
      return `Unique : ${formatDate(new Date(task.schedule_value).getTime())}`;
    }
    return `Cron : ${task.schedule_value}`;
  }

  function openChat(chatId: string) {
    goto(`/chat/${chatId}`);
  }

  function getStatusClass(task: ScheduledTask): string {
    if (!task.enabled) return 'text-slate-500';
    if (task.last_result?.startsWith('Erreur')) return 'text-red-400';
    return 'text-emerald-400';
  }

  function getStatusText(task: ScheduledTask): string {
    if (!task.enabled) return 'Désactivé';
    if (task.last_result?.startsWith('Erreur')) return 'Erreur';
    if (task.total_runs === 0) return 'En attente';
    return 'Actif';
  }

  function getProviderName(id: string): string {
    return providersList.find(p => p.id === id)?.name || id;
  }
</script>

<div class="max-w-5xl mx-auto p-6 space-y-6">
  <!-- Page Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold tracking-tight bg-gradient-to-r from-blue-400 via-indigo-400 to-purple-400 bg-clip-text text-transparent">Planifications</h1>
      <p class="text-slate-400 mt-1">Créez et gérez des tâches planifiées pour votre agent.</p>
    </div>
    <button
      onclick={openCreateModal}
      class="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl cursor-pointer text-sm font-bold transition-all shadow-md shadow-indigo-950/30"
    >
      <Plus size={16} />
      Nouvelle tâche
    </button>
  </div>

  <!-- Tasks List -->
  {#if isLoading}
    <div class="flex items-center justify-center py-20 text-slate-500">
      <div class="animate-pulse text-sm">Chargement...</div>
    </div>
  {:else if tasks.length === 0}
    <div class="bg-[#0b0f19] border border-slate-800/60 rounded-2xl p-12 text-center space-y-4">
      <div class="inline-flex p-4 bg-slate-900/40 rounded-full border border-slate-800/60 text-slate-500">
        <Clock size={36} />
      </div>
      <h2 class="text-lg font-bold text-slate-300">Aucune tâche planifiée</h2>
      <p class="text-sm text-slate-500 max-w-md mx-auto">
        Créez votre première tâche pour exécuter automatiquement des instructions à des horaires définis via cron ou une date unique.
      </p>
      <button
        onclick={openCreateModal}
        class="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl cursor-pointer text-sm font-bold transition-all"
      >
        <Plus size={16} />
        Créer une tâche
      </button>
    </div>
  {:else}
    <div class="space-y-3">
      {#each tasks as task (task.id)}
        <div class="bg-[#0b0f19] border border-slate-800/60 rounded-2xl p-5 transition-all hover:border-slate-700/60 group">
          <div class="flex items-start justify-between gap-4">
            <!-- Left: Task info -->
            <div class="flex-1 min-w-0 space-y-2">
              <div class="flex items-center gap-3">
                <h3 class="text-base font-bold text-slate-200 truncate">{task.name}</h3>
                <span class="text-[10px] font-bold px-2 py-0.5 rounded-full border {getStatusClass(task)} {task.enabled ? 'border-emerald-500/20 bg-emerald-950/10' : 'border-slate-700 bg-slate-900/40'}">
                  {getStatusText(task)}
                </span>
                {#if task.total_runs > 0}
                  <span class="text-[10px] text-slate-500 font-mono bg-slate-900/50 px-2 py-0.5 rounded-full border border-slate-800/60">
                    {task.total_runs} run{task.total_runs > 1 ? 's' : ''}
                  </span>
                {/if}
              </div>

              {#if task.description}
                <p class="text-xs text-slate-500">{task.description}</p>
              {/if}

              <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                <span class="font-mono text-indigo-400/80">{formatSchedule(task)}</span>
                <span>Provider : {getProviderName(task.provider_id)}</span>
                <span>Modèle : {task.model}</span>
              </div>

              <div class="flex flex-wrap gap-x-2 gap-y-1">
                {#if task.internet_access}
                  <span class="text-[10px] font-bold px-2 py-0.5 rounded-full border border-sky-500/20 bg-sky-950/10 text-sky-400">🌐 Internet</span>
                {/if}
                {#if task.workspace}
                  <span class="text-[10px] font-bold px-2 py-0.5 rounded-full border border-emerald-500/20 bg-emerald-950/10 text-emerald-400">📁 Workspace</span>
                {/if}
                {#if !task.internet_access && !task.workspace}
                  <span class="text-[10px] text-slate-600 italic">Réponse simple sans outils</span>
                {/if}
              </div>

              <div class="flex flex-wrap gap-x-6 gap-y-1 text-[11px] text-slate-600">
                {#if task.last_run}
                  <span>Dernier run : {formatDate(task.last_run)}</span>
                {/if}
                {#if task.next_run}
                  <span>Prochain run : {formatDate(task.next_run)}</span>
                {/if}
              </div>

              {#if task.last_result}
                <div class="text-xs text-slate-400 font-mono bg-slate-950/40 px-3 py-1.5 rounded-lg border border-slate-900/60 truncate max-w-xl">
                  {task.last_result}
                </div>
              {/if}
            </div>

            <!-- Right: Actions -->
            <div class="flex items-center gap-1.5 shrink-0 opacity-60 group-hover:opacity-100 transition-opacity">
              <button
                onclick={() => openChat(task.chat_id)}
                class="p-2 text-slate-500 hover:text-indigo-400 hover:bg-indigo-500/10 rounded-lg cursor-pointer transition-all"
                title="Voir le chat dédié"
              >
                <ExternalLink size={14} />
              </button>

              <button
                onclick={() => handleToggle(task)}
                class="p-2 text-slate-500 hover:text-amber-400 hover:bg-amber-500/10 rounded-lg cursor-pointer transition-all"
                title={task.enabled ? 'Désactiver' : 'Activer'}
              >
                {#if task.enabled}
                  <PowerOff size={14} />
                {:else}
                  <Power size={14} />
                {/if}
              </button>

              <button
                onclick={() => handleRunNow(task.id)}
                disabled={!task.enabled}
                class="p-2 text-slate-500 hover:text-emerald-400 hover:bg-emerald-500/10 rounded-lg cursor-pointer transition-all disabled:opacity-30 disabled:cursor-not-allowed"
                title="Exécuter maintenant"
              >
                <Play size={14} />
              </button>

              <button
                onclick={() => openEditModal(task)}
                class="p-2 text-slate-500 hover:text-sky-400 hover:bg-sky-500/10 rounded-lg cursor-pointer transition-all"
                title="Modifier"
              >
                <!-- <Save size={14} /> -->
                <Edit size={14} />
              </button>

              <button
                onclick={() => handleDelete(task.id)}
                class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg cursor-pointer transition-all"
                title="Supprimer"
              >
                <Trash2 size={14} />
              </button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Modal (Create/Edit) -->
{#if showModal}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4"
    onclick={closeModal}
    onkeydown={(e) => e.key === 'Escape' && closeModal()}
    role="dialog"
    aria-modal="true"
    tabindex="-1"
  >
    <!-- Modal Panel -->
    <div
      class="bg-[#0b0f19] border border-slate-800/60 rounded-2xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Modal Header -->
      <div class="flex items-center justify-between p-6 border-b border-slate-800/60">
        <h2 class="text-lg font-bold text-slate-200">
          {editingTask ? 'Modifier la tâche' : 'Nouvelle tâche planifiée'}
        </h2>
        <button
          onclick={closeModal}
          class="p-1.5 text-slate-500 hover:text-slate-200 hover:bg-slate-800 rounded-lg cursor-pointer transition-all"
        >
          <X size={18} />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-6 space-y-5">
        {#if modalError}
          <div class="flex items-center gap-2 px-4 py-3 bg-red-950/30 border border-red-500/20 rounded-xl text-sm text-red-400">
            <AlertCircle size={14} class="shrink-0" />
            <span>{modalError}</span>
          </div>
        {/if}

        <!-- Name -->
        <div class="space-y-1.5">
          <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Nom</label>
          <input
            type="text"
            bind:value={modalName}
            placeholder="Ex: Rapport quotidien"
            class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 placeholder-slate-600 outline-none transition-all"
          />
        </div>

        <!-- Description -->
        <div class="space-y-1.5">
          <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Description (optionnelle)</label>
          <input
            type="text"
            bind:value={modalDescription}
            placeholder="Brève description de la tâche"
            class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 placeholder-slate-600 outline-none transition-all"
          />
        </div>

        <!-- Schedule Type + Value -->
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Type</label>
            <select
              bind:value={modalType}
              class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 outline-none transition-all"
            >
              <option value="cron">Cron (récurrent)</option>
              <option value="once">Unique (date)</option>
            </select>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">
              {#if modalType === 'cron'}
                Expression Cron
              {:else}
                Date et heure
              {/if}
            </label>
            {#if modalType === 'cron'}
              <input
                type="text"
                bind:value={modalValue}
                placeholder="min heure jour mois jour_semaine"
                class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 placeholder-slate-600 outline-none transition-all font-mono"
              />
              <p class="text-[10px] text-slate-600 mt-1">
                Format : minute (0-59) heure (0-23) jour (1-31) mois (1-12) jour_semaine (0-6)
                — <span class="text-indigo-400/70">Ex: <code class="bg-slate-900 px-1 rounded">0 9 * * 1-5</code> (9h en semaine)</span>
              </p>
            {:else}
              <input
                type="datetime-local"
                bind:value={modalValue}
                class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 outline-none transition-all"
              />
            {/if}
          </div>
        </div>

        <!-- Provider + Model -->
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Provider</label>
            <select
              value={modalProvider}
              onchange={handleProviderChange}
              class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 outline-none transition-all"
            >
              <option value="" disabled>Sélectionner</option>
              {#each providersList as p}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Modèle</label>
            <select
              bind:value={modalModel}
              class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 outline-none transition-all"
            >
              <option value="" disabled>Sélectionner</option>
              {#each modelsList as m}
                <option value={m.name}>{m.name}</option>
              {/each}
            </select>
          </div>
        </div>

        <!-- Capacités de l'agent -->
        <div class="space-y-3 bg-slate-950/20 rounded-xl border border-slate-800/60 p-4">
          <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Capacités</label>

          <!-- Internet Access Toggle -->
          <div class="flex items-center justify-between">
            <div class="space-y-0.5">
              <h3 class="text-sm font-semibold text-slate-300">Accès Internet</h3>
              <p class="text-[11px] text-slate-500">L'agent peut effectuer des recherches web et consulter des pages.</p>
            </div>
            <button
              onclick={() => modalInternet = !modalInternet}
              class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out outline-none
                {modalInternet ? 'bg-indigo-600' : 'bg-slate-700'}"
            >
              <span
                class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out
                  {modalInternet ? 'translate-x-4' : 'translate-x-0'}"
              ></span>
            </button>
          </div>

          <!-- Workspace (dossier de travail) -->
          <div class="space-y-1.5 border-t border-slate-800/40 pt-3">
            <label class="text-sm font-semibold text-slate-300">
              Workspace <span class="text-slate-500 font-normal text-xs">(optionnel)</span>
            </label>
            <p class="text-[11px] text-slate-500 mb-2">Dossier dans lequel l'agent peut lire, écrire et exécuter des commandes. Laissez vide pour un agent sans accès fichiers.</p>
            <input
              type="text"
              bind:value={modalWorkspace}
              placeholder="Chemin absolu du dossier de travail"
              class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 placeholder-slate-600 outline-none transition-all font-mono"
            />
          </div>
        </div>

        <!-- Instructions -->
        <div class="space-y-1.5">
          <label class="text-xs font-bold text-slate-400 uppercase tracking-wider">Instructions pour l'agent</label>
          <textarea
            bind:value={modalInstructions}
            rows="5"
            placeholder="Décrivez ce que l'agent doit faire à chaque exécution..."
            class="w-full bg-slate-950/60 border border-slate-800/80 focus:border-indigo-500/40 rounded-xl px-4 py-2.5 text-sm text-slate-200 placeholder-slate-600 outline-none transition-all resize-none font-mono"
          ></textarea>
        </div>

        <!-- Info about the dedicated chat -->
        {#if editingTask?.chat_id}
          <div class="text-xs text-slate-500 bg-slate-950/30 px-4 py-2.5 rounded-xl border border-slate-900/60 flex items-center gap-2">
            <span>💬 Chat dédié :</span>
            <button
              onclick={() => { closeModal(); openChat(editingTask!.chat_id); }}
              class="text-indigo-400 hover:text-indigo-300 underline cursor-pointer"
            >
              {editingTask.chat_id}
            </button>
          </div>
        {/if}
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end gap-3 p-6 border-t border-slate-800/60">
        <button
          onclick={closeModal}
          class="px-4 py-2 bg-slate-900 hover:bg-slate-800 text-slate-300 rounded-xl cursor-pointer text-sm font-semibold transition-all"
        >
          Annuler
        </button>
        <button
          onclick={saveModal}
          class="px-5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl cursor-pointer text-sm font-bold transition-all shadow-md shadow-indigo-950/30 flex items-center gap-2"
        >
          <Save size={14} />
          {editingTask ? 'Enregistrer' : 'Créer la tâche'}
        </button>
      </div>
    </div>
  </div>
{/if}