import path from 'path';
import { app } from 'electron';
import fs from 'fs/promises';
import { existsSync } from 'fs';

const TALOS_DIR = path.join(app.getPath('home'), '.talos');
const CHATS_DIR = path.join(TALOS_DIR, 'chats');
const SETTINGS_FILE = path.join(TALOS_DIR, 'settings.json');
const PROVIDERS_FILE = path.join(TALOS_DIR, 'providers.json');
const MODELS_FILE = path.join(TALOS_DIR, 'models.json');

// Helper to safely read a JSON file with a fallback default value
async function readJsonFile<T>(filePath: string, defaultValue: T): Promise<T> {
  try {
    if (!existsSync(filePath)) {
      return defaultValue;
    }
    const content = await fs.readFile(filePath, 'utf-8');
    return JSON.parse(content) as T;
  } catch (error) {
    console.error(`Error reading file ${filePath}:`, error);
    return defaultValue;
  }
}

// Helper to safely write a JSON file atomically using a temp file
async function writeJsonFile<T>(filePath: string, data: T): Promise<void> {
  try {
    const tempPath = `${filePath}.tmp`;
    await fs.writeFile(tempPath, JSON.stringify(data, null, 2), 'utf-8');
    await fs.rename(tempPath, filePath);
  } catch (error) {
    console.error(`Error writing file ${filePath}:`, error);
    throw error;
  }
}

export function getDbPath(): string {
  return TALOS_DIR;
}

export async function initDb(): Promise<void> {
  // 1. Ensure directory structures exist
  await fs.mkdir(TALOS_DIR, { recursive: true });
  await fs.mkdir(CHATS_DIR, { recursive: true });

  // 2. Ensure files exist with proper initial values
  if (!existsSync(SETTINGS_FILE)) {
    await writeJsonFile(SETTINGS_FILE, {});
  }

  if (!existsSync(PROVIDERS_FILE)) {
    await writeJsonFile(PROVIDERS_FILE, [
      {
        id: 'ollama',
        name: 'Ollama',
        base_url: 'http://localhost:11434/v1',
        api_key: ''
      }
    ]);
  } else {
    // If providers list is empty, populate default Ollama
    const providers = await readJsonFile<any[]>(PROVIDERS_FILE, []);
    if (providers.length === 0) {
      await writeJsonFile(PROVIDERS_FILE, [
        {
          id: 'ollama',
          name: 'Ollama',
          base_url: 'http://localhost:11434/v1',
          api_key: ''
        }
      ]);
    }
  }

  if (!existsSync(MODELS_FILE)) {
    await writeJsonFile(MODELS_FILE, []);
  }

  console.log('JSON database initialized at:', TALOS_DIR);
}

// ==========================================
// CHATS DATABASE METHODS
// ==========================================

export async function getChats(): Promise<Array<{ id: string; title: string; created_at: number }>> {
  try {
    const entries = await fs.readdir(CHATS_DIR, { withFileTypes: true });
    const chatDirectories = entries.filter(entry => entry.isDirectory());
    const chatsData = await Promise.all(
      chatDirectories.map(async (dir) => {
        const metadataPath = path.join(CHATS_DIR, dir.name, 'metadata.json');
        if (!existsSync(metadataPath)) {
          return null;
        }
        const data = await readJsonFile<any>(metadataPath, null);
        if (data && data.id && data.title) {
          return {
            id: data.id,
            title: data.title,
            created_at: data.created_at || Date.now()
          };
        }
        return null;
      })
    );
    return (chatsData.filter(Boolean) as any[]).sort((a, b) => b.created_at - a.created_at);
  } catch (error) {
    console.error('Error getting chats:', error);
    return [];
  }
}

export async function createChat(id: string, title: string): Promise<void> {
  const chatFolder = path.join(CHATS_DIR, id);
  await fs.mkdir(chatFolder, { recursive: true });
  
  const metadataPath = path.join(chatFolder, 'metadata.json');
  const messagesPath = path.join(chatFolder, 'messages.json');
  
  const metadata = {
    id,
    title,
    created_at: Date.now()
  };
  
  await writeJsonFile(metadataPath, metadata);
  await writeJsonFile(messagesPath, []);
}

export async function deleteChat(id: string): Promise<void> {
  const chatFolder = path.join(CHATS_DIR, id);
  try {
    if (existsSync(chatFolder)) {
      await fs.rm(chatFolder, { recursive: true, force: true });
    }
  } catch (error) {
    console.error(`Error deleting chat ${id}:`, error);
    throw error;
  }
}

export async function renameChat(id: string, title: string): Promise<void> {
  const chatFolder = path.join(CHATS_DIR, id);
  const metadataPath = path.join(chatFolder, 'metadata.json');
  const metadata = await readJsonFile<any>(metadataPath, null);
  if (metadata) {
    metadata.title = title;
    await writeJsonFile(metadataPath, metadata);
  } else {
    throw new Error(`Chat ${id} not found`);
  }
}

