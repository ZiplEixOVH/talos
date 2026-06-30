import { app, BrowserWindow, ipcMain, dialog, Menu, MenuItem, protocol, net } from 'electron';
import { OpenAI } from 'openai';
import path from 'path';
import { fileURLToPath, pathToFileURL } from 'url';
import fsPromises from 'fs/promises';
import { existsSync, readFileSync } from 'fs';
import { initDb, getChats, createChat, deleteChat, renameChat, updateChatMode, getChatMode, getProviders, saveProvider, deleteProvider, getModels, addModel, deleteModel, getMessages, addMessage, saveMessages, getSetting, setSetting, getDbPath } from './db';
import { getOpenAITools, getOpenAIToolsForMode, executeTool, getToolParamValue, isCommandSafe, getToolPath, isPathAllowed } from './tools';
import { getSystemPrompt, getSubAgentPrompt } from './prompts';
import { TEMPLATE_VARIABLES, TEMPLATE_SYNTAX_HELP } from './promptVariables';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

protocol.registerSchemesAsPrivileged([
  {
    scheme: 'app',
    privileges: {
      standard: true,
      secure: true,
      supportFetchAPI: true,
      corsEnabled: true
    }
  }
]);

let mainWindow: BrowserWindow | null = null;

const pendingPermissions = new Map<string, (approved: boolean) => void>();

function registerAppProtocol() {
  const buildDir = path.resolve(__dirname, '..', 'build');

  protocol.handle('app', async (request) => {
    try {
      const url = new URL(request.url);
      let pathname = decodeURIComponent(url.pathname);

      if (pathname === '/' || pathname === '') {
        pathname = '/index.html';
      }

      const requestedPath = path.normalize(path.join(buildDir, pathname));
      if (!requestedPath.startsWith(buildDir)) {
        return new Response('Forbidden', { status: 403 });
      }

      let filePath = requestedPath;
      if (path.extname(filePath) === '' || !existsSync(filePath)) {
        filePath = path.join(buildDir, 'index.html');
      }

      if (!existsSync(filePath)) {
        return new Response('Not found', { status: 404 });
      }

      return net.fetch(pathToFileURL(filePath).toString());
    } catch (error) {
      console.error('Protocol handler error:', error);
      return new Response('Internal Server Error', { status: 500 });
    }
  });
}

function askUserPermission(
  window: BrowserWindow | null,
  data: {
    chatId: string;
    type: 'bash' | 'file_access';
    toolName: string;
    command?: string;
    path?: string;
    actionDescription: string;
    agentName?: string;
  }
): Promise<boolean> {
  return new Promise((resolve) => {
    if (!window) {
      resolve(false);
      return;
    }

    const permissionId = Math.random().toString(36).substring(2, 9);
    pendingPermissions.set(permissionId, resolve);

    // Send the request to the Svelte UI
    window.webContents.send('security:request-permission', {
      ...data,
      permissionId
    });
  });
}

// Global response handler for permission requests
ipcMain.on('security:response-permission', (_event, permissionId: string, approved: boolean) => {
  const resolve = pendingPermissions.get(permissionId);
  if (resolve) {
    resolve(approved);
    pendingPermissions.delete(permissionId);
  }
});

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
      spellcheck: true,
    },
  });

  const session = mainWindow.webContents.session;
  session.setSpellCheckerLanguages(['en-US', 'fr-FR']);

  mainWindow.webContents.on('context-menu', (event, params) => {
    const menu = new Menu();

    if (params.misspelledWord) {
      for (const suggestion of params.dictionarySuggestions) {
        menu.append(
          new MenuItem({
            label: suggestion,
            click: () => mainWindow?.webContents.replaceMisspelling(suggestion)
          })
        );
      }

      menu.append(new MenuItem({ type: 'separator' }));
      menu.append(
        new MenuItem({
          label: 'Add to Dictionary',
          click: () => mainWindow?.webContents.session.addWordToSpellCheckerDictionary(params.misspelledWord)
        })
      );

      menu.append(new MenuItem({ type: 'separator' }));
    }

    if (params.isEditable) {
      menu.append(new MenuItem({ label: 'Couper', role: 'cut' }));
      menu.append(new MenuItem({ label: 'Copier', role: 'copy' }));
      menu.append(new MenuItem({ label: 'Coller', role: 'paste' }));
    }

    if (menu.items.length > 0) {
      menu.popup({ window: mainWindow });
    }
  });

  if (process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
    mainWindow.webContents.openDevTools();
  } else {
    mainWindow.loadURL('app://index.html');
  }

  mainWindow.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL) => {
    console.error('Renderer failed to load:', { errorCode, errorDescription, validatedURL });
  });
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

