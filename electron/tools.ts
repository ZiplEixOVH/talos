import fs from 'fs';
import path from 'path';
import readline from 'readline';
import { exec } from 'child_process';
import ignore from 'ignore';

// Helper: Clean HTML tags and decode basic HTML entities
function cleanHTML(input: string): string {
  let res = input.replace(/<[^>]*>/g, '');
  res = res
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&#39;/g, "'");
  return res.trim();
}

// Helper: Search for .gitignore upwards from a start path
function findGitIgnore(startPath: string): string | null {
  try {
    const absStart = path.resolve(startPath);
    let curr = absStart;
    while (true) {
      const ignorePath = path.join(curr, '.gitignore');
      if (fs.existsSync(ignorePath)) {
        return ignorePath;
      }
      const parent = path.dirname(curr);
      if (parent === curr) {
        break;
      }
      curr = parent;
    }
  } catch (e) {
    // ignore
  }
  return null;
}

// Handler: Read a file
export function handleReadTool(args: any): string {
  const filePath = args.file_path;
  if (!filePath || typeof filePath !== 'string') {
    return 'error: file_path parameter is missing or not a string';
  }
  try {
    return fs.readFileSync(filePath, 'utf8');
  } catch (err: any) {
    return `error reading file: ${err.message}`;
  }
}

// Handler: Write to a file
export function handleWriteTool(args: any): string {
  const filePath = args.file_path;
  if (!filePath || typeof filePath !== 'string') {
    return 'error: file_path parameter is missing or not a string';
  }
  const content = args.content;
  if (typeof content !== 'string') {
    return 'error: content parameter is missing or not a string';
  }
  try {
    fs.writeFileSync(filePath, content, 'utf8');
    return content;
  } catch (err: any) {
    return `error writing file: ${err.message}`;
  }
}

// Handler: Create a directory recursivly
export function handleMkdirTool(args: any): string {
  const directoryPath = args.directory_path;
  if (!directoryPath || typeof directoryPath !== 'string') {
    return 'error: directory_path parameter is missing or not a string';
  }
  try {
    fs.mkdirSync(directoryPath, { recursive: true });
    return directoryPath;
  } catch (err: any) {
    return `error creating directory: ${err.message}`;
  }
}

// Handler: Execute a shell command
export function handleBashTool(args: any): Promise<string> {
  const command = args.command;
  if (!command || typeof command !== 'string') {
    return Promise.resolve('error: command parameter is missing or not a string');
  }
  return new Promise((resolve) => {
    exec(command, (error, stdout, stderr) => {
      if (error) {
        resolve(stderr || error.message);
      } else {
        resolve(stdout || stderr);
      }
    });
  });
}

// Handler: List directory files, filtering out .gitignore matches
export function handleListTool(args: any): string {
  const directory = args.directory;
  if (!directory || typeof directory !== 'string') {
    return 'error: directory parameter is missing or not a string';
  }

  try {
    const files = fs.readdirSync(directory, { withFileTypes: true });

    let gitignoreObj: any = null;
    let gitignoreDir = '';
    const gitignorePath = findGitIgnore(directory);
    if (gitignorePath) {
      try {
        const gitignoreContent = fs.readFileSync(gitignorePath, 'utf8');
        gitignoreObj = ignore().add(gitignoreContent);
        gitignoreDir = path.dirname(gitignorePath);
      } catch (e) {
        // ignore compile errors
      }
    }

    interface ListEntry {
      name: string;
      path: string;
      type: string;
      size_bytes: number;
      mode: string;
      modified_at: string;
      is_hidden: boolean;
      is_symlink: boolean;
      extension?: string;
    }

    const entries: ListEntry[] = [];
    for (const file of files) {
      const filePath = path.join(directory, file.name);
      if (gitignoreObj) {
        const relPath = path.relative(gitignoreDir, filePath);
        if (gitignoreObj.ignores(relPath)) {
          continue;
        }
      }
      let entryType = 'file';
      if (file.isDirectory()) {
        entryType = 'folder';
      } else if (file.isSymbolicLink()) {
        entryType = 'symlink';
      }
      const entry: ListEntry = {
        name: file.name,
        path: filePath,
        type: entryType,
        is_hidden: file.name.startsWith('.'),
        is_symlink: file.isSymbolicLink(),
        extension: path.extname(file.name),
        size_bytes: 0,
        mode: '',
        modified_at: ''
      };

      try {
        const info = fs.statSync(filePath);
        entry.size_bytes = info.size;
        entry.mode = info.mode.toString(8);
        entry.modified_at = info.mtime.toISOString();
      } catch (e) {}

      entries.push(entry);
    }
    return JSON.stringify(entries);
  } catch (err: any) {
    return `error listing directory: ${err.message}`;
  }
}

