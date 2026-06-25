import { app, BrowserWindow, ipcMain, dialog } from 'electron';
import { OpenAI } from 'openai';
import path from 'path';
import { fileURLToPath } from 'url';
import { initDb, getChats, createChat, deleteChat, getProviders, saveProvider, deleteProvider, getModels, addModel, deleteModel, getMessages, addMessage, getSetting, setSetting } from './db';
import { getOpenAITools, executeTool } from './tools';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

let mainWindow: BrowserWindow | null = null;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    titleBarStyle: 'hidden',
    titleBarOverlay: {
        color: '#111827',
        symbolColor: '#747d8c',
        height: 32
    },
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  if (process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
    mainWindow.webContents.openDevTools();
  } else {
    // SvelteKit génère son build dans le dossier /build (et non /dist)
    mainWindow.loadFile(path.join(__dirname, '../build/index.html'));
  }
}

// Enregistrement des IPC handlers pour SQLite
ipcMain.handle('chats:get', async () => {
  return await getChats();
});

ipcMain.handle('chats:create', async (_, id: string, title: string) => {
  return await createChat(id, title);
});

ipcMain.handle('chats:delete', async (_, id: string) => {
  return await deleteChat(id);
});

// Handlers pour les Providers et Modèles
ipcMain.handle('providers:get', async () => {
  return await getProviders();
});

ipcMain.handle('providers:save', async (_, id: string, name: string, baseUrl: string, apiKey: string) => {
  return await saveProvider(id, name, baseUrl, apiKey);
});

ipcMain.handle('providers:delete', async (_, id: string) => {
  return await deleteProvider(id);
});

ipcMain.handle('models:get', async (_, providerId: string) => {
  return await getModels(providerId);
});

ipcMain.handle('models:add', async (_, id: string, providerId: string, name: string) => {
  return await addModel(id, providerId, name);
});

ipcMain.handle('models:delete', async (_, id: string) => {
  return await deleteModel(id);
});

// Handlers pour les messages des chats
ipcMain.handle('messages:get', async (_, chatId: string) => {
  return await getMessages(chatId);
});

ipcMain.handle('messages:add', async (_, id: string, chatId: string, role: string, content: string) => {
  return await addMessage(id, chatId, role, content);
});

// Handlers pour les réglages de l'application (modèle actif, etc.)
ipcMain.handle('settings:get', async (_, key: string, defaultValue: string) => {
  return await getSetting(key, defaultValue);
});

ipcMain.handle('settings:set', async (_, key: string, value: string) => {
  return await setSetting(key, value);
});

// Handlers pour le dossier de travail actuel (CWD)
ipcMain.handle('cwd:get', () => {
  return process.cwd();
});

ipcMain.handle('cwd:select', async () => {
  const result = await dialog.showOpenDialog({
    properties: ['openDirectory'],
    title: 'Choisir le dossier de travail'
  });
  if (result.canceled) return null;
  const selectedPath = result.filePaths[0];
  try {
    process.chdir(selectedPath);
    await setSetting('talos_cwd', selectedPath);
    console.log('Working directory changed to:', selectedPath);
  } catch (err) {
    console.error('Failed to change process directory:', err);
  }
  return selectedPath;
});

// Handler pour l'exécution d'appels d'API OpenAI / Ollama
ipcMain.handle('openai:chat', async (_, providerId: string, model: string, chatMessages: any[]) => {
  const providersList = await getProviders();
  const provider = providersList.find(p => p.id === providerId);
  if (!provider) {
    throw new Error(`Provider introuvable : ${providerId}`);
  }
  
  // S'assurer que le chemin d'Ollama finit par /v1 pour le client officiel
  let baseUrl = provider.base_url;
  if (providerId === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
    baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
  }

  const client = new OpenAI({
    apiKey: provider.api_key || 'dummy-key',
    baseURL: baseUrl,
  });

  const response = await client.chat.completions.create({
    model: model,
    messages: chatMessages.map(m => ({ role: m.role, content: m.content })),
  });

  return response.choices[0].message;
});

