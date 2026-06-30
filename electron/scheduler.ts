import { BrowserWindow } from 'electron';
import { OpenAI } from 'openai';
import {
  getSchedules,
  updateScheduleStatus,
  addMessage,
  getProviders,
  ScheduledTask
} from './db';
import { getOpenAITools, executeTool, isCommandSafe } from './tools';
import { getSystemPrompt } from './prompts';

const POLL_INTERVAL_MS = 30_000; // 30 secondes
const MAX_AGENT_STEPS = 25;      // Sécurité anti-boucle infinie

let pollTimer: ReturnType<typeof setInterval> | null = null;
let mainWindow: BrowserWindow | null = null;

// ── Parsing cron (5 champs: minute heure jour mois jour_semaine) ──────────

function cronFieldMatches(pattern: string, value: number): boolean {
  if (pattern === '*') return true;

  const stepMatch = pattern.match(/^\*\/(\d+)$/);
  if (stepMatch) {
    const step = parseInt(stepMatch[1], 10);
    return step > 0 && value % step === 0;
  }

  if (pattern.includes(',')) {
    return pattern.split(',').some(p => cronFieldMatches(p.trim(), value));
  }

  const rangeMatch = pattern.match(/^(\d+)-(\d+)$/);
  if (rangeMatch) {
    const min = parseInt(rangeMatch[1], 10);
    const max = parseInt(rangeMatch[2], 10);
    return value >= min && value <= max;
  }

  return parseInt(pattern, 10) === value;
}

function matchesCron(expression: string, date: Date): boolean {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) return false;

  const [minutePattern, hourPattern, dayPattern, monthPattern, dowPattern] = parts;

  return (
    cronFieldMatches(minutePattern, date.getMinutes()) &&
    cronFieldMatches(hourPattern, date.getHours()) &&
    cronFieldMatches(dayPattern, date.getDate()) &&
    cronFieldMatches(monthPattern, date.getMonth() + 1) &&
    cronFieldMatches(dowPattern, date.getDay())
  );
}

// ── Calcul de la prochaine échéance ──────────────────────────────────────

export function computeNextRun(task: ScheduledTask): number | null {
  if (!task.enabled) return null;

  if (task.schedule_type === 'once') {
    const targetDate = new Date(task.schedule_value);
    if (isNaN(targetDate.getTime())) return null;
    const target = targetDate.getTime();
    if (target <= Date.now()) return null;
    return target;
  }

  if (task.schedule_type === 'cron') {
    const now = Date.now();
    const maxLookAhead = now + 365 * 24 * 60 * 60 * 1000;
    let checkDate = new Date(now + 60_000);
    checkDate.setSeconds(0, 0);

    while (checkDate.getTime() <= maxLookAhead) {
      if (matchesCron(task.schedule_value, checkDate)) {
        return checkDate.getTime();
      }
      checkDate = new Date(checkDate.getTime() + 60_000);
    }
    return null;
  }

  return null;
}

// ── Sélection des outils selon les options ───────────────────────────────

function getToolsForTask(task: ScheduledTask): any[] {
  const allTools = getOpenAITools();

  if (!task.internet_access && !task.workspace) {
    // Aucun outil
    return [];
  }

  if (task.internet_access && !task.workspace) {
    // Internet seulement : outils web + artifacts (toujours disponibles)
    const allowed = new Set(['FetchWebPage', 'BrowseWebPage', 'GoogleSearch',
      'WriteArtifact', 'ReadArtifact', 'ReplaceInArtifact', 'ListArtifacts']);
    return allTools.filter(t => allowed.has(t.function.name));
  }

  if (task.workspace) {
    // Workspace : tous les outils sauf run_parallel_agents
    return allTools.filter(t => t.function.name !== 'run_parallel_agents');
  }

  return [];
}

// ── Exécution d'une tâche (avec ou sans outils) ───────────────────────────

