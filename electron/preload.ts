import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("talosAPI", {
  getChats: () => ipcRenderer.invoke('chats:get'),
  createChat: (id: string, title: string) => ipcRenderer.invoke('chats:create', id, title),
  deleteChat: (id: string) => ipcRenderer.invoke('chats:delete', id),
  
  getProviders: () => ipcRenderer.invoke('providers:get'),
  saveProvider: (id: string, name: string, baseUrl: string, apiKey: string) => ipcRenderer.invoke('providers:save', id, name, baseUrl, apiKey),
  deleteProvider: (id: string) => ipcRenderer.invoke('providers:delete', id),
  
  getModels: (providerId: string) => ipcRenderer.invoke('models:get', providerId),
  addModel: (id: string, providerId: string, name: string) => ipcRenderer.invoke('models:add', id, providerId, name),
  deleteModel: (id: string) => ipcRenderer.invoke('models:delete', id),

  getMessages: (chatId: string) => ipcRenderer.invoke('messages:get', chatId),
  addMessage: (id: string, chatId: string, role: string, content: string) => ipcRenderer.invoke('messages:add', id, chatId, role, content),
  
  getSetting: (key: string, defaultValue: string) => ipcRenderer.invoke('settings:get', key, defaultValue),
  setSetting: (key: string, value: string) => ipcRenderer.invoke('settings:set', key, value),
  
  getCwd: () => ipcRenderer.invoke('cwd:get'),
  selectCwd: () => ipcRenderer.invoke('cwd:select'),
  
  chat: (providerId: string, model: string, chatMessages: any[]) => ipcRenderer.invoke('openai:chat', providerId, model, chatMessages),
  
  startChatStream: (providerId: string, model: string, chatMessages: any[], chatId: string, requestId: string) => 
    ipcRenderer.send('openai:chat-stream-start', providerId, model, chatMessages, chatId, requestId),
    
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
});