// Handler pour le streaming d'appels d'API OpenAI / Ollama avec exécution automatique d'outils
ipcMain.on('openai:chat-stream-start', async (event, providerId: string, model: string, chatMessages: any[], chatId: string, requestId: string) => {
  let currentRequestId = requestId;
  try {
    const providersList = await getProviders();
    const provider = providersList.find(p => p.id === providerId);
    if (!provider) {
      throw new Error(`Provider introuvable : ${providerId}`);
    }
    
    // S'assurer que le chemin d'Ollama finit par /v1 pour le client officiel
    let baseUrl = provider.base_url;
    if (providerId === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
      baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
    }

    const client = new OpenAI({
      apiKey: provider.api_key || 'dummy-key',
      baseURL: baseUrl,
    });

    // Définir le prompt système dynamique avec le CWD actuel
    const currentCwd = process.cwd();
    const systemPrompt = `Tu es Talos, un assistant de code intelligent.
Le répertoire de travail actuel (CWD) est : ${currentCwd}.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`;

    // Assainir l'historique et injecter le prompt système
    const apiMessages = [
      { role: 'system', content: systemPrompt },
      ...chatMessages.map((m: any) => {
        if (m.role === 'tool') {
          return { role: 'system', content: `[Sortie outil historique] : ${m.content}` };
        }
        return { role: m.role, content: m.content };
      })
    ];

    let continueAgentLoop = true;

    while (continueAgentLoop) {
      const streamParams: any = {
        model: model,
        messages: apiMessages,
      };

      let stream;
      try {
        streamParams.tools = getOpenAITools();
        stream = await client.chat.completions.create({
          ...streamParams,
          stream: true,
        });
      } catch (err: any) {
        // Si le modèle ou fournisseur ne prend pas en charge les tools, on retombe en standard
        if (err.message && (err.message.includes('tools') || err.message.includes('tool_choice') || err.message.includes('not supported'))) {
          console.warn('Tools not supported by this model, falling back to standard completion.');
          delete streamParams.tools;
          stream = await client.chat.completions.create({
            ...streamParams,
            stream: true,
          });
        } else {
          throw err;
        }
      }

      let fullText = '';
      const toolCallsAccumulator: any[] = [];

      for await (const chunk of stream) {
        const choice = chunk.choices[0];
        if (!choice) continue;

        const delta = choice.delta;

        // Diffuser les fragments de texte
        const text = delta.content || '';
        if (text) {
          fullText += text;
          event.sender.send('openai:chat-stream-chunk', { chatId, requestId: currentRequestId, text });
        }

        // Accumuler les fragments d'appels d'outils
        if (delta.tool_calls) {
          for (const tc of delta.tool_calls) {
            const idx = tc.index;
            if (!toolCallsAccumulator[idx]) {
              toolCallsAccumulator[idx] = {
                id: tc.id || '',
                type: tc.type || 'function',
                function: {
                  name: tc.function?.name || '',
                  arguments: tc.function?.arguments || ''
                }
              };
            } else {
              if (tc.id) toolCallsAccumulator[idx].id = tc.id;
              if (tc.function?.name) toolCallsAccumulator[idx].function.name += tc.function.name;
              if (tc.function?.arguments) toolCallsAccumulator[idx].function.arguments += tc.function.arguments;
            }
          }
        }
      }

      // Filtrer pour éliminer les structures d'appels vides
      const actualToolCalls = toolCallsAccumulator.filter(tc => tc && tc.function.name);

      if (actualToolCalls.length > 0) {
        // Enregistrer l'appel de l'assistant dans la liste de messages indigène à OpenAI
        apiMessages.push({
          role: 'assistant',
          content: fullText || undefined,
          tool_calls: actualToolCalls
        });

        // 1. Synthétiser l'appel d'outil sous forme textuelle lisible pour SQLite & l'IHM Svelte
        const toolCallSummaries = actualToolCalls.map(tc => {
          return `🔧 **Outil** : \`${tc.function.name}(${tc.function.arguments.trim()})\``;
        }).join('\n');

        const assistantToolMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        await addMessage(assistantToolMsgId, chatId, 'assistant', toolCallSummaries);
        event.sender.send('openai:chat-tool-message', {
          id: assistantToolMsgId,
          chatId,
          role: 'assistant',
          content: toolCallSummaries
        });

        // 2. Exécuter chaque outil et envoyer son résultat à l'IHM et au modèle
        for (const tc of actualToolCalls) {
          let args: any = {};
          try {
            args = JSON.parse(tc.function.arguments);
          } catch (e) {
            // Arguments JSON tronqués/malformés
          }

          // Exécuter l'outil
          const result = await executeTool(tc.function.name, args);

          // Ajouter le résultat dans l'historique OpenAI natif pour le prochain tour
          apiMessages.push({
            role: 'tool',
            tool_call_id: tc.id,
            content: result
          });

          // Enregistrer et notifier le renderer
          const toolResultMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
          const toolResultFormatted = `📥 **Résultat** : \n\`\`\`\n${result}\n\`\`\``;
          await addMessage(toolResultMsgId, chatId, 'tool', toolResultFormatted);
          event.sender.send('openai:chat-tool-message', {
            id: toolResultMsgId,
            chatId,
            role: 'tool',
            content: toolResultFormatted
          });
        }

        // On génère un nouvel identifiant pour la réponse finale ou la prochaine vague d'outils
        currentRequestId = `msg-${Math.random().toString(36).substring(2, 9)}`;
        // Notifier le renderer de créer un placeholder pour ce nouveau message
        event.sender.send('openai:chat-tool-message', {
          id: currentRequestId,
          chatId,
          role: 'assistant',
          content: ''
        });

        // La boucle continue avec l'historique enrichi des résultats d'outils
      } else {
        // Enregistrer la réponse finale dans SQLite
        await addMessage(currentRequestId, chatId, 'assistant', fullText);

        // Terminer le flux pour l'IHM
        event.sender.send('openai:chat-stream-end', { chatId, requestId: currentRequestId });
        continueAgentLoop = false;
      }
    }
  } catch (err: any) {
    console.error('Error in openai:chat-stream-start:', err);
    event.sender.send('openai:chat-stream-error', { 
      chatId, 
      requestId: currentRequestId, 
      error: err instanceof Error ? err.message : String(err) 
    });
  }
});


app.whenReady().then(async () => {
  try {
    await initDb();
    const savedCwd = await getSetting('talos_cwd', '');
    if (savedCwd) {
      try {
        process.chdir(savedCwd);
        console.log('Restored working directory on startup to:', savedCwd);
      } catch (e) {
        console.error('Failed to restore working directory on startup:', e);
      }
    }
  } catch (err) {
    console.error('Erreur lors de l\'initialisation de la base de données :', err);
  }
  createWindow();
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});