// Handler: Recursively display a visual tree diagram (respecting .gitignore)
export function handleTreeTool(args: any): string {
  const directory = args.directory;
  if (!directory || typeof directory !== 'string') {
    return 'error: directory parameter is missing or not a string';
  }

  let gitignoreObj: any = null;
  let gitignoreDir = '';
  const gitignorePath = findGitIgnore(directory);
  if (gitignorePath) {
    try {
      const gitignoreContent = fs.readFileSync(gitignorePath, 'utf8');
      gitignoreObj = ignore().add(gitignoreContent);
      gitignoreDir = path.dirname(gitignorePath);
    } catch (e) {
      // ignore compiling errors
    }
  }

  let maxDepth = 5;
  if ('max_depth' in args) {
    const d = Number(args.max_depth);
    if (!isNaN(d)) {
      maxDepth = d;
    }
  }

  const lines: string[] = [];
  walkTree(lines, directory, 0, maxDepth, gitignoreObj, gitignoreDir, '');
  return lines.join('\n');
}

function walkTree(
  lines: string[],
  dir: string,
  depth: number,
  maxDepth: number,
  gitignoreObj: any,
  gitignoreDir: string,
  prefix: string
) {
  if (depth > maxDepth) {
    return;
  }

  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (err: any) {
    lines.push(`${prefix}[error: ${err.message}]`);
    return;
  }

  const dirs: fs.Dirent[] = [];
  const files: fs.Dirent[] = [];

  for (const e of entries) {
    const name = e.name;
    if (name === '.' || name === '..' || name.startsWith('.')) {
      continue;
    }
    if (gitignoreObj) {
      const fullPath = path.join(dir, name);
      const relPath = path.relative(gitignoreDir, fullPath);
      if (gitignoreObj.ignores(relPath)) {
        continue;
      }
    }
    if (e.isDirectory()) {
      dirs.push(e);
    } else {
      files.push(e);
    }
  }

  const all = [...dirs, ...files];

  for (let i = 0; i < all.length; i++) {
    const entry = all[i];
    const isLast = i === all.length - 1;
    const connector = isLast ? '└── ' : '├── ';
    lines.push(`${prefix}${connector}${entry.name}`);

    if (entry.isDirectory()) {
      const nextPrefix = prefix + (isLast ? '    ' : '│   ');
      walkTree(
        lines,
        path.join(dir, entry.name),
        depth + 1,
        maxDepth,
        gitignoreObj,
        gitignoreDir,
        nextPrefix
      );
    }
  }
}

// Handler: Fetch URL content
export async function handleWebSearchTool(args: any): Promise<string> {
  const url = args.url;
  if (!url || typeof url !== 'string') {
    return 'error: url parameter is missing or not a string';
  }
  try {
    const resp = await fetch(url);
    if (!resp.ok) {
      return `error: fetch request failed with status code ${resp.status}`;
    }
    const text = await resp.text();
    return text;
  } catch (err: any) {
    return `error fetching page: ${err.message}`;
  }
}