export async function runTaskNow(
  task: ScheduledTask,
  window: BrowserWindow | null
): Promise<string> {
  try {
    // 1. Récupérer le provider
    const providersList = await getProviders();
    const provider = providersList.find(p => p.id === task.provider_id);
    if (!provider) {
      throw new Error(`Provider introuvable : ${task.provider_id}`);
    }

    let baseUrl = provider.base_url;
    if (task.provider_id === 'ollama' && !baseUrl.endsWith('/v1') && !baseUrl.endsWith('/v1/')) {
      baseUrl = baseUrl.replace(/\/$/, '') + '/v1';
    }

    const client = new OpenAI({
      apiKey: provider.api_key || 'dummy-key',
      baseURL: baseUrl,
    });

    // 2. Ajouter le message user dans le chat
    const userMsgId = `sched-${task.id}-${Date.now()}-user`;
    await addMessage(userMsgId, task.chat_id, 'user', task.instructions);

    // 3. Déterminer les outils disponibles
    const tools = getToolsForTask(task);
    const hasTools = tools.length > 0;
    const hasWorkspace = !!task.workspace;

    // Changer le CWD si workspace défini
    let previousCwd: string | null = null;
    if (hasWorkspace) {
      previousCwd = process.cwd();
      process.chdir(task.workspace!);
      console.log(`[Scheduler] Changed CWD to workspace: ${task.workspace}`);
    }

    try {
      let finalContent: string;

      if (!hasTools) {
        // ── Mode simple (sans outils) ──
        const response = await client.chat.completions.create({
          model: task.model,
          messages: [
            { role: 'user', content: task.instructions }
          ],
        });
        finalContent = response.choices[0]?.message?.content || '(aucune réponse)';

      } else {
        // ── Mode agent avec outils ──
        const systemPrompt = await getSystemPrompt('agent', task.chat_id);

        const messages: OpenAI.Chat.Completions.ChatCompletionMessageParam[] = [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: task.instructions }
        ];

        let steps = 0;
        let finalResponse = '';

        while (steps < MAX_AGENT_STEPS) {
          steps++;

          const response = await client.chat.completions.create({
            model: task.model,
            messages: messages,
            tools: tools,
          });

          const message = response.choices[0].message;

          if (message.tool_calls && message.tool_calls.length > 0) {
            messages.push({
              role: 'assistant',
              content: message.content || null,
              tool_calls: message.tool_calls as any,
            });

            for (const tc of message.tool_calls) {
              let args: any = {};
              try {
                args = JSON.parse(tc.function.arguments);
              } catch (e) {}

              let result: string;

              if (tc.function.name === 'Bash') {
                const command = args.command || '';
                if (!isCommandSafe(command)) {
                  result = `error: Command execution blocked by security guardrails. The command "${command}" contains forbidden patterns.`;
                } else {
                  result = await executeTool(tc.function.name, args, task.chat_id);
                }
              } else {
                result = await executeTool(tc.function.name, args, task.chat_id);
              }

              messages.push({
                role: 'tool',
                tool_call_id: tc.id,
                content: result,
              } as any);
            }
          } else {
            finalResponse = message.content || '(aucune réponse)';
            break;
          }
        }

        if (steps >= MAX_AGENT_STEPS) {
          finalResponse = `[L'agent a atteint la limite de ${MAX_AGENT_STEPS} étapes]`;
        }

        finalContent = finalResponse;
      }

      // 4. Ajouter la réponse dans le chat
      const assistantMsgId = `sched-${task.id}-${Date.now()}-assistant`;
      await addMessage(assistantMsgId, task.chat_id, 'assistant', finalContent);

      // 5. Mettre à jour le statut de la tâche
      const nextRun = computeNextRun(task);
      const runLabel = `Run #${(task.total_runs || 0) + 1} - ${new Date().toLocaleString('fr-FR')}`;
      await updateScheduleStatus(task.id, {
        last_run: Date.now(),
        last_result: runLabel,
        next_run: nextRun,
        total_runs: (task.total_runs || 0) + 1,
      });

      // 6. Notifier le renderer
      if (window && !window.isDestroyed()) {
        window.webContents.send('scheduler:task-executed', {
          taskId: task.id,
          chatId: task.chat_id,
          last_run: Date.now(),
          last_result: runLabel,
          next_run: nextRun,
          total_runs: (task.total_runs || 0) + 1,
        });

        window.webContents.send('scheduler:chat-created', {
          chatId: task.chat_id,
        });
      }

      return finalContent;

    } finally {
      // Restaurer le CWD original si on l'avait changé
      if (previousCwd) {
        try {
          process.chdir(previousCwd);
          console.log(`[Scheduler] Restored CWD to: ${previousCwd}`);
        } catch (e) {
          console.error('[Scheduler] Failed to restore CWD:', e);
        }
      }
    }

  } catch (err: any) {
    console.error(`[Scheduler] Error running task ${task.id} (${task.name}):`, err);

    try {
      const errorMsgId = `sched-${task.id}-${Date.now()}-error`;
      await addMessage(errorMsgId, task.chat_id, 'assistant', `**Erreur lors de l'exécution :** ${err.message}`);
    } catch (e) {
      console.error('[Scheduler] Failed to save error message:', e);
    }

    if (window && !window.isDestroyed()) {
      window.webContents.send('scheduler:task-executed', {
        taskId: task.id,
        chatId: task.chat_id,
        last_run: Date.now(),
        last_result: `Erreur : ${err.message}`,
        error: true,
      });
    }

    return `error: ${err.message}`;
  }
}

