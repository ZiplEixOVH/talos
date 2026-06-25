<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { page } from '$app/stores';
  import { 
    Send, Bot, User, Cpu, Sparkles, FolderOpen, 
    Paperclip, X, RefreshCw, AlertCircle 
  } from 'lucide-svelte';
  import { marked } from 'marked';
  import ModelSelector from '$lib/components/ModelSelector.svelte';

  // Récupération de l'id réactif depuis la route
  let chatId = $derived($page.params.id || '');

  let chatTitle = $state('Discussion');
  let messages = $state<Array<{ id: string; role: string; content: string }>>([]);

  let streamCleanups: (() => void)[] = [];
  function clearStreamSubscriptions() {
    streamCleanups.forEach(unsub => unsub());
    streamCleanups = [];
  }

  onDestroy(() => {
    clearStreamSubscriptions();
  });
  let inputMessage = $state('');
  let isThinking = $state(false);
  let chatContainer = $state<HTMLDivElement | null>(null);

  // Configuration de l'environnement actif
  let cwd = $state('');
  let activeProviderId = $state('ollama');
  let activeModel = $state('');
  let isSettingsLoading = $state(true);

  // Pièces jointes
  let attachedFiles = $state<string[]>([]);
  let fileInput = $state<HTMLInputElement | null>(null);

  // Nom abrégé du dossier de travail (CWD)
  let folderName = $derived(cwd ? (cwd.split(/[/\\]/).pop() || cwd) : 'Dossier');
  
  let textareaElement = $state<HTMLTextAreaElement | null>(null);

  // Auto-resize the input textarea height based on content
  $effect(() => {
    const _val = inputMessage;
    if (textareaElement) {
      textareaElement.style.height = 'auto';
      textareaElement.style.height = `${textareaElement.scrollHeight}px`;
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
      loadConversationData(chatId);
    }
  });

  onMount(() => {
    loadInitialSettings();

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
    isThinking = false;
    messages = [];
    attachedFiles = [];

    // 1. Charger le titre de la discussion
    let foundTitle = 'Discussion';
    if (window.talosAPI) {
      try {
        const chats = await window.talosAPI.getChats();
        const chat = chats.find(c => c.id === id);
        if (chat) foundTitle = chat.title;
      } catch (err) {
        console.error(err);
      }
    } else {
      const saved = localStorage.getItem('talos_chats');
      if (saved) {
        const chats = JSON.parse(saved);
        const chat = chats.find((c: any) => c.id === id);
        if (chat) foundTitle = chat.title;
      }
    }
    chatTitle = foundTitle;

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
      const names = Array.from(target.files).map(file => file.name);
      attachedFiles = [...attachedFiles, ...names];
    }
  }

  function removeFile(index: number) {
    attachedFiles = attachedFiles.filter((_, i) => i !== index);
  }

  async function sendMessage() {
    const text = inputMessage.trim();
    if (!text && attachedFiles.length === 0) return;

    // S'assurer qu'un modèle est sélectionné
    if (!activeModel) {
      messages = [...messages, {
        id: `err-${Date.now()}`,
        role: 'assistant',
        content: 'Veuillez sélectionner un modèle dans les outils au bas de l\'écran avant d\'envoyer un message.'
      }];
      await scrollToBottom();
      return;
    }

    const textWithFiles = attachedFiles.length > 0 
      ? `[Fichiers joints: ${attachedFiles.join(', ')}]\n\n${text}` 
      : text;

    inputMessage = '';
    attachedFiles = [];

    const userMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
    const userMsg = { id: userMsgId, role: 'user', content: textWithFiles };

    // Ajout à l'interface
    messages = [...messages, userMsg];
    
    // Sauvegarde en base
    if (window.talosAPI) {
      try {
        await window.talosAPI.addMessage(userMsgId, chatId, 'user', textWithFiles);
      } catch (err) {
        console.error(err);
        saveMessageToLocalStorage(chatId, userMsg);
      }
    } else {
      saveMessageToLocalStorage(chatId, userMsg);
    }

    await scrollToBottom();

    // Lancer la réflexion
    isThinking = true;
    try {
      if (window.talosAPI) {
        // Envoi en mode streaming réel via Electron
        const plainMessages = messages.map(m => ({ role: m.role, content: m.content }));
        const cleanMessages = $state.snapshot(plainMessages);
        
        const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        const assistantMsg = { id: aiMsgId, role: 'assistant', content: '' };
        messages = [...messages, assistantMsg];
        await scrollToBottom();

        // Nettoyer les abonnements précédents avant de démarrer un nouveau stream
        clearStreamSubscriptions();

        const unsubChunk = window.talosAPI.onChatStreamChunk((data) => {
          if (data.chatId === chatId) {
            const idx = messages.findIndex(m => m.id === data.requestId);
            if (idx !== -1) {
              messages[idx] = {
                ...messages[idx],
                content: messages[idx].content + data.text
              };
              messages = [...messages]; // Force Svelte reactivity update
              if (isThinking) {
                isThinking = false;
              }
              scrollToBottom();
            }
          }
        });

        const unsubEnd = window.talosAPI.onChatStreamEnd((data) => {
          if (data.chatId === chatId) {
            clearStreamSubscriptions();
            isThinking = false;
            scrollToBottom();
          }
        });

        const unsubError = window.talosAPI.onChatStreamError((data) => {
          if (data.chatId === chatId) {
            clearStreamSubscriptions();
            isThinking = false;
            const idx = messages.findIndex(m => m.id === data.requestId);
            if (idx !== -1) {
              messages[idx] = {
                ...messages[idx],
                content: messages[idx].content + `\n\n*(Erreur lors du streaming : ${data.error})*`
              };
              messages = [...messages]; // Force Svelte reactivity update
            }
            scrollToBottom();
          }
        });

        const unsubToolMessage = window.talosAPI.onChatToolMessage((data) => {
          if (data.chatId === chatId) {
            const idx = messages.findIndex(m => m.id === data.id);
            if (idx === -1) {
              messages = [...messages, data]; // Force Svelte reactivity update on push
            } else {
              messages[idx] = data;
              messages = [...messages]; // Force Svelte reactivity update on modification
            }
            if (isThinking) {
              isThinking = false;
            }
            scrollToBottom();
          }
        });

        streamCleanups.push(unsubChunk, unsubEnd, unsubError, unsubToolMessage);

        window.talosAPI.startChatStream(activeProviderId, activeModel, cleanMessages, chatId, aiMsgId);
      } else {
        // Simulation en mode fallback localStorage
        await new Promise(r => setTimeout(r, 1200));
        const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        const content = `[Simulation Fallback Browser]\nModèle sélectionné : ${activeModel}\nFournisseur : ${activeProviderId}\nDossier de travail : ${cwd}\n\nVotre message a été reçu ! Pour exécuter de vrais appels d'API, veuillez lancer l'application avec Electron et configurer un fournisseur de clés.`;
        const assistantMsg = { id: aiMsgId, role: 'assistant', content };
        
        messages = [...messages, assistantMsg];
        saveMessageToLocalStorage(chatId, assistantMsg);
        isThinking = false;
        await scrollToBottom();
      }
    } catch (err: any) {
      console.error(err);
      isThinking = false;
      const aiMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
      const errMsg = { id: aiMsgId, role: 'assistant', content: `Désolé, une erreur s'est produite lors de l'appel d'API : ${err.message || err}` };
      messages.push(errMsg);
      await scrollToBottom();
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }
</script>

<div class="flex flex-col h-full w-full bg-transparent overflow-hidden">
  
  <!-- Messages List Feed (Takes all upper space, padded nicely so text isn't against screen edges) -->
  <div 
    bind:this={chatContainer}
    class="flex-1 overflow-y-auto px-8 py-6 space-y-6 scrollbar-thin scrollbar-thumb-slate-900 scrollbar-track-transparent"
  >
    {#if messages.filter(m => m.role !== 'tool' && m.content !== '').length === 0}
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
      {#each messages.filter(m => m.role !== 'tool' && m.content !== '') as msg (msg.id)}
        <div class="flex w-full {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
          {#if msg.role === 'user'}
            <!-- User Message Bubble (aligned right) -->
            <div class="max-w-[70%] bg-gradient-to-br from-indigo-600 to-blue-600 text-white rounded-2xl rounded-tr-sm px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap shadow-md">
              {msg.content}
            </div>
          {:else}
            <!-- Assistant Message (Markdown HTML, left-aligned, no bubble) -->
            <div class="max-w-[85%] text-slate-200 text-sm leading-relaxed py-2 markdown-body w-full">
              {@html renderMarkdown(msg.content)}
            </div>
          {/if}
        </div>
      {/each}
    {/if}

    {#if isThinking}
      <div class="flex justify-start text-xs text-slate-500 font-mono animate-pulse py-1">
        <span>talos est en train d'écrire...</span>
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
      <div class="flex items-center gap-3 w-full bg-slate-900/20 border border-slate-900 focus-within:border-indigo-500/40 rounded-2xl px-4 py-2 transition-all relative">
        <textarea
          placeholder="Envoyez un message à votre agent..."
          bind:value={inputMessage}
          bind:this={textareaElement}
          onkeydown={handleKeydown}
          rows="1"
          class="flex-1 bg-transparent text-sm text-slate-200 placeholder-slate-500 resize-none outline-none max-h-[240px] py-1.5 scrollbar-thin scrollbar-thumb-slate-900 no-drag"
        ></textarea>
        
        <button
          type="button"
          onclick={sendMessage}
          disabled={(!inputMessage.trim() && attachedFiles.length === 0) || isThinking}
          class="p-2.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-full transition-all cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed no-drag shrink-0 flex items-center justify-center shadow-md hover:scale-105"
          title="Envoyer le message"
        >
          <Send size={14} />
        </button>
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