// Handler: Query DuckDuckGo html search endpoint and parse results
export async function handleGoogleSearchTool(args: any): Promise<string> {
  const query = args.query;
  if (!query || typeof query !== 'string') {
    return 'error: query parameter is missing or not a string';
  }
  try {
    const searchURL = 'https://html.duckduckgo.com/html/?q=' + encodeURIComponent(query);
    const resp = await fetch(searchURL, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
      }
    });
    if (!resp.ok) {
      return `error: search request failed with status code ${resp.status}`;
    }
    const html = await resp.text();
    const alternativeBlocks = html.split('<div class="result results_links');
    if (alternativeBlocks.length <= 1) {
      return '[]';
    }

    interface SearchResult {
      title: string;
      url: string;
      snippet: string;
    }
    const results: SearchResult[] = [];
    const titleRe = /class="result__a"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)<\/a>/;
    const snippetRe = /class="result__snippet"[^>]*>([\s\S]*?)<\/a>/;

    for (const block of alternativeBlocks.slice(1)) {
      const titleMatch = titleRe.exec(block);
      if (!titleMatch) continue;
      const resURL = titleMatch[1];
      const title = cleanHTML(titleMatch[2]);

      let snippet = '';
      const snippetMatch = snippetRe.exec(block);
      if (snippetMatch) {
        snippet = cleanHTML(snippetMatch[1]);
      }

      results.push({
        title,
        url: resURL,
        snippet
      });
    }
    return JSON.stringify(results);
  } catch (err: any) {
    return `error performing search request: ${err.message}`;
  }
}

// Handler: Recursively search text in a directory or file (skips binary/massive files)
export function handleFileSearchTool(args: any): string {
  const pattern = args.pattern;
  if (!pattern || typeof pattern !== 'string') {
    return 'error: pattern parameter is missing or not a string';
  }
  const dirPath = args.directory;
  if (!dirPath || typeof dirPath !== 'string') {
    return 'error: directory parameter is missing or not a string';
  }

  interface Match {
    file_path: string;
    line: number;
    content: string;
  }

  const matches: Match[] = [];
  const maxMatches = 100;
  const patternLower = pattern.toLowerCase();

  try {
    const stats = fs.statSync(dirPath);
    if (!stats.isDirectory()) {
      searchFile(dirPath, patternLower, matches, maxMatches);
    } else {
      walkSearch(dirPath, patternLower, matches, maxMatches);
    }
    return JSON.stringify(matches);
  } catch (err: any) {
    return `error checking path: ${err.message}`;
  }
}

function isBinaryFile(filePath: string): boolean {
  try {
    const fd = fs.openSync(filePath, 'r');
    const buf = Buffer.alloc(512);
    const bytesRead = fs.readSync(fd, buf, 0, 512, 0);
    fs.closeSync(fd);
    for (let i = 0; i < bytesRead; i++) {
      if (buf[i] === 0) {
        return true;
      }
    }
    return false;
  } catch (e) {
    return true;
  }
}

function searchFile(filePath: string, patternLower: string, matches: any[], maxMatches: number) {
  try {
    const stats = fs.statSync(filePath);
    if (stats.size > 1024 * 1024) return; // skip > 1MB
    if (isBinaryFile(filePath)) return; // skip binary files

    const content = fs.readFileSync(filePath, 'utf8');
    const lines = content.split('\n');
    for (let i = 0; i < lines.length; i++) {
      if (lines[i].toLowerCase().includes(patternLower)) {
        matches.push({
          file_path: filePath,
          line: i + 1,
          content: lines[i].trim()
        });
        if (matches.length >= maxMatches) {
          break;
        }
      }
    }
  } catch (e) {
    // ignore
  }
}