// ── Boucle de vérification ───────────────────────────────────────────────

async function checkAndRunTasks(): Promise<void> {
  try {
    const schedules = await getSchedules();
    const now = Date.now();

    for (const task of schedules) {
      if (!task.enabled) continue;

      let shouldRun = false;

      if (task.schedule_type === 'once') {
        const targetDate = new Date(task.schedule_value);
        if (!isNaN(targetDate.getTime())) {
          if (targetDate.getTime() <= now && !task.last_run) {
            shouldRun = true;
          }
        }
      } else if (task.schedule_type === 'cron') {
        const currentMinute = new Date();
        currentMinute.setSeconds(0, 0);
        const lastRunDate = task.last_run ? new Date(task.last_run) : null;
        let lastRunMinute: Date | null = null;
        if (lastRunDate) {
          lastRunMinute = new Date(lastRunDate);
          lastRunMinute.setSeconds(0, 0);
        }

        if (matchesCron(task.schedule_value, currentMinute)) {
          if (!lastRunMinute || lastRunMinute.getTime() < currentMinute.getTime()) {
            if (now - currentMinute.getTime() < 2 * 60 * 1000) {
              shouldRun = true;
            }
          }
        }
      }

      if (shouldRun) {
        console.log(`[Scheduler] Running task: ${task.name} (${task.id})`);
        await runTaskNow(task, mainWindow);
      }
    }
  } catch (err) {
    console.error('[Scheduler] Error in checkAndRunTasks:', err);
  }
}

// ── Gestion du poller intelligent ─────────────────────────────────────────

async function refreshPollingState(): Promise<void> {
  try {
    const schedules = await getSchedules();
    const hasActive = schedules.some(s => s.enabled);

    if (hasActive && pollTimer === null) {
      console.log('[Scheduler] Starting poller (active tasks detected)');
      pollTimer = setInterval(checkAndRunTasks, POLL_INTERVAL_MS);
      checkAndRunTasks();
    } else if (!hasActive && pollTimer !== null) {
      console.log('[Scheduler] Stopping poller (no active tasks)');
      clearInterval(pollTimer);
      pollTimer = null;
    }
  } catch (err) {
    console.error('[Scheduler] Error refreshing polling state:', err);
  }
}

// ── API publique ──────────────────────────────────────────────────────────

export function initScheduler(window: BrowserWindow | null): void {
  mainWindow = window;
  console.log('[Scheduler] Initialized');
  refreshPollingState();
}

export function stopScheduler(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  console.log('[Scheduler] Stopped');
}

export function triggerSchedulerCheck(): void {
  refreshPollingState();
}