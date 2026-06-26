import fs from 'fs/promises';
import { existsSync } from 'fs';
import path from 'path';
import { getOpenAIToolsForMode } from './tools';
import { getDbPath } from './db';

const PROMPTS_DIR = path.join(getDbPath(), 'prompts');

// Simple RegExp-based template rendering engine
export function renderTemplate(template: string, data: Record<string, any>): string {
  let result = template;

  // 1. Loops: {{#each list}}...{{/each}}
  const eachRegex = /\{\{#each\s+(\w+)\}\}([\s\S]*?)\{\{\/each\}\}/g;
  result = result.replace(eachRegex, (_, listKey, innerContent) => {
    const list = data[listKey];
    if (Array.isArray(list)) {
      return list.map(item => {
        let itemContent = innerContent;
        if (typeof item === 'object' && item !== null) {
          const itemKeys = Object.keys(item);
          for (const key of itemKeys) {
            const val = item[key];
            const escapedVal = typeof val === 'object' ? JSON.stringify(val) : String(val);
            itemContent = itemContent.replace(new RegExp(`\\{\\{\\s*${key}\\s*\\}\\}`, 'g'), escapedVal);
          }
        } else {
          itemContent = itemContent.replace(/\{\{\s*this\s*\}\}/g, String(item));
        }
        return itemContent;
      }).join('');
    }
    return '';
  });

  // 2. Conditionals: {{#if variable}}...{{/if}}
  const ifRegex = /\{\{#if\s+(\w+)\}\}([\s\S]*?)\{\{\/if\}\}/g;
  result = result.replace(ifRegex, (_, conditionKey, innerContent) => {
    const val = data[conditionKey];
    if (val && (!Array.isArray(val) || val.length > 0)) {
      return innerContent;
    }
    return '';
  });

  // 3. Simple variables: {{variable}}
  const varKeys = Object.keys(data);
  for (const key of varKeys) {
    const val = data[key];
    if (typeof val !== 'object' || val === null) {
      result = result.replace(new RegExp(`\\{\\{\\s*${key}\\s*\\}\\}`, 'g'), String(val));
    }
  }

  return result;
}

// Assemble and compile the full system prompt based on mode
export async function getSystemPrompt(mode: string, chatId?: string): Promise<string> {
  try {
    const systemPromptPath = path.join(PROMPTS_DIR, 'system.md');
    const modePromptPath = path.join(PROMPTS_DIR, `${mode}.md`);

    let systemContent = '';
    if (existsSync(systemPromptPath)) {
      systemContent = await fs.readFile(systemPromptPath, 'utf-8');
    } else {
      systemContent = `You are Talos, an advanced software engineering agent.\nCurrent Working Directory (CWD): {{currentCwd}}\nArtifacts Directory: {{chatFolder}}`;
    }

    let modeContent = '';
    if (existsSync(modePromptPath)) {
      modeContent = await fs.readFile(modePromptPath, 'utf-8');
    }

    // Retrieve active tools for this specific mode
    const tools = getOpenAIToolsForMode(mode);
    const toolsData = tools.map(t => ({
      name: t.function.name,
      description: t.function.description
    }));

    const chatsDir = path.join(getDbPath(), 'chats');
    const chatFolder = chatId ? path.join(chatsDir, chatId) : '';

    const data = {
      currentCwd: process.cwd(),
      chatFolder: chatFolder,
      tools: toolsData,
      hasTools: toolsData.length > 0
    };

    const renderedSystem = renderTemplate(systemContent, data);
    const renderedMode = renderTemplate(modeContent, data);

    return `${renderedSystem}\n\n${renderedMode}`.trim();
  } catch (error) {
    console.error('Error constructing system prompt:', error);
    return `You are Talos, a software engineering agent. CWD: ${process.cwd()}`;
  }
}