function walkSearch(dir: string, patternLower: string, matches: any[], maxMatches: number) {
  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (e) {
    return;
  }

  for (const e of entries) {
    if (matches.length >= maxMatches) break;
    const name = e.name;
    if (name.startsWith('.') || name === 'node_modules' || name === 'vendor') {
      continue;
    }
    const fullPath = path.join(dir, name);
    if (e.isDirectory()) {
      walkSearch(fullPath, patternLower, matches, maxMatches);
    } else {
      searchFile(fullPath, patternLower, matches, maxMatches);
    }
  }
}

// Handler: Read a range of lines from a file
export async function handleReadRangeTool(args: any): Promise<string> {
  const filePath = args.file_path;
  if (!filePath || typeof filePath !== 'string') {
    return 'error: file_path parameter is missing or not a string';
  }
  let startLine = Number(args.start_line) || 1;
  if (startLine <= 0) startLine = 1;
  const endLine = Number(args.end_line) || 0;

  try {
    const fileStream = fs.createReadStream(filePath);
    const rl = readline.createInterface({
      input: fileStream,
      crlfDelay: Infinity
    });

    const lines: string[] = [];
    let currentLine = 0;
    for await (const line of rl) {
      currentLine++;
      if (currentLine >= startLine) {
        if (endLine > 0 && currentLine > endLine) {
          break;
        }
        lines.push(line);
      }
    }
    return lines.join('\n');
  } catch (err: any) {
    return `error reading file: ${err.message}`;
  }
}

// Handler: Replace a specific block of text in a file
export function handleReplaceInFileTool(args: any): string {
  const filePath = args.file_path;
  if (!filePath || typeof filePath !== 'string') {
    return 'error: file_path parameter is missing or not a string';
  }
  const oldContent = args.old_content;
  if (typeof oldContent !== 'string') {
    return 'error: old_content parameter is missing or not a string';
  }
  const newContent = args.new_content;
  if (typeof newContent !== 'string') {
    return 'error: new_content parameter is missing or not a string';
  }

  try {
    if (!fs.existsSync(filePath)) {
      return `error reading file: File does not exist`;
    }
    const content = fs.readFileSync(filePath, 'utf8');
    const count = content.split(oldContent).length - 1;
    if (count === 0) {
      return 'error: old_content was not found in the file';
    }
    if (count > 1) {
      return `error: old_content matches multiple locations (${count} occurrences); please provide more surrounding lines to uniquely identify the block to replace`;
    }

    const updatedContent = content.replace(oldContent, newContent);
    fs.writeFileSync(filePath, updatedContent, 'utf8');
    return 'success: content replaced successfully';
  } catch (err: any) {
    return `error: ${err.message}`;
  }
}

const primaryParamNames: Record<string, string> = {
  Read: 'file_path',
  Write: 'file_path',
  Mkdir: 'directory_path',
  Bash: 'command',
  List: 'directory',
  Tree: 'directory',
  FetchWebPage: 'url',
  GoogleSearch: 'query',
  FileSearch: 'pattern',
  ReadRange: 'file_path',
  ReplaceInFile: 'file_path'
};

export function getToolParamValue(name: string, args: any): string {
  if (!args) return '';
  const key = primaryParamNames[name];
  if (key && args[key] !== undefined) {
    const val = args[key];
    return typeof val === 'string' ? val : String(val);
  }
  // Fallback: return first parameter value found
  const keys = Object.keys(args);
  if (keys.length > 0) {
    const val = args[keys[0]];
    return typeof val === 'string' ? val : String(val);
  }
  return '';
}

// Dispatch tool execution by name
export async function executeTool(name: string, args: any): Promise<string> {
  switch (name) {
    case 'Read':
      return handleReadTool(args);
    case 'Write':
      return handleWriteTool(args);
    case 'Mkdir':
      return handleMkdirTool(args);
    case 'Bash':
      return await handleBashTool(args);
    case 'List':
      return handleListTool(args);
    case 'Tree':
      return handleTreeTool(args);
    case 'FetchWebPage':
      return await handleWebSearchTool(args);
    case 'GoogleSearch':
      return await handleGoogleSearchTool(args);
    case 'FileSearch':
      return handleFileSearchTool(args);
    case 'ReadRange':
      return await handleReadRangeTool(args);
    case 'ReplaceInFile':
      return handleReplaceInFileTool(args);
    default:
      return `error: unknown tool call ${name}`;
  }
}

