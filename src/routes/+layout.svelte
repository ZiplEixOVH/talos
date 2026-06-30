<script lang="ts">
  import './layout.css';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import favicon from '$lib/assets/favicon.svg';
  import Sidebar from '$lib/components/Sidebar.svelte';
  import Header from '$lib/components/Header.svelte';



  let { children } = $props();

  let isSidebarOpen = $state(true);
  let chats = $state<Array<{ id: string; title: string; created_at: number; mode: string }>>([]);

  onMount(async () => {
    // Load sidebar state
    const savedSidebarState = localStorage.getItem('talos_sidebar_open');
    if (savedSidebarState !== null) {
      isSidebarOpen = savedSidebarState === 'true';
    }

    // Load chats
    await loadChats();

    // Listen for chat events
    window.addEventListener('talos:chat-created', loadChats);
    window.addEventListener('talos:trigger-new-chat', createNewChat);
    window.addEventListener('talos:chat-renamed', loadChats);

    // Listen for scheduler chat creation events (from IPC)
    if (window.talosAPI?.onSchedulerChatCreated) {
      const unsubScheduler = window.talosAPI.onSchedulerChatCreated(() => {
        loadChats();
      });
      // Store cleanup
      (window as any).__talosSchedulerUnsub = unsubScheduler;
    }

    return () => {
      window.removeEventListener('talos:chat-created', loadChats);
      window.removeEventListener('talos:trigger-new-chat', createNewChat);
      window.removeEventListener('talos:chat-renamed', loadChats);
      if (typeof (window as any).__talosSchedulerUnsub === 'function') {
        (window as any).__talosSchedulerUnsub();
      }
    };
  });

  async function loadChats() {
    if (window.talosAPI) {
      try {
        chats = await window.talosAPI.getChats();
      } catch (err) {
        console.error('Failed to fetch from SQLite, fallback to localStorage:', err);
        loadChatsFromLocalStorage();
      }
    } else {
      loadChatsFromLocalStorage();
    }
  }

  function loadChatsFromLocalStorage() {
    const saved = localStorage.getItem('talos_chats');
    if (saved) {
      chats = JSON.parse(saved);
    } else {
      chats = [];
      localStorage.setItem('talos_chats', JSON.stringify(chats));
    }
  }

  function toggleSidebar() {
    isSidebarOpen = !isSidebarOpen;
    localStorage.setItem('talos_sidebar_open', String(isSidebarOpen));
  }

  async function createNewChat() {
    const newId = Math.random().toString(36).substring(2, 9);
    const newTitle = `Nouveau Chat ${chats.length + 1}`;
    const newChat = { id: newId, title: newTitle, created_at: Date.now() };

    if (window.talosAPI) {
      try {
        await window.talosAPI.createChat(newId, newTitle);
        chats = await window.talosAPI.getChats();
      } catch (err) {
        console.error(err);
        saveNewChatToLocalStorage(newChat);
      }
    } else {
      saveNewChatToLocalStorage(newChat);
    }

    goto(`/chat/${newId}`);
  }

  function saveNewChatToLocalStorage(newChat: any) {
    chats = [newChat, ...chats];
    localStorage.setItem('talos_chats', JSON.stringify(chats));
  }

  async function deleteChat(id: string, event: Event) {
    event.stopPropagation();
    event.preventDefault();

    if (window.talosAPI) {
      try {
        await window.talosAPI.deleteChat(id);
        chats = await window.talosAPI.getChats();
      } catch (err) {
        console.error(err);
        deleteChatFromLocalStorage(id);
      }
    } else {
      deleteChatFromLocalStorage(id);
    }

    // Redirect to home if deleting active chat
    if ($page.params.id === id) {
      goto('/');
    }
  }

  function deleteChatFromLocalStorage(id: string) {
    chats = chats.filter(c => c.id !== id);
    localStorage.setItem('talos_chats', JSON.stringify(chats));
  }

  async function renameChat(id: string, title: string) {
    if (window.talosAPI) {
      try {
        await window.talosAPI.renameChat(id, title);
        chats = await window.talosAPI.getChats();
      } catch (err) {
        console.error(err);
        renameChatInLocalStorage(id, title);
      }
    } else {
      renameChatInLocalStorage(id, title);
    }
    window.dispatchEvent(new CustomEvent('talos:chat-renamed', { detail: { id, title } }));
  }

  function renameChatInLocalStorage(id: string, title: string) {
    chats = chats.map(c => c.id === id ? { ...c, title } : c);
    localStorage.setItem('talos_chats', JSON.stringify(chats));
  }
</script>

<svelte:head>
  <link rel="icon" href={favicon} />
  <title>Talos AI Platform</title>
</svelte:head>

<div class="flex h-screen w-screen bg-[#070b15] text-slate-100 overflow-hidden font-sans animate-fade-in">
  <!-- Sidebar Component -->
  <Sidebar
    bind:isSidebarOpen
    {chats}
    onCreateChat={createNewChat}
    onDeleteChat={deleteChat}
    onRenameChat={renameChat}
  />

  <!-- Main View Container -->
  <div class="flex-1 flex flex-col h-full min-w-0 overflow-hidden relative">

    <!-- Top Header Component (Drag Region) -->
    <Header
      bind:isSidebarOpen
      {toggleSidebar}
    />

    <!-- Main Page Content -->
    <main class="flex-1 overflow-auto bg-[#070b15] relative z-10">
      {@render children()}
    </main>
  </div>
</div>

<style>
  :global(.no-drag) {
    -webkit-app-region: no-drag;
  }
</style>
