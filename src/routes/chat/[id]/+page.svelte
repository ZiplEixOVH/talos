<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    Send, Sparkles, FolderOpen,
    Paperclip, X, Square, Pencil
  } from 'lucide-svelte';
  import { marked } from 'marked';
  import ModelSelector from '$lib/components/ModelSelector.svelte';

  // Récupération de l'id réactif depuis la route
  let chatId = $derived($page.params.id || '');

  let chatTitle = $state('Discussion');
  let messages = $state<Array<{ id: string; role: string; content: string; tool_calls?: any[]; tool_call_id?: string }>>([]);
  let visibleMessages = $derived(messages.filter(m => m.role !== 'tool' && (m.content !== '' || (m.tool_calls && m.tool_calls.length > 0))));

  let streamCleanups: (() => void)[] = [];
  function clearStreamSubscriptions() {
    streamCleanups.forEach(unsub => unsub());
    streamCleanups = [];
  }

  onDestroy(() => {
    clearStreamSubscriptions();
  });
  let inputMessage = $state('');
  let thinkingStatus = $state<'thinking' | 'writing' | 'executing' | ''>('');
  let isThinking = $derived(thinkingStatus !== '');
  let chatContainer = $state<HTMLDivElement | null>(null);

  // Configuration de l'environnement actif
  let cwd = $state('');
  let activeProviderId = $state('ollama');
  let activeModel = $state('');
  let isSettingsLoading = $state(true);

  // Pièces jointes
  let attachedFiles = $state<string[]>([]);
  let attachedFileObjects = $state<File[]>([]);
  let fileInput = $state<HTMLInputElement | null>(null);

  // Modification de message
  let editingMessageId = $state<string | null>(null);
  let editingMessageText = $state<string>('');

  // Mode actif de discussion
  let currentMode = $state<'agent' | 'plan' | 'ask'>('agent');

  // Drag and drop / Paste state
  let isDragging = $state(false);

  // Nom abrégé du dossier de travail (CWD)
  let folderName = $derived(cwd ? (cwd.split(/[/\\]/).pop() || cwd) : 'Dossier');
  
  let textareaElement = $state<HTMLTextAreaElement | null>(null);
  let editAreaElement = $state<HTMLTextAreaElement | null>(null);

  // Auto-resize the input textarea height based on content
  $effect(() => {
    const _val = inputMessage;
    if (textareaElement) {
      textareaElement.style.height = 'auto';
      textareaElement.style.height = `${textareaElement.scrollHeight}px`;
    }
  });

  // Auto-resize the editing textarea height based on content
  $effect(() => {
    const _val = editingMessageText;
    if (editAreaElement) {
      editAreaElement.style.height = 'auto';
      editAreaElement.style.height = `${editAreaElement.scrollHeight}px`;
    }
  });

  // Render markdown dynamically
  function renderMarkdown(content: string): string {
    try {
      return marked.parse(content, { async: false }) as string;
    } catch (e) {
      console.error(e);
      return content;
    }
  }

  // Surveille le changement de chatId pour recharger la conversation
  $effect(() => {
    if (chatId) {
      clearStreamSubscriptions();
      loadConversationData(chatId).then(() => {
        // Après HMR : si un stream est encore en cours pour ce chat, se réabonner
        const activeStream = sessionStorage.getItem('talos_active_stream');
        if (activeStream) {
          try {
            const streamInfo = JSON.parse(activeStream);
            if (streamInfo.chatId === chatId) {
              console.log('[HMR-RECOVERY] Re-subscribing to active stream for chat:', chatId);
              thinkingStatus = 'writing';
              subscribeToStream(chatId);
            }
          } catch (e) {
            sessionStorage.removeItem('talos_active_stream');
          }
        }
      });
    }
  });

  onMount(() => {
    loadInitialSettings();
    loadModelSuggestions();

    const handleRenameEvent = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      if (detail.id === chatId) {
        chatTitle = detail.title;
      }
    };

    window.addEventListener('talos:chat-renamed', handleRenameEvent);
    return () => {
      window.removeEventListener('talos:chat-renamed', handleRenameEvent);
    };
  });

  async function loadInitialSettings() {
    // Récupérer le dossier de travail actuel
    if (window.talosAPI) {
      try {
        cwd = await window.talosAPI.getCwd();
        activeProviderId = await window.talosAPI.getSetting('active_provider_id', 'ollama');
        activeModel = await window.talosAPI.getSetting('active_model_name', '');
      } catch (err) {
        console.error(err);
        loadSettingsFromLocalStorage();
      }
    } else {
      loadSettingsFromLocalStorage();
    }
    isSettingsLoading = false;
  }

  function loadSettingsFromLocalStorage() {
    cwd = localStorage.getItem('talos_cwd') || '/Users/bleroyer/perso/talos';
    activeProviderId = localStorage.getItem('talos_active_provider_id') || 'ollama';
    activeModel = localStorage.getItem('talos_active_model_name') || '';
  }

  async function loadConversationData(id: string) {
    thinkingStatus = '';
    messages = [];
    attachedFiles = [];
    attachedFileObjects = [];

    // 1. Charger le titre de la discussion
    let foundTitle = 'Discussion';
    let foundMode = 'agent';
    if (window.talosAPI) {
      try {
        const chats = await window.talosAPI.getChats();
        const chat = chats.find(c => c.id === id);
        if (chat) {
          foundTitle = chat.title;
          foundMode = chat.mode || 'agent';
        }
      } catch (err) {
        console.error(err);
      }
    } else {
      const saved = localStorage.getItem('talos_chats');
      if (saved) {
        const chats = JSON.parse(saved);
        const chat = chats.find((c: any) => c.id === id);
        if (chat) {
          foundTitle = chat.title;
          foundMode = chat.mode || 'agent';
        }
      }
    }
    chatTitle = foundTitle;
    currentMode = foundMode as any;

    // 2. Charger les messages réels de l'historique
    if (window.talosAPI) {
      try {
        messages = await window.talosAPI.getMessages(id);
      } catch (err) {
        console.error('Failed to load messages from SQLite, fallback:', err);
        loadMessagesFromLocalStorage(id);
      }
    } else {
      loadMessagesFromLocalStorage(id);
    }

    await scrollToBottom();
  }

  function loadMessagesFromLocalStorage(id: string) {
    const saved = localStorage.getItem(`talos_messages_${id}`);
    messages = saved ? JSON.parse(saved) : [];
  }

  function saveMessageToLocalStorage(id: string, msg: { id: string; role: string; content: string }) {
    const saved = localStorage.getItem(`talos_messages_${id}`);
    const msgs = saved ? JSON.parse(saved) : [];
    msgs.push(msg);
    localStorage.setItem(`talos_messages_${id}`, JSON.stringify(msgs));
  }

  async function scrollToBottom() {
    await tick();
    if (chatContainer) {
      chatContainer.scrollTop = chatContainer.scrollHeight;
    }
  }

  async function selectDirectory() {
    if (window.talosAPI) {
      try {
        const selected = await window.talosAPI.selectCwd();
        if (selected) {
          cwd = selected;
        }
      } catch (err) {
        console.error(err);
      }
    } else {
      // Simulé hors Electron
      const newPath = prompt('Entrez le chemin absolu du dossier de travail :', cwd);
      if (newPath) {
        cwd = newPath;
        localStorage.setItem('talos_cwd', newPath);
      }
    }
  }

  async function handleSelectModel(providerId: string, modelName: string) {
    activeProviderId = providerId;
    activeModel = modelName;
    
    if (window.talosAPI) {
      try {
        await window.talosAPI.setSetting('active_provider_id', providerId);
        await window.talosAPI.setSetting('active_model_name', modelName);
      } catch (err) {
        console.error(err);
      }
    } else {
      localStorage.setItem('talos_active_provider_id', providerId);
      localStorage.setItem('talos_active_model_name', modelName);
    }
  }

  function triggerFileSelector() {
    if (fileInput) fileInput.click();
  }

  function handleFileChange(e: Event) {
    const target = e.target as HTMLInputElement;
    if (target.files) {
      const files = Array.from(target.files);
      attachedFileObjects = [...attachedFileObjects, ...files];
      const names = files.map(file => file.name);
      attachedFiles = [...attachedFiles, ...names];
      target.value = '';
    }
  }

  function removeFile(index: number) {
    attachedFiles = attachedFiles.filter((_, i) => i !== index);
    attachedFileObjects = attachedFileObjects.filter((_, i) => i !== index);
  }

  // ── Stream IPC subscription (réutilisable après HMR) ──────────────────
  function subscribeToStream(targetChatId: string) {
    clearStreamSubscriptions();
    if (!window.talosAPI) return;

    const unsubChunk = window.talosAPI.onChatStreamChunk((data: any) => {
      if (data.chatId === targetChatId) {
        const idx = messages.findIndex(m => m.id === data.requestId);
        if (idx !== -1) {
          messages[idx].content += data.text;
        } else {
          // Le placeholder n'existe pas encore (perdu lors d'un HMR) → créer le message
          messages.push({
            id: data.requestId,
            role: 'assistant',
            content: data.text
          });
        }
        if (thinkingStatus === 'thinking' || thinkingStatus === 'executing') {
          thinkingStatus = 'writing';
        }
        scrollToBottom();
      }
    });

    const unsubEnd = window.talosAPI.onChatStreamEnd((data: any) => {
      if (data.chatId === targetChatId) {
        sessionStorage.removeItem('talos_active_stream');
        clearStreamSubscriptions();
        thinkingStatus = '';
        // Recharger depuis le JSON pour s'assurer que l'état final est correct
        loadConversationData(targetChatId);
      }
    });

    const unsubError = window.talosAPI.onChatStreamError((data: any) => {
      if (data.chatId === targetChatId) {
        sessionStorage.removeItem('talos_active_stream');
        clearStreamSubscriptions();
        thinkingStatus = '';
        const idx = messages.findIndex(m => m.id === data.requestId);
        if (idx !== -1) {
          messages[idx].content += `\n\n*(Erreur lors du streaming : ${data.error})*`;
        }
        scrollToBottom();
      }
    });

    const unsubToolMessage = window.talosAPI.onChatToolMessage((data: any) => {
      if (data.chatId === targetChatId) {
        const idx = messages.findIndex(m => m.id === data.id);
        if (idx === -1) {
          // Nouveau message — on l'ajoute (même vide, ça sert de placeholder)
          messages.push(data);
        } else {
          // ⚠️ RACE CONDITION : le canal onChatStreamChunk peut avoir déjà
          // rempli ce message. Si data.content est vide, on préserve le
          // contenu existant pour ne pas écraser ce qui a déjà été streamé.
          if (data.content !== '') {
            Object.assign(messages[idx], data);
          } else if (messages[idx].content === '') {
            // Seulement si le message est vraiment encore vide, on met à jour
            Object.assign(messages[idx], data);
          }
        }
        if (data.role === 'assistant' && data.content.startsWith('`')) {
          thinkingStatus = 'executing';
        }
        scrollToBottom();
      }
    });

    streamCleanups.push(unsubChunk, unsubEnd, unsubError, unsubToolMessage);
  }

  async function startStreamGeneration() {
    if (!activeModel) {
      messages.push({
        id: `err-${Date.now()}`,
        role: 'assistant',
        content: 'Veuillez sélectionner un modèle dans les outils au bas de l\'écran avant d\'envoyer un message.'
      });
      await scrollToBottom();
      return;
    }

    thinkingStatus = 'thinking';
    try {
      if (window.talosAPI) {
        const plainMessages = messages.map(m => {
          const apiMsg: any = { role: m.role, content: m.content || '' };
          if (m.tool_calls) {
            apiMsg.tool_calls = m.tool_calls;
          }
          if (m.tool_call_id) {
            apiMsg.tool_call_id = m.tool_call_id;
          }
          return apiMsg;
        });
        const cleanMessages = $state.snapshot(plainMessages);
        
        const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        const assistantMsg = { id: aiMsgId, role: 'assistant', content: '' };
        messages.push(assistantMsg);
        await scrollToBottom();

        sessionStorage.setItem('talos_active_stream', JSON.stringify({ chatId }));
        subscribeToStream(chatId);

        window.talosAPI.startChatStream(activeProviderId, activeModel, cleanMessages, chatId, aiMsgId);
      } else {
        await new Promise(r => setTimeout(r, 1200));
        const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        const content = `[Simulation Fallback Browser]\nModèle sélectionné : ${activeModel}\nFournisseur : ${activeProviderId}\nDossier de travail : ${cwd}\n\nVotre message a été reçu ! Pour exécuter de vrais appels d'API, veuillez lancer l'application avec Electron et configurer un fournisseur de clés.`;
        const assistantMsg = { id: aiMsgId, role: 'assistant', content };
        
        messages.push(assistantMsg);
        saveMessageToLocalStorage(chatId, assistantMsg);
        thinkingStatus = '';
        await scrollToBottom();
      }
    } catch (err: any) {
      console.error(err);
      sessionStorage.removeItem('talos_active_stream');
      thinkingStatus = '';
      const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
      const errMsg = { id: aiMsgId, role: 'assistant', content: `Désolé, une erreur s'est produite lors de l'appel d'API : ${err.message || err}` };
      messages.push(errMsg);
      await scrollToBottom();
    }
  }

  // ── Slash Commands & Autocomplete ─────────────────────────────

  const slashCommands = [
    { name: '/help', desc: 'Show available commands' },
    { name: '/model', desc: 'Show current model / provider, or switch model with /model <provider/model>' },
    { name: '/clear', desc: 'Start a new conversation' },
    { name: '/new', desc: 'Start a new conversation' },
    { name: '/agent', desc: 'Switch to Agent mode (autonomous execution)' },
    { name: '/plan', desc: 'Switch to Plan mode (technical design & planning)' },
    { name: '/ask', desc: 'Switch to Ask mode (questions, Q&A, brainstorming)' },
  ];

  let showSuggestions = $state(false);
  let selectedSuggestionIndex = $state(0);
  let modelSuggestions = $state<Array<{ label: string; providerId: string; modelName: string }>>([]);

  // Load available models for autocomplete suggestions
  async function loadModelSuggestions() {
    if (window.talosAPI) {
      try {
        const provs = await window.talosAPI.getProviders();
        const suggestions: Array<{ label: string; providerId: string; modelName: string }> = [];
        for (const p of provs) {
          const mods = await window.talosAPI.getModels(p.id);
          for (const m of mods) {
            suggestions.push({
              label: `${p.name}/${m.name}`,
              providerId: p.id,
              modelName: m.name
            });
          }
        }
        modelSuggestions = suggestions;
      } catch (err) {
        console.error(err);
        loadModelSuggestionsFromLocalStorage();
      }
    } else {
      loadModelSuggestionsFromLocalStorage();
    }
  }

  function loadModelSuggestionsFromLocalStorage() {
    const savedProvs = localStorage.getItem('talos_providers');
    const provs = savedProvs ? JSON.parse(savedProvs) : [];
    const suggestions: Array<{ label: string; providerId: string; modelName: string }> = [];
    for (const p of provs) {
      const savedMods = localStorage.getItem(`talos_models_${p.id}`);
      const mods = savedMods ? JSON.parse(savedMods) : [];
      for (const m of mods) {
        suggestions.push({
          label: `${p.name}/${m.name}`,
          providerId: p.id,
          modelName: m.name
        });
      }
    }
    modelSuggestions = suggestions;
  }

  // Reactive: update suggestion visibility when input changes
  $effect(() => {
    const text = inputMessage;
    if (text.startsWith('/') && !text.includes('\n')) {
      const parts = text.split(/\s+/);

      // If we have "/model " followed by a partial model name
      if (parts[0].toLowerCase() === '/model' && parts.length >= 2) {
        const partial = parts.slice(1).join(' ').toLowerCase();
        const matched = modelSuggestions.filter(m => m.label.toLowerCase().includes(partial));
        if (matched.length > 0) {
          showSuggestions = true;
          if (selectedSuggestionIndex >= matched.length) {
            selectedSuggestionIndex = 0;
          }
        } else {
          showSuggestions = false;
        }
      }
      // If we're still typing the command itself (no space yet)
      else if (parts.length === 1) {
        const partial = parts[0].toLowerCase();
        const matched = slashCommands.filter(cmd => cmd.name.startsWith(partial));
        if (matched.length > 0) {
          showSuggestions = true;
          if (selectedSuggestionIndex >= matched.length) {
            selectedSuggestionIndex = 0;
          }
        } else {
          showSuggestions = false;
        }
      } else {
        showSuggestions = false;
      }
    } else {
      showSuggestions = false;
    }
  });

  // Filtered suggestions based on current input
  let filteredSuggestions = $derived.by(() => {
    if (!showSuggestions) return [];
    const parts = inputMessage.split(/\s+/);

    // If we're past "/model " ➜ show model suggestions
    if (parts[0].toLowerCase() === '/model' && parts.length >= 2) {
      const partial = parts.slice(1).join(' ').toLowerCase();
      return modelSuggestions.filter(m => m.label.toLowerCase().includes(partial));
    }

    // Otherwise show command name suggestions
    const partial = parts[0].toLowerCase();
    return slashCommands.filter(cmd => cmd.name.startsWith(partial));
  });

  let providersList = $state<Array<{ id: string; name: string }>>([]);

  async function getProviderName(providerId: string): Promise<string> {
    // Try to load providers if not already loaded
    if (providersList.length === 0) {
      if (window.talosAPI) {
        try {
          const provs = await window.talosAPI.getProviders();
          providersList = provs;
        } catch (err) {
          console.error(err);
        }
      } else {
        const saved = localStorage.getItem('talos_providers');
        if (saved) {
          providersList = JSON.parse(saved);
        }
      }
    }
    const provider = providersList.find(p => p.id === providerId);
    return provider?.name || providerId;
  }

  async function handleSlashCommand(text: string): Promise<boolean> {
    const parts = text.split(/\s+/);
    const cmd = parts[0].toLowerCase();

    // /model <name> — change model (no provider change, keep current provider)
    if (cmd === '/model' && parts.length >= 2) {
      const newModelName = parts.slice(1).join(' ');

      // Persist the change
      activeModel = newModelName;
      if (window.talosAPI) {
        try {
          await window.talosAPI.setSetting('active_model_name', newModelName);
        } catch (err) {
          console.error(err);
        }
      } else {
        localStorage.setItem('talos_active_model_name', newModelName);
      }

      // Add a system message to confirm (not sent to AI, just for the user)
      const providerName = await getProviderName(activeProviderId);
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: `Model set to **${newModelName}** (provider: ${providerName})` };
      messages.push(sysMsg);
      await scrollToBottom();
      return true;
    }

    // /model — show current model/provider info
    if (cmd === '/model') {
      const providerName = await getProviderName(activeProviderId);
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: `**Current model:** ${activeModel || 'none'}\n**Current provider:** ${providerName}` };
      messages.push(sysMsg);
      await scrollToBottom();
      return true;
    }

    // /clear or /new — create a new conversation and navigate to it
    if (cmd === '/clear' || cmd === '/new') {
      const newId = Math.random().toString(36).substring(2, 9);
      const newTitle = `Chat ${newId.substring(0, 5)}`;

      // Notify the layout to add the new chat via CustomEvent (the layout listens for it)
      // But the layout creates chats from its own Sidebar button. We'll create it ourselves.
      if (window.talosAPI) {
        try {
          await window.talosAPI.createChat(newId, newTitle);
        } catch (err) {
          console.error(err);
          // fallback: localStorage
          const savedChats = JSON.parse(localStorage.getItem('talos_chats') || '[]');
          savedChats.unshift({ id: newId, title: newTitle, created_at: Date.now() });
          localStorage.setItem('talos_chats', JSON.stringify(savedChats));
        }
      } else {
        const savedChats = JSON.parse(localStorage.getItem('talos_chats') || '[]');
        savedChats.unshift({ id: newId, title: newTitle, created_at: Date.now() });
        localStorage.setItem('talos_chats', JSON.stringify(savedChats));
      }

      // Dispatch event so the layout re-fetches the chats list
      window.dispatchEvent(new CustomEvent('talos:chat-created'));

      // Navigate to the new chat
      await goto(`/chat/${newId}`);
      return true;
    }

    // /help — show available commands
    if (cmd === '/help') {
      let helpText = '**Available slash commands:**\n\n';
      for (const sc of slashCommands) {
        helpText += `- \`${sc.name}\` — ${sc.desc}\n`;
      }
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: helpText };
      messages.push(sysMsg);
      await scrollToBottom();
      return true;
    }

    // /agent — switch to Agent mode
    if (cmd === '/agent') {
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: 'Switched to **Agent mode** (autonomous execution).' };
      messages.push(sysMsg);
      await scrollToBottom();
      selectMode('agent');
      return true;
    }

    // /plan — switch to Plan mode
    if (cmd === '/plan') {
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: 'Switched to **Plan mode** (technical design & planning).' };
      messages.push(sysMsg);
      await scrollToBottom();
      selectMode('plan');
      return true;
    }

    // /ask — switch to Ask mode
    if (cmd === '/ask') {
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: 'Switched to **Ask mode** (questions, Q&A, brainstorming).' };
      messages.push(sysMsg);
      await scrollToBottom();
      selectMode('ask');
      return true;
    }

    // Unknown command — show a hint
    if (cmd.startsWith('/')) {
      const sysMsgId = `sys-${Math.random().toString(36).substring(2, 9)}`;
      const sysMsg = { id: sysMsgId, role: 'system', content: `Unknown command: \`${cmd}\`. Type \`/help\` for available commands.` };
      messages.push(sysMsg);
      await scrollToBottom();
      return true;
    }

    return false; // Not a slash command, proceed normally
  }

  async function readFileAsText(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = () => reject(reader.error);
      reader.readAsText(file);
    });
  }

  async function sendMessage() {
    const text = inputMessage.trim();
    if (!text && attachedFiles.length === 0) return;

    // Read attached files first
    let fileContents = '';
    const filesToRead = [...attachedFileObjects];

    // Clear inputs immediately
    inputMessage = '';
    attachedFiles = [];
    attachedFileObjects = [];

    // ── Check for slash commands first ──
    if (text.startsWith('/')) {
      const handled = await handleSlashCommand(text);
      if (handled) return;
    }

    if (!activeModel) {
      messages.push({
        id: `err-${Date.now()}`,
        role: 'assistant',
        content: 'Veuillez sélectionner un modèle dans les outils au bas de l\'écran avant d\'envoyer un message.'
      });
      await scrollToBottom();
      return;
    }

    for (const file of filesToRead) {
      try {
        const content = await readFileAsText(file);
        fileContents += `\n\n--- Fichier joint : ${file.name} ---\n\`\`\`\n${content}\n\`\`\``;
      } catch (err) {
        console.error(`Failed to read file ${file.name}:`, err);
        fileContents += `\n\n[Erreur de lecture du fichier joint : ${file.name}]`;
      }
    }

    const textToSend = text + fileContents;
    const userMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
    const userMsg = { id: userMsgId, role: 'user', content: textToSend };
    
    messages.push(userMsg);
    
    if (window.talosAPI) {
      try {
        await window.talosAPI.addMessage(userMsgId, chatId, 'user', textToSend);
      } catch (err) {
        console.error(err);
        saveMessageToLocalStorage(chatId, userMsg);
      }
    } else {
      saveMessageToLocalStorage(chatId, userMsg);
    }

    await scrollToBottom();
    await startStreamGeneration();
  }

  function startEditing(msg: any) {
    editingMessageId = msg.id;
    editingMessageText = msg.content;
  }

  function cancelEditing() {
    editingMessageId = null;
    editingMessageText = '';
  }

  function saveMessagesToLocalStorage(id: string, msgs: any[]) {
    localStorage.setItem(`talos_messages_${id}`, JSON.stringify(msgs));
  }

  async function saveEditedMessage(id: string) {
    const idx = messages.findIndex(m => m.id === id);
    if (idx === -1) return;

    const updatedMessages = messages.slice(0, idx + 1);
    updatedMessages[idx].content = editingMessageText;

    editingMessageId = null;
    editingMessageText = '';

    if (window.talosAPI) {
      try {
        await window.talosAPI.saveMessages(chatId, $state.snapshot(updatedMessages));
      } catch (err) {
        console.error('Failed to save edited messages:', err);
        saveMessagesToLocalStorage(chatId, updatedMessages);
      }
    } else {
      saveMessagesToLocalStorage(chatId, updatedMessages);
    }

    messages = updatedMessages;
    await scrollToBottom();
    await startStreamGeneration();
  }

  async function stopChatStream() {
    if (window.talosAPI) {
      try {
        window.talosAPI.stopChatStream(chatId);
      } catch (err) {
        console.error('Failed to stop stream:', err);
      }
    }
    sessionStorage.removeItem('talos_active_stream');
    clearStreamSubscriptions();
    thinkingStatus = '';
    await loadConversationData(chatId);
  }

  async function selectMode(mode: 'agent' | 'plan' | 'ask') {
    if (currentMode === mode) return;
    currentMode = mode;
    
    if (window.talosAPI) {
      try {
        await window.talosAPI.updateChatMode(chatId, mode);
        window.dispatchEvent(new CustomEvent('talos:chat-created'));
      } catch (err) {
        console.error('Failed to update chat mode:', err);
      }
    } else {
      const savedChats = JSON.parse(localStorage.getItem('talos_chats') || '[]');
      const index = savedChats.findIndex((c: any) => c.id === chatId);
      if (index !== -1) {
        savedChats[index].mode = mode;
        localStorage.setItem('talos_chats', JSON.stringify(savedChats));
        window.dispatchEvent(new CustomEvent('talos:chat-created'));
      }
    }
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (isThinking && e.ctrlKey && e.key.toLowerCase() === 'c') {
      const selection = window.getSelection()?.toString();
      if (!selection) {
        e.preventDefault();
        stopChatStream();
      }
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    // ── Autocomplete navigation ──
    if (showSuggestions && filteredSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedSuggestionIndex = Math.min(selectedSuggestionIndex + 1, filteredSuggestions.length - 1);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedSuggestionIndex = Math.max(selectedSuggestionIndex - 1, 0);
        return;
      }
      if (e.key === 'Tab' || e.key === 'Enter') {
        e.preventDefault();
        const selected = filteredSuggestions[selectedSuggestionIndex];
        if (selected) {
          // Check if this is a model suggestion (has providerId)
          if ('providerId' in selected && 'modelName' in selected) {
            const modelSuggestion = selected as any;
            // Change both provider and model directly
            activeProviderId = modelSuggestion.providerId;
            activeModel = modelSuggestion.modelName;
            if (window.talosAPI) {
              window.talosAPI.setSetting('active_provider_id', modelSuggestion.providerId);
              window.talosAPI.setSetting('active_model_name', modelSuggestion.modelName);
            } else {
              localStorage.setItem('talos_active_provider_id', modelSuggestion.providerId);
              localStorage.setItem('talos_active_model_name', modelSuggestion.modelName);
            }
            inputMessage = '';
            showSuggestions = false;
          } else {
            // It's a slash command suggestion
            inputMessage = selected.name + ' ';
          }
          selectedSuggestionIndex = 0;
          return;
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        showSuggestions = false;
        selectedSuggestionIndex = 0;
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }
  async function readFileAsBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const result = reader.result as string;
        const base64 = result.split(',')[1];
        resolve(base64);
      };
      reader.onerror = () => reject(reader.error);
      reader.readAsDataURL(file);
    });
  }

  async function processFiles(files: FileList | File[]) {
    for (const file of Array.from(files)) {
      if (file.type.startsWith('image/')) {
        const base64 = await readFileAsBase64(file);
        const safeName = file.name.replace(/[^a-zA-Z0-9.]/g, '_');
        const filename = `image_${Date.now()}_${safeName}`;
        if (window.talosAPI && window.talosAPI.saveMedia) {
          try {
            const fileUrl = await window.talosAPI.saveMedia(chatId, filename, base64);
            const imageLink = `\n![Image](${fileUrl})\n`;
            inputMessage = inputMessage ? inputMessage + imageLink : imageLink;
          } catch (err) {
            console.error('Failed to save media:', err);
          }
        } else {
          const mockUrl = `data:${file.type};base64,${base64}`;
          const imageLink = `\n![Image](${mockUrl})\n`;
          inputMessage = inputMessage ? inputMessage + imageLink : imageLink;
        }
      } else {
        if (!attachedFiles.includes(file.name)) {
          attachedFileObjects = [...attachedFileObjects, file];
          attachedFiles = [...attachedFiles, file.name];
        }
      }
    }
  }

  function handlePaste(e: ClipboardEvent) {
    if (e.clipboardData && e.clipboardData.files.length > 0) {
      e.preventDefault();
      processFiles(e.clipboardData.files);
    }
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<div 
  class="flex flex-col h-full w-full bg-transparent overflow-hidden relative"
  onpaste={handlePaste}
  ondragover={(e) => { e.preventDefault(); isDragging = true; }}
  ondragleave={() => isDragging = false}
  ondrop={(e) => { e.preventDefault(); isDragging = false; if (e.dataTransfer) processFiles(e.dataTransfer.files); }}
>
  
  {#if isDragging}
    <div class="absolute inset-0 bg-[#020617]/85 border-2 border-dashed border-indigo-500/50 rounded-2xl z-50 flex flex-col items-center justify-center pointer-events-none select-none animate-in fade-in duration-200">
      <div class="p-6 bg-indigo-900/10 rounded-full border border-indigo-500/20 text-indigo-400 mb-4 animate-bounce">
        <Sparkles size={48} />
      </div>
      <h3 class="text-base font-bold text-slate-200">Relâchez pour joindre les fichiers</h3>
      <p class="text-xs text-slate-500 mt-1.5">Déposez vos images ou fichiers de code directement ici.</p>
    </div>
  {/if}
  
  <!-- Top bar of the discussion: chat title & Mode Selector segmented tabs -->
  <div class="h-12 border-b border-slate-900/60 bg-[#070b15]/65 backdrop-blur-md px-8 flex items-center justify-between shrink-0 select-none z-10">
    <div class="flex items-center gap-3">
      <h2 class="text-xs font-bold tracking-wide text-slate-300">{chatTitle}</h2>
    </div>
    
    <!-- Mode Switcher segment controls -->
    <div class="flex bg-slate-950/80 p-0.5 border border-slate-900 rounded-lg text-[10px] font-bold">
      {#each ['agent', 'plan', 'ask'] as m}
        <button
          onclick={() => selectMode(m as any)}
          class="px-3 py-1 rounded-md transition-all cursor-pointer capitalize
            {currentMode === m 
              ? m === 'agent' 
                ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/25' 
                : m === 'plan' 
                  ? 'bg-sky-600/10 text-sky-400 border border-sky-500/25' 
                  : 'bg-emerald-600/10 text-emerald-400 border border-emerald-500/25'
              : 'text-slate-500 hover:text-slate-350 border border-transparent hover:bg-slate-900/40'
            }"
        >
          {m}
        </button>
      {/each}
    </div>
  </div>

  <!-- Messages List Feed (Takes all upper space, padded nicely so text isn't against screen edges) -->
  <div 
    bind:this={chatContainer}
    class="flex-1 overflow-y-auto px-8 py-6 space-y-6 scrollbar-thin scrollbar-thumb-slate-900 scrollbar-track-transparent"
  >
    {#if visibleMessages.length === 0}
      <div class="h-full flex flex-col items-center justify-center text-center py-20 text-slate-500 space-y-3">
        <div class="p-4 bg-slate-900/40 rounded-full border border-slate-900/60 text-slate-400">
          <Sparkles size={32} />
        </div>
        <div class="max-w-md">
          <h3 class="text-sm font-bold text-slate-300">Nouvelle discussion commencée</h3>
          <p class="text-xs text-slate-450 mt-1 leading-relaxed">Saisissez un message ci-dessous pour démarrer. Vous pouvez ajuster le répertoire de travail et le modèle d'IA directement dans la barre d'outils au bas.</p>
        </div>
      </div>
    {:else}
      {#each visibleMessages as msg (msg.id)}
        <div class="flex w-full {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          {#if msg.role === 'user'}
            {#if editingMessageId === msg.id}
              <div class="flex flex-col gap-2 w-full max-w-[70%] no-drag">
                <div class="flex items-center gap-3 w-full bg-slate-900/20 border border-slate-900 focus-within:border-indigo-500/40 rounded-2xl px-4 py-2 transition-all relative">
                  <textarea
                    bind:value={editingMessageText}
                    bind:this={editAreaElement}
                    rows="1"
                    class="flex-1 bg-transparent text-sm text-slate-200 placeholder-slate-500 resize-none outline-none max-h-[240px] py-1.5 scrollbar-thin scrollbar-thumb-slate-900"
                  ></textarea>
                </div>
                <div class="flex justify-end gap-2 text-xs">
                  <button 
                    onclick={cancelEditing}
                    class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-350 rounded-md cursor-pointer transition-colors"
                  >
                    Annuler
                  </button>
                  <button 
                    onclick={() => saveEditedMessage(msg.id)}
                    class="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-md cursor-pointer font-bold transition-colors"
                  >
                    Valider
                  </button>
                </div>
              </div>
            {:else}
              <!-- User Message Bubble (aligned right) -->
              <div class="group relative flex items-start gap-2 max-w-[70%]">
                <!-- Edit button (visible on hover) -->
                <button
                  onclick={() => startEditing(msg)}
                  disabled={isThinking}
                  class="opacity-0 group-hover:opacity-100 p-1.5 text-slate-400 hover:text-slate-200 bg-slate-900/60 hover:bg-slate-900 border border-slate-800/80 rounded-lg cursor-pointer transition-all shrink-0 self-center disabled:opacity-0 disabled:cursor-not-allowed"
                  title="Modifier le message"
                >
                  <Pencil size={11} />
                </button>
                
                <div class="bg-gradient-to-br from-indigo-600 to-blue-600 text-white rounded-2xl rounded-tr-sm px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap shadow-md">
                  {msg.content}
                </div>
              </div>
            {/if}
          {:else}
            <!-- Assistant Message (Markdown HTML, left-aligned, no bubble) -->
            <div class="max-w-[85%] text-slate-200 text-sm leading-relaxed py-2 markdown-body w-full space-y-3">
              {#if msg.content}
                <div>{@html renderMarkdown(msg.content)}</div>
              {/if}
              
              {#if msg.tool_calls && msg.tool_calls.length > 0}
                <div class="space-y-3 border-l-2 border-slate-800 pl-4 py-1.5 mt-2 bg-slate-900/10 rounded-r-lg">
                  {#each msg.tool_calls as tc}
                    {@const response = messages.find(m => m.role === 'tool' && m.tool_call_id === tc.id)}
                    <div class="space-y-1">
                      <div class="flex items-center gap-2 text-xs font-semibold text-indigo-400">
                        <span class="p-1 rounded bg-indigo-500/10 text-indigo-400">🔧</span>
                        <span>Appel d'outil : {tc.function.name}</span>
                      </div>
                      {#if tc.function.arguments}
                        <pre class="bg-slate-950/60 p-2.5 rounded border border-slate-900/60 text-[11px] font-mono text-slate-300 overflow-x-auto max-w-full">{tc.function.arguments}</pre>
                      {/if}
                      
                      {#if response}
                        <details class="group mt-2">
                          <summary class="flex items-center gap-1 text-[11px] font-medium text-slate-500 hover:text-slate-300 cursor-pointer select-none outline-none">
                            <span class="transition-transform group-open:rotate-90">▶</span>
                            <span>Afficher le résultat de l'outil</span>
                          </summary>
                          <div class="mt-1.5 border border-slate-900 rounded bg-slate-950/40 p-3 text-slate-350 text-xs font-mono max-h-80 overflow-y-auto whitespace-pre-wrap leading-relaxed">
                            {response.content}
                          </div>
                        </details>
                      {:else}
                        <div class="flex items-center gap-1.5 text-[11px] text-slate-500 italic mt-1 animate-pulse">
                          <span class="inline-block w-1.5 h-1.5 rounded-full bg-slate-500"></span>
                          <span>En attente du résultat...</span>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    {/if}

    {#if thinkingStatus === 'thinking'}
      <div class="flex justify-start text-xs text-slate-500 font-mono animate-pulse py-1">
        <span>talos réfléchit...</span>
      </div>
    {:else if thinkingStatus === 'writing'}
      <div class="flex justify-start text-xs text-slate-500 font-mono animate-pulse py-1">
        <span>talos est en train d'écrire...</span>
      </div>
    {:else if thinkingStatus === 'executing'}
      <div class="flex justify-start text-xs text-slate-400 font-mono py-1 items-center gap-1.5">
        <span class="inline-block w-1.5 h-1.5 rounded-full bg-indigo-500 animate-ping"></span>
        <span class="animate-pulse">talos exécute les outils...</span>
      </div>
    {/if}
  </div>

  <!-- Bottom input and settings controls zone (Stretches to edges, border-t at top) -->
  <footer class="border-t border-slate-900 bg-slate-950/40 px-8 py-5 shrink-0">
    
    <!-- Hidden File Input -->
    <input 
      type="file" 
      multiple 
      class="hidden" 
      bind:this={fileInput} 
      onchange={handleFileChange} 
    />

    <!-- Attached files tags list (Minimalist tags above the input box) -->
    {#if attachedFiles.length > 0}
      <div class="flex flex-wrap gap-1.5 pb-3 no-drag">
        {#each attachedFiles as filename, index}
          <div class="flex items-center gap-1 px-2.5 py-1 bg-indigo-950/40 border border-indigo-900/40 text-indigo-400 rounded-md text-[10px] font-bold">
            <span>{filename}</span>
            <button 
              onclick={() => removeFile(index)} 
              class="hover:text-red-400 cursor-pointer p-0.5 rounded"
              title="Retirer"
            >
              <X size={10} />
            </button>
          </div>
        {/each}
      </div>
    {/if}

    <div class="flex flex-col gap-3">
      <!-- Message input card (Borderless text, round send button inside) -->
      <div class="flex items-center gap-3 w-full bg-slate-900/20 border border-slate-900 
        {currentMode === 'agent' 
          ? 'focus-within:border-indigo-500/40' 
          : currentMode === 'plan' 
            ? 'focus-within:border-sky-500/40' 
            : 'focus-within:border-emerald-500/40'
        } rounded-2xl px-4 py-2 transition-all relative">

        <!-- Slash command autocomplete dropdown -->
        {#if showSuggestions && filteredSuggestions.length > 0}
          <div class="absolute bottom-full left-0 right-0 mb-2 bg-[#0b0f19] border border-slate-900 rounded-xl shadow-2xl z-50 p-1.5 overflow-hidden">
            {#each filteredSuggestions as item, i}
              {@const isModelSugg = 'providerId' in item}
              <button
                onclick={() => {
                  if (isModelSugg) {
                    const ms = item as any;
                    activeProviderId = ms.providerId;
                    activeModel = ms.modelName;
                    if (window.talosAPI) {
                      window.talosAPI.setSetting('active_provider_id', ms.providerId);
                      window.talosAPI.setSetting('active_model_name', ms.modelName);
                    } else {
                      localStorage.setItem('talos_active_provider_id', ms.providerId);
                      localStorage.setItem('talos_active_model_name', ms.modelName);
                    }
                    inputMessage = '';
                  } else {
                    inputMessage = (item as any).name + ' ';
                  }
                  showSuggestions = false;
                  selectedSuggestionIndex = 0;
                  textareaElement?.focus();
                }}
                class="w-full text-left px-3 py-2 rounded-lg text-xs transition-all cursor-pointer flex items-center justify-between gap-2 {
                  i === selectedSuggestionIndex
                    ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/15'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'
                }"
              >
                {#if isModelSugg}
                  <span class="font-mono font-bold">{(item as any).label}</span>
                  <span class="text-slate-500 text-[10px]">Switch model</span>
                {:else}
                  <span class="font-mono font-bold">{(item as any).name}</span>
                  <span class="text-slate-500 truncate">{(item as any).desc}</span>
                {/if}
              </button>
            {/each}
            <div class="px-3 py-1.5 text-[10px] text-slate-600 border-t border-slate-900/80 mt-1 pt-2 flex items-center gap-3">
              <span><kbd class="text-slate-400 font-mono">↑↓</kbd> Navigate</span>
              <span><kbd class="text-slate-400 font-mono">⏎</kbd><kbd class="text-slate-400 font-mono ml-1">⇥</kbd> Select</span>
              <span><kbd class="text-slate-400 font-mono">⎋</kbd> Close</span>
            </div>
          </div>
        {/if}

        <textarea
          placeholder="Envoyez un message à votre agent..."
          bind:value={inputMessage}
          bind:this={textareaElement}
          onkeydown={handleKeydown}
          rows="1"
          class="flex-1 bg-transparent text-sm text-slate-200 placeholder-slate-500 resize-none outline-none max-h-[240px] py-1.5 scrollbar-thin scrollbar-thumb-slate-900 no-drag"
        ></textarea>
        
        {#if isThinking}
          <button
            type="button"
            onclick={stopChatStream}
            class="p-2.5 bg-red-600 hover:bg-red-500 text-white rounded-full transition-all cursor-pointer no-drag shrink-0 flex items-center justify-center shadow-md hover:scale-105"
            title="Interrompre la génération (Ctrl+C)"
          >
            <Square size={14} fill="white" />
          </button>
        {:else}
          <button
            type="button"
            onclick={sendMessage}
            disabled={!inputMessage.trim() && attachedFiles.length === 0}
            class="p-2.5 text-white rounded-full transition-all cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed no-drag shrink-0 flex items-center justify-center shadow-md hover:scale-105
              {currentMode === 'agent' 
                ? 'bg-indigo-600 hover:bg-indigo-500' 
                : currentMode === 'plan' 
                  ? 'bg-sky-600 hover:bg-sky-500' 
                  : 'bg-emerald-600 hover:bg-emerald-500'
              }"
            title="Envoyer le message"
          >
            <Send size={14} />
          </button>
        {/if}
      </div>
      
      <!-- Toolbar (Bottom of the zone): minimalist text line: path | model | join -->
      <div class="flex items-center justify-start gap-2.5 text-xs text-slate-500 font-medium px-1">
        
        <!-- CWD path (Truncated to avoid breaking small screens, but prints absolute path) -->
        <button 
          type="button"
          onclick={selectDirectory}
          class="hover:text-indigo-400 transition-colors cursor-pointer font-mono text-[11px] truncate max-w-[450px] flex items-center gap-1.5"
          title="Dossier de travail actuel (cliquez pour changer)"
        >
          <FolderOpen size={11} class="text-slate-500 shrink-0" />
          <span class="truncate">{cwd || 'Sélectionner un dossier'}</span>
        </button>

        <span class="text-slate-800">|</span>

        {#if !isSettingsLoading}
          <!-- Model Selector popover with text variant -->
          <ModelSelector 
            bind:activeProviderId 
            bind:activeModel 
            variant="text"
            onSelect={handleSelectModel} 
          />
        {/if}

        <span class="text-slate-800">|</span>

        <!-- Join Files paperclip icon button -->
        <button 
          type="button"
          onclick={triggerFileSelector}
          class="hover:text-indigo-400 transition-colors cursor-pointer flex items-center justify-center text-slate-500 hover:text-indigo-400"
          title="Joindre des fichiers"
        >
          <Paperclip size={14} />
          {#if attachedFiles.length > 0}
            <span class="text-indigo-400 font-bold font-mono text-[10px] ml-0.5">({attachedFiles.length})</span>
          {/if}
        </button>

      </div>

    </div>
  </footer>
</div>

<style>
  :global(.markdown-body p) {
    margin-bottom: 0.75rem;
  }
  :global(.markdown-body p:last-child) {
    margin-bottom: 0;
  }
  :global(.markdown-body pre) {
    background-color: #090d16;
    padding: 1rem;
    border-radius: 0.75rem;
    overflow-x: auto;
    margin-top: 0.5rem;
    margin-bottom: 0.75rem;
    border: 1px solid rgba(226, 232, 240, 0.06);
  }
  :global(.markdown-body code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.85em;
    background-color: rgba(99, 102, 241, 0.1);
    padding: 0.15em 0.3em;
    border-radius: 0.375rem;
    color: #a5b4fc;
  }
  :global(.markdown-body pre code) {
    background-color: transparent;
    padding: 0;
    font-size: 0.85rem;
    color: #f8fafc;
  }
  :global(.markdown-body ul) {
    list-style-type: disc;
    margin-left: 1.5rem;
    margin-bottom: 0.75rem;
  }
  :global(.markdown-body ol) {
    list-style-type: decimal;
    margin-left: 1.5rem;
    margin-bottom: 0.75rem;
  }
  :global(.markdown-body li) {
    margin-bottom: 0.25rem;
  }
  :global(.markdown-body h1) {
    font-size: 1.25rem;
    font-weight: 700;
    margin-top: 1rem;
    margin-bottom: 0.5rem;
    color: #ffffff;
  }
  :global(.markdown-body h2) {
    font-size: 1.1rem;
    font-weight: 700;
    margin-top: 1rem;
    margin-bottom: 0.5rem;
    color: #ffffff;
  }
  :global(.markdown-body strong) {
    font-weight: 700;
    color: #ffffff;
  }
  :global(.markdown-body a) {
    color: #818cf8;
    text-decoration: underline;
  }
  :global(.markdown-body a:hover) {
    color: #a5b4fc;
  }
</style>