// ==========================================
// MESSAGES DATABASE METHODS
// ==========================================

export async function getMessages(chatId: string): Promise<Array<{ id: string; role: string; content: string }>> {
  const messagesPath = path.join(CHATS_DIR, chatId, 'messages.json');
  return await readJsonFile<any[]>(messagesPath, []);
}

export async function addMessage(
  id: string,
  chatId: string,
  role: string,
  content: string,
  toolCalls?: any[],
  toolCallId?: string
): Promise<void> {
  const chatFolder = path.join(CHATS_DIR, chatId);
  const messagesPath = path.join(chatFolder, 'messages.json');
  
  if (!existsSync(chatFolder)) {
    throw new Error(`Chat ${chatId} not found to add message`);
  }
  
  const messages = await readJsonFile<any[]>(messagesPath, []);
  const index = messages.findIndex((m: any) => m.id === id);
  
  const messageObj: any = {
    id,
    role,
    content,
    created_at: Date.now()
  };
  if (toolCalls !== undefined) {
    messageObj.tool_calls = toolCalls;
  }
  if (toolCallId !== undefined) {
    messageObj.tool_call_id = toolCallId;
  }

  if (index !== -1) {
    messages[index] = {
      ...messages[index],
      ...messageObj
    };
  } else {
    messages.push(messageObj);
  }
  await writeJsonFile(messagesPath, messages);
}

export async function saveMessages(chatId: string, messages: any[]): Promise<void> {
  const chatFolder = path.join(CHATS_DIR, chatId);
  const messagesPath = path.join(chatFolder, 'messages.json');
  
  if (!existsSync(chatFolder)) {
    throw new Error(`Chat ${chatId} not found to save messages`);
  }
  
  await writeJsonFile(messagesPath, messages);
}

// ==========================================
// APPLICATION SETTINGS DATABASE METHODS
// ==========================================

export async function getSetting(key: string, defaultValue: string): Promise<string> {
  const settings = await readJsonFile<Record<string, string>>(SETTINGS_FILE, {});
  return settings[key] !== undefined ? settings[key] : defaultValue;
}

export async function setSetting(key: string, value: string): Promise<void> {
  const settings = await readJsonFile<Record<string, string>>(SETTINGS_FILE, {});
  settings[key] = value;
  await writeJsonFile(SETTINGS_FILE, settings);
}

// ==========================================
// PROVIDERS & MODELS DATABASE METHODS
// ==========================================

export async function getProviders(): Promise<Array<{ id: string; name: string; base_url: string; api_key: string }>> {
  return await readJsonFile<Array<{ id: string; name: string; base_url: string; api_key: string }>>(PROVIDERS_FILE, []);
}

export async function saveProvider(id: string, name: string, baseUrl: string, apiKey: string): Promise<void> {
  const providers = await getProviders();
  const index = providers.findIndex(p => p.id === id);
  const updatedProvider = { id, name, base_url: baseUrl, api_key: apiKey };
  if (index !== -1) {
    providers[index] = updatedProvider;
  } else {
    providers.push(updatedProvider);
  }
  await writeJsonFile(PROVIDERS_FILE, providers);
}

export async function deleteProvider(id: string): Promise<void> {
  const providers = await getProviders();
  const filteredProviders = providers.filter(p => p.id !== id);
  await writeJsonFile(PROVIDERS_FILE, filteredProviders);

  const models = await readJsonFile<Array<{ id: string; provider_id: string; name: string }>>(MODELS_FILE, []);
  const filteredModels = models.filter(m => m.provider_id !== id);
  await writeJsonFile(MODELS_FILE, filteredModels);
}

export async function getModels(providerId: string): Promise<Array<{ id: string; name: string }>> {
  const models = await readJsonFile<Array<{ id: string; provider_id: string; name: string }>>(MODELS_FILE, []);
  return models
    .filter(m => m.provider_id === providerId)
    .map(m => ({ id: m.id, name: m.name }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

export async function addModel(id: string, providerId: string, name: string): Promise<void> {
  const models = await readJsonFile<Array<{ id: string; provider_id: string; name: string }>>(MODELS_FILE, []);
  const index = models.findIndex(m => m.id === id);
  const newModel = { id, provider_id: providerId, name };
  if (index !== -1) {
    models[index] = newModel;
  } else {
    models.push(newModel);
  }
  await writeJsonFile(MODELS_FILE, models);
}

export async function deleteModel(id: string): Promise<void> {
  const models = await readJsonFile<Array<{ id: string; provider_id: string; name: string }>>(MODELS_FILE, []);
  const filteredModels = models.filter(m => m.id !== id);
  await writeJsonFile(MODELS_FILE, filteredModels);
}
