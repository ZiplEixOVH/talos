<script lang="ts">
  import { page } from '$app/stores';
  import { Home, Bot, Settings, Plus, Trash2, PanelLeftClose, MessageSquare, Edit2, Clock } from 'lucide-svelte';

  interface Chat {
    id: string;
    title: string;
    created_at: number;
    mode?: string;
  }

  let {
    isSidebarOpen = $bindable(true),
    chats = [],
    onCreateChat,
    onDeleteChat,
    onRenameChat
  }: {
    isSidebarOpen: boolean;
    chats: Chat[];
    onCreateChat: () => void;
    onDeleteChat: (id: string, event: Event) => void;
    onRenameChat: (id: string, title: string) => Promise<void>;
  } = $props();

  let editingChatId = $state<string | null>(null);
  let editingTitle = $state<string>('');

  function startEditing(id: string, title: string, event: Event) {
    event.stopPropagation();
    event.preventDefault();
    editingChatId = id;
    editingTitle = title;
  }

  async function saveRename(id: string) {
    if (!editingTitle.trim()) return cancelEditing();
    const titleToSave = editingTitle.trim();
    cancelEditing();
    await onRenameChat(id, titleToSave);
  }

  function cancelEditing() {
    editingChatId = null;
    editingTitle = '';
  }

  function handleRenameKeydown(event: KeyboardEvent, id: string) {
    if (event.key === 'Enter') {
      event.preventDefault();
      saveRename(id);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      cancelEditing();
    }
  }

  // Svelte Action to focus and select text on mount
  function focusOnMount(node: HTMLInputElement) {
    node.focus();
    node.select();
  }
</script>

<aside
  class="bg-[#0b0f19] border-r border-slate-900 flex flex-col h-full shrink-0 transition-all duration-300 ease-in-out relative z-35 {
    isSidebarOpen ? 'w-64 opacity-100' : 'w-0 opacity-0 pointer-events-none'
  }"
>
  <!-- macOS traffic light spacer & Header inside sidebar -->
  <div class="h-10 pt-10 flex items-center justify-between px-4 mt-2">
    <span class="font-bold text-sm tracking-wider text-slate-350 font-mono pl-16">TALOS</span>
    <button
      onclick={() => isSidebarOpen = false}
      class="text-slate-500 hover:text-slate-200 transition-colors p-1 rounded hover:bg-slate-900/60 cursor-pointer no-drag"
      title="Masquer la barre"
    >
      <PanelLeftClose size={16} />
    </button>
  </div>

  <!-- Navigation links -->
  <nav class="px-3 mt-6 space-y-1">
    <a
      href="/"
      class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition-all duration-250 {$page.url.pathname === '/' ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/20' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'}"
    >
      <Home size={16} />
      Accueil
    </a>
    <a
      href="/agents"
      class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition-all duration-250 {$page.url.pathname === '/agents' ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/20' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'}"
    >
      <Bot size={16} />
      Agents
    </a>
    <a
      href="/schedules"
      class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition-all duration-250 {$page.url.pathname === '/schedules' ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/20' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'}"
    >
      <Clock size={16} />
      Planifications
    </a>
    <a
      href="/settings"
      class="flex items-center gap-3 px-3 py-2 text-sm font-semibold rounded-lg transition-all duration-250 {$page.url.pathname === '/settings' ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/20' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'}"
    >
      <Settings size={16} />
      Paramètres
    </a>
  </nav>

  <!-- Divider -->
  <div class="h-px bg-slate-900/60 mx-4 my-6"></div>

  <!-- Discussions List Section -->
  <div class="flex-1 flex flex-col min-h-0">
    <div class="px-4 flex items-center justify-between mb-2">
      <span class="text-[10px] font-bold text-slate-500 uppercase tracking-wider">Discussions</span>
      <button
        onclick={onCreateChat}
        class="p-1 rounded text-slate-500 hover:text-slate-200 hover:bg-slate-900/60 cursor-pointer transition-colors"
        title="Nouvelle discussion"
      >
        <Plus size={14} />
      </button>
    </div>

    <!-- Chats Scroll Container -->
    <div class="flex-1 overflow-y-auto px-2 pb-4 space-y-0.5 scrollbar-thin scrollbar-thumb-slate-900 scrollbar-track-transparent">
      {#if chats.length === 0}
        <div class="text-xs text-slate-600 italic px-4 py-2">Aucune discussion</div>
      {:else}
        {#each chats as chat (chat.id)}
          {#if editingChatId === chat.id}
            <div
              class="group flex items-center justify-between px-3 py-2 text-xs font-medium rounded-lg bg-slate-900/60 border border-indigo-500/30 shadow-[inset_2px_0_0_#6366f1] relative"
            >
              <div class="flex items-center gap-2 overflow-hidden w-full mr-1">
                <MessageSquare size={12} class="shrink-0 text-indigo-400" />
                <input
                  type="text"
                  bind:value={editingTitle}
                  onblur={() => saveRename(chat.id)}
                  onkeydown={(e) => handleRenameKeydown(e, chat.id)}
                  class="bg-slate-950 text-slate-200 text-xs px-1.5 py-0.5 border border-indigo-500/40 rounded focus:outline-none focus:border-indigo-400 w-full font-semibold"
                  use:focusOnMount
                />
              </div>
            </div>
          {:else}
            <a
              href="/chat/{chat.id}"
              ondblclick={(e) => startEditing(chat.id, chat.title, e)}
              class="group flex items-center justify-between px-3 py-2 text-xs font-medium rounded-lg transition-all duration-200 relative {
                $page.params.id === chat.id
                  ? 'bg-indigo-600/10 text-indigo-450 border border-indigo-550/10 shadow-[inset_2px_0_0_#6366f1]'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/30 border border-transparent'
              }"
            >
              <div class="flex items-center gap-2 overflow-hidden mr-2">
                <MessageSquare size={12} class="shrink-0 {$page.params.id === chat.id ? 'text-indigo-400' : 'text-slate-500'}" />
                <span class="truncate">{chat.title}</span>
                {#if chat.mode && chat.mode !== 'agent'}
                  <span class="px-1 py-0.5 rounded text-[8px] font-bold border shrink-0 capitalize
                    {chat.mode === 'plan' 
                      ? 'border-sky-500/30 text-sky-400 bg-sky-950/20' 
                      : 'border-emerald-500/30 text-emerald-400 bg-emerald-950/20'
                    }"
                  >
                    {chat.mode}
                  </span>
                {/if}
              </div>
              <div class="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 shrink-0">
                <button
                  onclick={(e) => startEditing(chat.id, chat.title, e)}
                  class="p-1 text-slate-500 hover:text-indigo-400 hover:bg-indigo-500/15 rounded transition-all cursor-pointer"
                  title="Renommer la discussion"
                >
                  <Edit2 size={11} />
                </button>
                <button
                  onclick={(e) => onDeleteChat(chat.id, e)}
                  class="p-1 text-slate-500 hover:text-red-400 hover:bg-red-500/15 rounded transition-all cursor-pointer"
                  title="Supprimer la discussion"
                >
                  <Trash2 size={11} />
                </button>
              </div>
            </a>
          {/if}
        {/each}
      {/if}
    </div>
  </div>
</aside>
