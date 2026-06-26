<script lang="ts">
  import { onMount } from 'svelte';
  import { ShieldCheck, CheckCircle2, AlertTriangle, RefreshCw } from 'lucide-svelte';

  let dbStatus = $state<'loading' | 'connected' | 'fallback'>('loading');
  let dbPath = $state<string>('');

  onMount(async () => {
    // Vérifier si nous tournons dans Electron et si l'API est disponible
    if (window.talosAPI) {
      try {
        await window.talosAPI.getChats();
        dbStatus = 'connected';
        if (window.talosAPI.getDbPath) {
          dbPath = await window.talosAPI.getDbPath();
        }
      } catch (e) {
        console.error('JSON DB connection error:', e);
        dbStatus = 'fallback';
      }
    } else {
      dbStatus = 'fallback';
    }
  });

  async function testDatabaseConnection() {
    dbStatus = 'loading';
    setTimeout(async () => {
      if (window.talosAPI) {
        try {
          await window.talosAPI.getChats();
          dbStatus = 'connected';
          if (window.talosAPI.getDbPath) {
            dbPath = await window.talosAPI.getDbPath();
          }
        } catch (e) {
          dbStatus = 'fallback';
        }
      } else {
        dbStatus = 'fallback';
      }
    }, 800);
  }
</script>

<div class="space-y-6 animate-fade-in">
  <div>
    <h2 class="text-xl font-bold text-slate-100">Statut du Stockage</h2>
    <p class="text-slate-450 text-xs mt-0.5">Suivi de la persistance locale et des fichiers JSON.</p>
  </div>

  <!-- Connection Status Card -->
  <div class="p-5 rounded-xl border flex flex-col sm:flex-row sm:items-center justify-between gap-4 {
    dbStatus === 'connected' ? 'bg-emerald-500/5 border-emerald-500/20' :
    dbStatus === 'fallback' ? 'bg-amber-500/5 border-amber-500/20' :
    'bg-slate-900/50 border-slate-800'
  }">
    <div class="flex items-start gap-3.5">
      {#if dbStatus === 'connected'}
        <div class="p-2.5 rounded-lg bg-emerald-500/10 text-emerald-400">
          <ShieldCheck size={22} />
        </div>
        <div class="space-y-1">
          <h3 class="font-bold text-emerald-400 text-sm">Stockage JSON Actif</h3>
          <p class="text-slate-355 text-xs leading-relaxed">La persistance des conversations, modèles et paramètres est active sous forme de fichiers JSON.</p>
          <p class="text-slate-500 text-[10px] font-mono break-all bg-slate-950/40 p-1.5 rounded border border-slate-900/60 w-fit">Dossier : {dbPath || '~/.talos'}</p>
        </div>
      {:else if dbStatus === 'fallback'}
        <div class="p-2.5 rounded-lg bg-amber-500/10 text-amber-400">
          <AlertTriangle size={22} />
        </div>
        <div class="space-y-1">
          <h3 class="font-bold text-amber-400 text-sm">Mode Fallback (LocalStorage)</h3>
          <p class="text-slate-355 text-xs leading-relaxed font-normal">Stockage local indisponible (environnement navigateur hors Electron). Les discussions sont sauvegardées localement dans votre navigateur.</p>
        </div>
      {:else}
        <div class="p-2.5 rounded-lg bg-slate-800 text-slate-400 animate-pulse">
          <RefreshCw size={22} class="animate-spin" />
        </div>
        <div>
          <h3 class="font-bold text-slate-400 text-sm">Vérification de la connexion...</h3>
        </div>
      {/if}
    </div>

    <button
      onclick={testDatabaseConnection}
      disabled={dbStatus === 'loading'}
      class="flex items-center justify-center gap-1.5 px-4 py-2 bg-slate-950 hover:bg-slate-900 border border-slate-800 rounded-lg text-xs font-semibold text-slate-350 hover:text-white transition-colors cursor-pointer disabled:opacity-50 shrink-0 self-end sm:self-center"
    >
      <RefreshCw size={12} class={dbStatus === 'loading' ? 'animate-spin' : ''} />
      Re-tester
    </button>
  </div>

  <div class="bg-slate-950/40 p-4 border border-slate-900/60 rounded-lg space-y-2">
    <h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-wider">Statistiques de stockage</h4>
    <ul class="text-xs text-slate-400 space-y-1">
      <li class="flex justify-between border-b border-slate-900/40 py-1">
        <span>Moteur actif</span>
        <span class="text-slate-200 font-semibold">{dbStatus === 'connected' ? 'Dossier .talos (JSON)' : 'LocalStorage (Web API)'}</span>
      </li>
      <li class="flex justify-between py-1">
        <span>Synchronisation</span>
        <span class="text-slate-200 flex items-center gap-1 font-semibold">
          <CheckCircle2 size={12} class="text-emerald-500" />
          Automatique
        </span>
      </li>
    </ul>
  </div>
</div>