// Return OpenAI JSON schema definitions for the tools
export function getOpenAITools() {
  return [
    {
      type: 'function',
      function: {
        name: 'Read',
        description: 'Read and return the content of a file',
        parameters: {
          type: 'object',
          properties: {
            file_path: {
              type: 'string',
              description: 'The path to the file to read'
            }
          },
          required: ['file_path']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'Write',
        description: 'Write content to a file, create the file if it does not exist',
        parameters: {
          type: 'object',
          properties: {
            file_path: {
              type: 'string',
              description: 'The path to the file to write'
            },
            content: {
              type: 'string',
              description: 'The content to write to the file'
            }
          },
          required: ['file_path', 'content']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'Mkdir',
        description: 'Create a directory (including parent directories if needed)',
        parameters: {
          type: 'object',
          properties: {
            directory_path: {
              type: 'string',
              description: 'The path to the directory to create'
            }
          },
          required: ['directory_path']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'Bash',
        description: 'Execute a shell command',
        parameters: {
          type: 'object',
          properties: {
            command: {
              type: 'string',
              description: 'The shell command to execute'
            }
          },
          required: ['command']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'List',
        description: 'List files in a directory',
        parameters: {
          type: 'object',
          properties: {
            directory: {
              type: 'string',
              description: 'The directory path to list files from'
            }
          },
          required: ['directory']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'Tree',
        description: 'Display a visual tree representation of a directory structure (respects .gitignore)',
        parameters: {
          type: 'object',
          properties: {
            directory: {
              type: 'string',
              description: 'The directory path to display the tree for'
            },
            max_depth: {
              type: 'integer',
              description: 'Maximum depth to traverse (default: 5)'
            }
          },
          required: ['directory']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'FetchWebPage',
        description: 'Fetch the content of a webpage',
        parameters: {
          type: 'object',
          properties: {
            url: {
              type: 'string',
              description: 'The URL of the webpage to fetch'
            }
          },
          required: ['url']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'GoogleSearch',
        description: 'Search Google for a given query and return a list of search results (titles, URLs, snippets)',
        parameters: {
          type: 'object',
          properties: {
            query: {
              type: 'string',
              description: 'The search query'
            }
          },
          required: ['query']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'FileSearch',
        description: 'Search for a pattern or keyword recursively within a directory or file',
        parameters: {
          type: 'object',
          properties: {
            pattern: {
              type: 'string',
              description: 'The pattern or keyword to search for'
            },
            directory: {
              type: 'string',
              description: 'The directory or file path to search inside'
            }
          },
          required: ['pattern', 'directory']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'ReadRange',
        description: 'Read a specific line range from a file, avoiding loading the entire file',
        parameters: {
          type: 'object',
          properties: {
            file_path: {
              type: 'string',
              description: 'The path to the file to read'
            },
            start_line: {
              type: 'integer',
              description: 'The first line to read (1-indexed)'
            },
            end_line: {
              type: 'integer',
              description: 'The last line to read (inclusive)'
            }
          },
          required: ['file_path', 'start_line', 'end_line']
        }
      }
    },
    {
      type: 'function',
      function: {
        name: 'ReplaceInFile',
        description: 'Replace a specific block of text in a file with another block (uniquely identified)',
        parameters: {
          type: 'object',
          properties: {
            file_path: {
              type: 'string',
              description: 'The path to the file to modify'
            },
            old_content: {
              type: 'string',
              description: 'The exact content in the file to be replaced'
            },
            new_content: {
              type: 'string',
              description: 'The new content to replace it with'
            }
          },
          required: ['file_path', 'old_content', 'new_content']
        }
      }
    }
  ];
}
