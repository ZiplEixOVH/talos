import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("talosAPI", {
  getChats: () => ipcRenderer.invoke('chats:get'),
  createChat: (id: string, title: string) => ipcRenderer.invoke('chats:create', id, title),
  deleteChat: (id: string) => ipcRenderer.invoke('chats:delete', id),
  renameChat: (id: string, title: string) => ipcRenderer.invoke('chats:rename', id, title),
  updateChatMode: (chatId: string, mode: string) => ipcRenderer.invoke('chats:update-mode', chatId, mode),
  getDbPath: () => ipcRenderer.invoke('db:path'),
  
  getProviders: () => ipcRenderer.invoke('providers:get'),
  saveProvider: (id: string, name: string, baseUrl: string, apiKey: string) => ipcRenderer.invoke('providers:save', id, name, baseUrl, apiKey),
  deleteProvider: (id: string) => ipcRenderer.invoke('providers:delete', id),
  
  getModels: (providerId: string) => ipcRenderer.invoke('models:get', providerId),
  addModel: (id: string, providerId: string, name: string) => ipcRenderer.invoke('models:add', id, providerId, name),
  deleteModel: (id: string) => ipcRenderer.invoke('models:delete', id),

  getMessages: (chatId: string) => ipcRenderer.invoke('messages:get', chatId),
  addMessage: (id: string, chatId: string, role: string, content: string, toolCalls?: any[], toolCallId?: string) => 
    ipcRenderer.invoke('messages:add', id, chatId, role, content, toolCalls, toolCallId),
  saveMessages: (chatId: string, messages: any[]) => ipcRenderer.invoke('messages:save', chatId, messages),
  
  getSetting: (key: string, defaultValue: string) => ipcRenderer.invoke('settings:get', key, defaultValue),
  setSetting: (key: string, value: string) => ipcRenderer.invoke('settings:set', key, value),
  
  getCwd: () => ipcRenderer.invoke('cwd:get'),
  selectCwd: () => ipcRenderer.invoke('cwd:select'),
  
  chat: (providerId: string, model: string, chatMessages: any[]) => ipcRenderer.invoke('openai:chat', providerId, model, chatMessages),
  
  startChatStream: (providerId: string, model: string, chatMessages: any[], chatId: string, requestId: string) => 
    ipcRenderer.send('openai:chat-stream-start', providerId, model, chatMessages, chatId, requestId),
  
  stopChatStream: (chatId: string) => 
    ipcRenderer.send('openai:chat-stream-stop', chatId),
    
  onChatStreamChunk: (callback: (data: { chatId: string; requestId: string; text: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:chat-stream-chunk', subscription);
    return () => {
      ipcRenderer.off('openai:chat-stream-chunk', subscription);
    };
  },
  
  onChatStreamEnd: (callback: (data: { chatId: string; requestId: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:chat-stream-end', subscription);
    return () => {
      ipcRenderer.off('openai:chat-stream-end', subscription);
    };
  },
  
  onChatStreamError: (callback: (data: { chatId: string; requestId: string; error: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:chat-stream-error', subscription);
    return () => {
      ipcRenderer.off('openai:chat-stream-error', subscription);
    };
  },
  
  onChatToolMessage: (callback: (data: { id: string; chatId: string; role: string; content: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:chat-tool-message', subscription);
    return () => {
      ipcRenderer.off('openai:chat-tool-message', subscription);
    };
  },
  
  getPrompts: () => ipcRenderer.invoke('prompts:list'),
  readPrompt: (name: string) => ipcRenderer.invoke('prompts:read', name),
  savePrompt: (name: string, content: string) => ipcRenderer.invoke('prompts:save', name, content),
  resetPrompt: (name: string) => ipcRenderer.invoke('prompts:reset', name),
  getTemplateVariables: () => ipcRenderer.invoke('prompts:template-variables'),
  saveMedia: (chatId: string, filename: string, base64Data: string) => ipcRenderer.invoke('chat:save-media', chatId, filename, base64Data),
  
  onSecurityRequestPermission: (callback: (data: { permissionId: string; chatId: string; type: 'bash' | 'file_access'; toolName: string; command?: string; path?: string; actionDescription: string; agentName?: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('security:request-permission', subscription);
    return () => {
      ipcRenderer.off('security:request-permission', subscription);
    };
  },
  respondSecurityPermission: (permissionId: string, approved: boolean) => ipcRenderer.send('security:response-permission', permissionId, approved),
  generateChatTitle: (chatId: string, firstMessage: string, providerId: string, model: string) => 
    ipcRenderer.invoke('chat:generate-title', chatId, firstMessage, providerId, model),
  
  onSubAgentsStarted: (callback: (data: { chatId: string; tasks: Array<{ agent_name: string; mission: string }> }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:sub-agents-started', subscription);
    return () => {
      ipcRenderer.off('openai:sub-agents-started', subscription);
    };
  },

  onSubAgentStatus: (callback: (data: { chatId: string; agent_name: string; status: string; isDone: boolean; error?: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('openai:sub-agent-status', subscription);
    return () => {
      ipcRenderer.off('openai:sub-agent-status', subscription);
    };
  },

  // ── Scheduler APIs ──────────────────────────────────────────────────────
  getSchedules: () => ipcRenderer.invoke('schedules:get'),
  saveSchedule: (task: any) => ipcRenderer.invoke('schedules:save', task),
  deleteSchedule: (id: string) => ipcRenderer.invoke('schedules:delete', id),
  runScheduleNow: (id: string) => ipcRenderer.invoke('schedules:run-now', id),
  
  onSchedulerTaskExecuted: (callback: (data: { taskId: string; chatId: string; last_run: number; last_result: string; next_run: number | null; total_runs: number; error?: boolean }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('scheduler:task-executed', subscription);
    return () => {
      ipcRenderer.off('scheduler:task-executed', subscription);
    };
  },

  onSchedulerChatCreated: (callback: (data: { chatId: string }) => void) => {
    const subscription = (_event: any, data: any) => callback(data);
    ipcRenderer.on('scheduler:chat-created', subscription);
    return () => {
      ipcRenderer.off('scheduler:chat-created', subscription);
    };
  },
});