ipcMain.handle('chats:rename', async (_, id: string, title: string) => {
  return await renameChat(id, title);
});

ipcMain.handle('chats:update-mode', async (_, chatId: string, mode: string) => {
  return await updateChatMode(chatId, mode);
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

ipcMain.handle('messages:add', async (_, id: string, chatId: string, role: string, content: string, toolCalls?: any[], toolCallId?: string) => {
  return await addMessage(id, chatId, role, content, toolCalls, toolCallId);
});

ipcMain.handle('messages:save', async (_, chatId: string, messages: any[]) => {
  return await saveMessages(chatId, messages);
});

// Handlers pour les réglages de l'application (modèle actif, etc.)
ipcMain.handle('settings:get', async (_, key: string, defaultValue: string) => {
  return await getSetting(key, defaultValue);
});

ipcMain.handle('settings:set', async (_, key: string, value: string) => {
  return await setSetting(key, value);
});

// Handler pour récupérer le chemin de la base de données
ipcMain.handle('db:path', async () => {
  return getDbPath();
});

// Handlers pour la gestion des prompts
ipcMain.handle('prompts:list', async () => {
  try {
    const promptsPath = path.join(getDbPath(), 'prompts');
    if (!existsSync(promptsPath)) {
      return [];
    }
    const files = await fsPromises.readdir(promptsPath);
    return files.filter(f => f.endsWith('.md'));
  } catch (error) {
    console.error('Failed to list prompts:', error);
    return [];
  }
});

// Expose les variables disponibles pour les templates de prompts
ipcMain.handle('prompts:template-variables', async () => {
  return {
    variables: TEMPLATE_VARIABLES,
    syntax: TEMPLATE_SYNTAX_HELP
  };
});

ipcMain.handle('prompts:read', async (_event, name: string) => {
  try {
    const filePath = path.join(getDbPath(), 'prompts', name);
    if (!existsSync(filePath)) {
      throw new Error(`File ${name} not found`);
    }
    return await fsPromises.readFile(filePath, 'utf-8');
  } catch (error) {
    console.error(`Failed to read prompt ${name}:`, error);
    throw error;
  }
});

ipcMain.handle('prompts:save', async (_event, name: string, content: string) => {
  try {
    const filePath = path.join(getDbPath(), 'prompts', name);
    await fsPromises.writeFile(filePath, content, 'utf-8');
    return true;
  } catch (error) {
    console.error(`Failed to save prompt ${name}:`, error);
    throw error;
  }
});

ipcMain.handle('prompts:reset', async (_event, name: string) => {
  try {
    const srcPromptsDir = existsSync(path.join(process.cwd(), 'prompts'))
      ? path.join(process.cwd(), 'prompts')
      : path.join(__dirname, '../prompts');
    
    const srcFile = path.join(srcPromptsDir, name);
    const destFile = path.join(getDbPath(), 'prompts', name);
    
    if (!existsSync(srcFile)) {
      throw new Error(`Default template for ${name} not found`);
    }
    
    const content = await fsPromises.readFile(srcFile, 'utf-8');
    await fsPromises.writeFile(destFile, content, 'utf-8');
    return content;
  } catch (error) {
    console.error(`Failed to reset prompt ${name}:`, error);
    throw error;
  }
});

ipcMain.handle('chat:save-media', async (_event, chatId: string, filename: string, base64Data: string) => {
  try {
    const chatsDir = path.join(getDbPath(), 'chats');
    const mediaDir = path.join(chatsDir, chatId, 'media');
    await fsPromises.mkdir(mediaDir, { recursive: true });
    
    const filePath = path.join(mediaDir, filename);
    const buffer = Buffer.from(base64Data, 'base64');
    await fsPromises.writeFile(filePath, buffer);
    
    return `file://${filePath}`;
  } catch (error) {
    console.error('Failed to save media:', error);
    throw error;
  }
});

ipcMain.handle('chat:generate-title', async (_, chatId: string, firstMessage: string, providerId: string, model: string) => {
  try {
    const providersList = await getProviders();
    const provider = providersList.find(p => p.id === providerId);
    if (!provider) {
      throw new Error(`Provider introuvable : ${providerId}`);
    }

    let baseUrl = provider.base_url;
    if (providerId === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
      baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
    }

    const client = new OpenAI({
      apiKey: provider.api_key || 'dummy-key',
      baseURL: baseUrl,
    });

    const prompt = `Generate a short title (four words or less) that describes the topic of the user's message.
Reply with only the title, nothing else. Do not show your reasoning.

Examples:
- "how do I reverse a list in python?" → Python list reversal
- "what's the weather in Tokyo?" → Tokyo weather
- "explain how transformers work in ML" → ML transformers explained

User message:
"${firstMessage}"`;

    const response = await client.chat.completions.create({
      model: model,
      messages: [{ role: 'user', content: prompt }],
      max_tokens: 15,
      temperature: 0.1,
    });

    const generatedTitle = response.choices[0].message.content?.trim().replace(/^["'«»“”]|["'«»“”]$/g, '') || 'Discussion';
    
    await renameChat(chatId, generatedTitle);
    return generatedTitle;
  } catch (error) {
    console.error('Failed to generate chat title:', error);
    return null;
  }
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

const activeStreams = new Map<string, { abort: () => void }>();

ipcMain.on('openai:chat-stream-stop', (_, chatId: string) => {
  const active = activeStreams.get(chatId);
  if (active) {
    active.abort();
    console.log(`[IPC] Aborted stream for chat: ${chatId}`);
  }
  // Cancel all pending security approvals immediately
  for (const [permissionId, resolve] of pendingPermissions.entries()) {
    resolve(false);
    pendingPermissions.delete(permissionId);
  }
});

function parseMessageContent(text: string): any {
  const parts: any[] = [];
  // Matches either file:///path/to/image or data:image/png;base64,...
  const regex = /!\[.*?\]\(((file:\/\/\/(.*?))|(data:(image\/[a-zA-Z+]+);base64,([a-zA-Z0-9+/=]+)))\)/g;
  let lastIndex = 0;
  let match;

  while ((match = regex.exec(text)) !== null) {
    const textBefore = text.substring(lastIndex, match.index);
    if (textBefore) {
      parts.push({ type: 'text', text: textBefore });
    }
    
    const isDataUrl = !!match[4]; // if data URL match group is present
    if (isDataUrl) {
      const fullDataUrl = match[1];
      parts.push({
        type: 'image_url',
        image_url: {
          url: fullDataUrl
        }
      });
    } else {
      const filePath = decodeURIComponent(match[3]);
      try {
        if (existsSync(filePath)) {
          const buffer = readFileSync(filePath);
          const ext = path.extname(filePath).replace('.', '').toLowerCase();
          const base64 = buffer.toString('base64');
          const mimeType = ext === 'jpg' || ext === 'jpeg' ? 'image/jpeg' : `image/${ext}`;
          parts.push({
            type: 'image_url',
            image_url: {
              url: `data:${mimeType};base64,${base64}`
            }
          });
        }
      } catch (e) {
        console.error('Failed to read image for multimodal payload:', e);
      }
    }
    lastIndex = regex.lastIndex;
  }

  const textAfter = text.substring(lastIndex);
  if (textAfter) {
    parts.push({ type: 'text', text: textAfter });
  }

  return parts.length > 1 ? parts : text;
}

async function runSubAgent(
  agentName: string,
  mission: string,
  chatId: string,
  providerId: string,
  model: string,
  tools: any[],
  window: BrowserWindow | null
): Promise<string> {
  try {
    const providersList = await getProviders();
    const provider = providersList.find(p => p.id === providerId);
    if (!provider) {
      throw new Error(`Provider introuvable : ${providerId}`);
    }

    let baseUrl = provider.base_url;
    if (providerId === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
      baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
    }

    const client = new OpenAI({
      apiKey: provider.api_key || 'dummy-key',
      baseURL: baseUrl,
    });

    const promptContent = await getSubAgentPrompt(agentName, mission, chatId);
    const subAgentMemory: any[] = [
      {
        role: 'system',
        content: promptContent
      }
    ];

    let isDone = false;
    let finalReport = '';
    let stepsCount = 0;
    const maxSteps = 15; // Sécurité contre les boucles infinies

    // Notifier le frontend que ce sous-agent démarre
    window?.webContents.send('openai:sub-agent-status', {
      chatId,
      agent_name: agentName,
      status: 'Initialisation...',
      isDone: false
    });

    while (!isDone && stepsCount < maxSteps) {
      stepsCount++;
      
      window?.webContents.send('openai:sub-agent-status', {
        chatId,
        agent_name: agentName,
        status: 'Réflexion...',
        isDone: false
      });

      const response = await client.chat.completions.create({
        model: model,
        messages: subAgentMemory,
        tools: tools.length > 0 ? tools : undefined
      });

      const message = response.choices[0].message;
      subAgentMemory.push(message);

      if (message.tool_calls && message.tool_calls.length > 0) {
        for (const tc of message.tool_calls) {
          window?.webContents.send('openai:sub-agent-status', {
            chatId,
            agent_name: agentName,
            status: `Appel de l'outil : ${tc.function.name}...`,
            isDone: false
          });

          let args: any = {};
          try {
            args = JSON.parse(tc.function.arguments);
          } catch (e) {}

          let toolResult = '';
          let isBlocked = false;

          // Sécurités
          if (tc.function.name === 'Bash') {
            const command = args.command || '';
            if (!isCommandSafe(command)) {
              toolResult = "error: Command execution blocked by security guardrails (forbidden pattern).";
              isBlocked = true;
            } else {
              const approved = await askUserPermission(window, {
                chatId,
                type: 'bash',
                toolName: 'Bash',
                command: command,
                actionDescription: `Commande demandée par le sous-agent ${agentName} : ${command}`,
                agentName: agentName
              });
              if (!approved) {
                toolResult = "error: User rejected the execution of this Bash command.";
                isBlocked = true;
              }
            }
          } else {
            const targetPath = getToolPath(tc.function.name, args);
            if (targetPath) {
              const allowed = isPathAllowed(targetPath, chatId);
              if (!allowed) {
                const approved = await askUserPermission(window, {
                  chatId,
                  type: 'file_access',
                  toolName: tc.function.name,
                  path: targetPath,
                  actionDescription: `Accès hors espace de travail demandé par le sous-agent ${agentName} (${tc.function.name}) : ${targetPath}`,
                  agentName: agentName
                });
                if (!approved) {
                  toolResult = `error: User rejected access to this path.`;
                  isBlocked = true;
                }
              }
            }
          }

          if (!isBlocked) {
            toolResult = await executeTool(tc.function.name, args, chatId);
          }

          subAgentMemory.push({
            role: 'tool',
            tool_call_id: tc.id,
            content: toolResult
          });
        }
      } else {
        finalReport = message.content || 'Aucun rapport fourni.';
        isDone = true;
      }
    }

    if (stepsCount >= maxSteps) {
      finalReport = `Le sous-agent a été arrêté car il a atteint la limite de ${maxSteps} étapes.`;
    }

    window?.webContents.send('openai:sub-agent-status', {
      chatId,
      agent_name: agentName,
      status: 'Terminé',
      isDone: true
    });

    return `### Rapport de ${agentName}\n\n**Mission** : ${mission}\n\n**Résultat** :\n${finalReport}`;
  } catch (err: any) {
    console.error(`[SubAgent ${agentName}] Error:`, err);
    window?.webContents.send('openai:sub-agent-status', {
      chatId,
      agent_name: agentName,
      status: `Erreur : ${err.message}`,
      isDone: true,
      error: err.message
    });
    return `### Rapport de ${agentName}\n\n**Mission** : ${mission}\n\n**Erreur** :\n${err.message}`;
  }
}

async function executeParallelAgents(
  tasks: any[],
  chatId: string,
  providerId: string,
  model: string,
  tools: any[],
  window: BrowserWindow | null
): Promise<string> {
  const agentPromises = tasks.map((task) =>
    runSubAgent(task.agent_name, task.mission, chatId, providerId, model, tools, window)
  );
  const results = await Promise.all(agentPromises);
  return results.join('\n\n---\n\n');
}

// Handler pour le streaming d'appels d'API OpenAI / Ollama avec exécution automatique d'outils
ipcMain.on('openai:chat-stream-start', async (event, providerId: string, model: string, chatMessages: any[], chatId: string, requestId: string) => {
  let currentRequestId = requestId;
  let aborted = false;
  const abortController = new AbortController();

  // Si un flux existe déjà pour ce chat, l'interrompre
  const existing = activeStreams.get(chatId);
  if (existing) {
    existing.abort();
  }

  activeStreams.set(chatId, {
    abort: () => {
      aborted = true;
      abortController.abort();
    }
  });

  let fullText = '';

  try {
    const providersList = await getProviders();
    const provider = providersList.find(p => p.id === providerId);
    if (!provider) {
      throw new Error(`Provider introuvable : ${providerId}`);
    }
    console.log(`[IPC] openai:chat-stream-start called:`, { providerId, model, resolvedBaseUrl: provider.base_url });
    
    // S'assurer que le chemin d'Ollama finit par /v1 pour le client officiel
    let baseUrl = provider.base_url;
    if (providerId === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
      baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
    }

    const client = new OpenAI({
      apiKey: provider.api_key || 'dummy-key',
      baseURL: baseUrl,
    });

    // Récupérer le mode du chat et compiler le prompt système associé
    const mode = await getChatMode(chatId);
    const systemPrompt = await getSystemPrompt(mode, chatId);
    console.log(`[Prompt Manager] Final system prompt for chat ${chatId} (mode: ${mode}):\n========================================\n${systemPrompt}\n========================================`);
    const globalSubagentsEnabled = (await getSetting('subagents_enabled', 'true')) === 'true';
    const chatSubagentsEnabled = (await getSetting(`chat_${chatId}_subagents_enabled`, 'true')) === 'true';
    const subagentsAllowed = globalSubagentsEnabled && chatSubagentsEnabled;

    let toolsForMode = getOpenAIToolsForMode(mode);
    if (!subagentsAllowed) {
      toolsForMode = toolsForMode.filter(t => t.function.name !== 'run_parallel_agents');
    }

    // Assainir l'historique et injecter le prompt système
    const apiMessages = [
      { role: 'system', content: systemPrompt },
      ...chatMessages.map((m: any) => {
        const msg: any = { role: m.role };
        if (m.role === 'user') {
          msg.content = parseMessageContent(m.content || '');
        } else {
          msg.content = m.content || '';
        }
        if (m.tool_calls) {
          msg.tool_calls = m.tool_calls;
        }
        if (m.tool_call_id) {
          msg.tool_call_id = m.tool_call_id;
        }
        return msg;
      })
    ];

    let continueAgentLoop = true;

    while (continueAgentLoop) {
      if (aborted) break;

      const streamParams: any = {
        model: model,
        messages: apiMessages,
      };

      let stream;
      try {
        if (toolsForMode.length > 0) {
          streamParams.tools = toolsForMode;
        } else {
          delete streamParams.tools;
        }
        stream = await client.chat.completions.create({
          ...streamParams,
          stream: true,
        }, { signal: abortController.signal });
      } catch (err: any) {
        if (aborted || err.name === 'AbortError') {
          throw err;
        }
        // Si le modèle ou fournisseur ne prend pas en charge les tools, on retombe en standard
        if (err.message && (err.message.includes('tools') || err.message.includes('tool_choice') || err.message.includes('not supported'))) {
          console.warn('Tools not supported by this model, falling back to standard completion.');
          delete streamParams.tools;
          stream = await client.chat.completions.create({
            ...streamParams,
            stream: true,
          }, { signal: abortController.signal });
        } else {
          throw err;
        }
      }

      fullText = '';
      const toolCallsAccumulator: any[] = [];

      for await (const chunk of stream) {
        if (aborted) break;
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

      if (aborted) break;

      // Filtrer pour éliminer les structures d'appels vides
      const actualToolCalls = toolCallsAccumulator.filter(tc => tc && tc.function.name);

      if (actualToolCalls.length > 0) {
        // Enregistrer l'appel de l'assistant dans la liste de messages indigène à OpenAI
        apiMessages.push({
          role: 'assistant',
          content: fullText || undefined,
          tool_calls: actualToolCalls
        });

        // Enregistrer l'appel de l'assistant dans la base de données
        await addMessage(currentRequestId, chatId, 'assistant', fullText, actualToolCalls);

        // Envoyer la notification de message outil à l'IHM
        event.sender.send('openai:chat-tool-message', {
          id: currentRequestId,
          chatId,
          role: 'assistant',
          content: fullText,
          tool_calls: actualToolCalls
        });

        // 2. Exécuter chaque outil et envoyer son résultat à l'IHM et au modèle
        for (const tc of actualToolCalls) {
          if (aborted) break;

          let args: any = {};
          try {
            args = JSON.parse(tc.function.arguments);
          } catch (e) {
            // Arguments JSON tronqués/malformés
          }

          let result = '';
          let isBlocked = false;

          // ── Security & Tool Interception ──────────────────────────────────
          if (tc.function.name === 'run_parallel_agents') {
            const tasks = args.tasks || [];
            console.log(`[Parallel Agents] Starting parallel execution for ${tasks.length} tasks...`);
            
            // Notify Svelte of parallel tasks start
            mainWindow?.webContents.send('openai:sub-agents-started', {
              chatId,
              tasks
            });

            // Filter out run_parallel_agents to prevent subagents from spawning nested subagents
            const subTools = toolsForMode.filter(t => t.function.name !== 'run_parallel_agents');
            
            result = await executeParallelAgents(tasks, chatId, providerId, model, subTools, mainWindow);
            isBlocked = true;
          } else if (tc.function.name === 'Bash') {
            const command = args.command || '';
            
            // 1. Guardrail blacklist check
            if (!isCommandSafe(command)) {
              result = "error: Command execution blocked by security guardrails. The command contains forbidden patterns (destructive actions).";
              isBlocked = true;
            } else {
              // 2. Human approval check
              console.log(`[Security] Requesting permission for Bash command: "${command}"`);
              const approved = await askUserPermission(mainWindow, {
                chatId,
                type: 'bash',
                toolName: 'Bash',
                command: command,
                actionDescription: `Exécution de la commande Bash : ${command}`
              });
              
              if (!approved) {
                result = "error: User rejected the execution of this Bash command for security reasons.";
                isBlocked = true;
              }
            }
          } else {
            const targetPath = getToolPath(tc.function.name, args);
            if (targetPath) {
              const allowed = isPathAllowed(targetPath, chatId);
              if (!allowed) {
                console.log(`[Security] Requesting permission for path access: "${targetPath}" (${tc.function.name})`);
                const approved = await askUserPermission(mainWindow, {
                  chatId,
                  type: 'file_access',
                  toolName: tc.function.name,
                  path: targetPath,
                  actionDescription: `Accès hors espace de travail (${tc.function.name}) : ${targetPath}`
                });
                
                if (!approved) {
                  result = `error: User rejected the access to this path (${targetPath}) for security reasons.`;
                  isBlocked = true;
                }
              }
            }
          }

          if (!isBlocked) {
            // Exécuter l'outil si non bloqué
            result = await executeTool(tc.function.name, args, chatId);
          }
          // ──────────────────────────────────────────────────────────────────

          if (aborted) break;

          // Ajouter le résultat dans l'historique OpenAI natif pour le prochain tour
          apiMessages.push({
            role: 'tool',
            tool_call_id: tc.id,
            content: result
          });

          // Enregistrer et notifier le renderer
          const toolResultMsgId = `msg-${Math.random().toString(36).substring(2, 9)}`;
          await addMessage(toolResultMsgId, chatId, 'tool', result, undefined, tc.id);
          event.sender.send('openai:chat-tool-message', {
            id: toolResultMsgId,
            chatId,
            role: 'tool',
            content: result,
            tool_call_id: tc.id
          });
        }

        if (aborted) break;

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
    if (aborted || err.name === 'AbortError') {
      console.log(`[Stream] Stream for chat ${chatId} successfully aborted.`);
      if (fullText) {
        try {
          await addMessage(currentRequestId, chatId, 'assistant', fullText);
        } catch (dbErr) {
          console.error('Error saving partial text on abort:', dbErr);
        }
      }
      event.sender.send('openai:chat-stream-end', { chatId, requestId: currentRequestId });
    } else {
      console.error('Error in openai:chat-stream-start:', err);
      event.sender.send('openai:chat-stream-error', { 
        chatId, 
        requestId: currentRequestId, 
        error: err instanceof Error ? err.message : String(err) 
      });
    }
  } finally {
    if (activeStreams.get(chatId)?.abort === abortController.abort) {
      activeStreams.delete(chatId);
    }
  }
});


app.whenReady().then(async () => {
  registerAppProtocol();

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

  // Create standard menu for copy-paste on macOS and general application shortcuts
  const template: any[] = [
    ...(process.platform === 'darwin' ? [{
      label: app.name,
      submenu: [
        { role: 'about', label: 'À propos' },
        { type: 'separator' },
        { role: 'services', label: 'Services' },
        { type: 'separator' },
        { role: 'hide', label: 'Masquer' },
        { role: 'hideOthers', label: 'Masquer les autres' },
        { role: 'unhide', label: 'Tout afficher' },
        { type: 'separator' },
        { role: 'quit', label: 'Quitter' }
      ]
    }] : []),
    {
      label: 'Édition',
      submenu: [
        { role: 'undo', label: 'Annuler' },
        { role: 'redo', label: 'Rétablir' },
        { type: 'separator' },
        { role: 'cut', label: 'Couper' },
        { role: 'copy', label: 'Copier' },
        { role: 'paste', label: 'Coller' },
        ...(process.platform === 'darwin' ? [
          { role: 'selectAll', label: 'Tout sélectionner' }
        ] : [
          { type: 'separator' },
          { role: 'selectAll', label: 'Tout sélectionner' }
        ])
      ]
    },
    {
      label: 'Présentation',
      submenu: [
        { role: 'reload', label: 'Recharger' },
        { role: 'forceReload', label: 'Forcer le rechargement' },
        { role: 'toggleDevTools', label: 'Outils de développement' },
        { type: 'separator' },
        { role: 'resetZoom', label: 'Taille réelle' },
        { role: 'zoomIn', label: 'Zoom avant' },
        { role: 'zoomOut', label: 'Zoom arrière' },
        { type: 'separator' },
        { role: 'togglefullscreen', label: 'Plein écran' }
      ]
    },
    {
      label: 'Fenêtre',
      submenu: [
        { role: 'minimize', label: 'Placer dans le Dock' },
        { role: 'zoom', label: 'Zoom' },
        ...(process.platform === 'darwin' ? [
          { type: 'separator' },
          { role: 'front', label: 'Tout ramener au premier plan' }
        ] : [
          { role: 'close', label: 'Fermer' }
        ])
      ]
    }
  ];

  const menu = Menu.buildFromTemplate(template);
  Menu.setApplicationMenu(menu);

  createWindow();
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});
