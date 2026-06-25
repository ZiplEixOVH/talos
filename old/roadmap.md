# ◆ Talos — Roadmap

> Vision, fonctionnalités et évolutions futures de l'assistant de code IA dans le terminal.

---

## Légende

| Symbole | Signification |
|---|---|
| 🟢 Prêt / Implémenté | Fonctionne déjà |
| 🟡 En cours | En développement |
| 🔵 À faire | Prioritaire |
| ⚪ Suggestion | Idée à étudier |

---

## 1. 📂 Gestion des conversations

| Priorité | Fonctionnalité | Description |
|---|---|---|
| 🔵 | **Charger une conversation passée** | `/conversations` pour lister, charger, renommer et supprimer les conversations sauvegardées dans `.talos/conversations/`. Les données existent déjà, il manque l'UI et les commandes. |
| 🔵 | **Reprendre la dernière conversation** | Au lancement, détecter et restaurer automatiquement la dernière session active plutôt que de repartir à zéro. |
| ⚪ | **Export Markdown/HTML** | Exporter une conversation en fichier Markdown propre (ou HTML) pour la partager ou la documenter. |
| ⚪ | **Recherche dans l'historique** | `/search <mot-clé>` pour retrouver une conversation par son contenu ou son ID. |

---

## 2. 🧠 Améliorations du contexte IA

| Priorité | Fonctionnalité | Description |
|---|---|---|
| 🔵 | **Attacher des fichiers au contexte** | `/attach <fichier>` pour injecter le contenu d'un fichier directement dans le system prompt ou les messages. Utile pour donner du contexte sans outil. |
| 🔵 | **Contexte projet automatique** | Au démarrage, détecter `go.mod`, `package.json`, `Cargo.toml`, `README.md`, l'arborescence… et les injecter automatiquement dans le system prompt pour que l'IA comprenne le projet. |
| 🔵 | **Instructions personnalisées** | `/instructions "Sois concis, réponds en français"` — modifier le system prompt à la volée sans recréer la conversation. |
| ⚪ | **Mémoire persistante entre sessions** | Stocker des informations sur le projet dans `.talos/memory.json` (ex: "l'utilisateur préfère les réponses en anglais", "technos du projet") pour les réutiliser au prochain lancement. |
| ⚪ | **Sliding window context** | Gérer intelligemment la limite de tokens en résumant les messages les plus anciens plutôt que de les tronquer. |

---

## 3. 🔧 Outils avancés

| Priorité | Fonctionnalité | Description |
|---|---|---|
| 🔵 | **Diff avant/après** | Avant qu'un `ReplaceInFile` ou `Write` ne soit appliqué, afficher un diff coloré et demander confirmation à l'utilisateur. Plus sûr et pédagogique. |
| 🔵 | **Undo / rollback** | `/undo` pour annuler la dernière modification de fichier (via un système de backup automatique dans `.talos/backups/`). |
| ⚪ | **Exécution parallèle d'outils** | Permettre à l'IA d'appeler plusieurs outils indépendants en parallèle, au lieu de les exécuter séquentiellement un par un. Gain de temps sur les recherches. |
| ⚪ | **Outil `Tree`** | `Tree(directory)` — retourne une arborescence formatée (en ASCII ou JSON) pour visualiser la structure d'un dossier. |
| ⚪ | **Sandbox sécurisé** | Exécuter les commandes Bash dans un environnement isolé (Docker, conteneur temporaire) pour éviter les risques sur le système hôte. |
| ⚪ | **Clipboard** | `/copy <bloc>` ou `/copy last` pour copier la dernière réponse de l'IA dans le presse-papier système. |
| ⚪ | **Vision / Images** | Permettre d'envoyer des captures d'écran ou schémas aux modèles vision-capables (via base64) pour du debugging visuel. |

---

## 4. 🎛️ Paramètres et personnalisation

| Priorité | Fonctionnalité | Description |
|---|---|---|
| 🔵 | **Paramètres du LLM** | Commandes pour ajuster le comportement du modèle : `/temperature 0.7`, `/max-tokens 4096`, `/top-p 0.9`. À stocker dans les settings. |
| ⚪ | **Thèmes visuels** | Plusieurs palettes de couleurs commutables via `/theme` (dark, light, dracula, nord, catppuccin, etc.). |
| ⚪ | **Key bindings personnalisables** | Permettre à l'utilisateur de redéfinir ses raccourcis clavier dans `.talos/settings.json`. |
| ⚪ | **Multi-langue (i18n)** | Support de l'internationalisation pour l'interface (messages système, help, label). |

---

## 5. ⚡ Productivité & Git

| Priorité | Fonctionnalité | Description |
|---|---|---|
| 🔵 | **Git integration** | `/commit "message"` — l'IA génère un message de commit basé sur le diff et l'exécute. `/diff` pour voir les changements non commités. |
| 🔵 | **Review de code** | `/review` — envoyer le diff actuel (ou un fichier) à l'IA pour une revue de code automatique. |
| ⚪ | **Génération de tests** | `/test <fichier>` — demander à l'IA de générer des tests unitaires pour un fichier donné. |
| ⚪ | **Fix automatique** | `/fix` — l'IA analyse les erreurs de compilation/tests et propose/applique des corrections. |
| ⚪ | **Lint intelligent** | `/lint` — exécuter le linter du projet et envoyer les avertissements à l'IA pour les corriger. |

---

## 6. 📊 Monitoring & Statistiques

| Priorité | Fonctionnalité | Description |
|---|---|---|
| ⚪ | **Compteur de tokens** | Afficher le nombre de tokens utilisés par message ou par conversation. |
| ⚪ | **Suivi des coûts** | Pour les providers payants (OpenAI, OpenRouter), estimer le coût de la session en temps réel. |
| ⚪ | **Temps de réponse** | Afficher la latence de chaque appel API. |
| ⚪ | **Statistiques d'utilisation** | Nombre d'appels outil, fichiers modifiés, commandes exécutées par session. |

---

## 7. 🏗️ Architecture & Évolutions

| Priorité | Fonctionnalité | Description |
|---|---|---|
| ⚪ | **Plugin system** | Permettre aux utilisateurs d'écrire leurs propres outils (Go natif, ou via Lua/Python) chargés dynamiquement. |
| ⚪ | **MCP Protocol** | Implémenter le [Model Context Protocol](https://github.com/modelcontextprotocol) pour standardiser les outils et interopérer avec d'autres agents. |
| ⚪ | **Multi-session / Tabs** | Plusieurs conversations simultanées accessibles via des onglets ou un sélecteur. |
| ⚪ | **`talos init`** | Initialiser un projet Talos : créer `.talos/`, écrire un fichier de configuration `.talos/talos.yaml` détaillé. |
| ⚪ | **Mode headless / API** | Exposer Talos comme un serveur HTTP local (ou via socket) pour l'intégrer dans VS Code, Neovim, Helix, etc. |
| ⚪ | **WebUI** | Optionnellement, une interface web légère (via un serveur Go intégré) pour ceux qui préfèrent le navigateur. |

---

## 🎯 Priorités recommandées (short-term)

Ces fonctionnalités sont celles qui apportent le plus de valeur avec un effort d'implémentation raisonnable :

1. **🔵 Charger une conversation passée** — les données existent, plus qu'à coder l'UI
2. **🔵 Attacher des fichiers (`/attach`)** — très utile au quotidien
3. **🔵 Diff avant/après + confirmation** — sécurité et confiance
4. **🔵 Instructions personnalisées (`/instructions`)** — flexibilité immédiate
5. **🔵 Paramètres du LLM (`/temperature`, etc.)** — contrôle fin du comportement
6. **🔵 Contexte projet automatique** — meilleure compréhension du projet
7. **🔵 Git integration (`/commit`, `/diff`)** — workflow tout-en-un
8. **🔵 Undo / rollback** — filet de sécurité

---

*Cette roadmap est évolutive. Les idées marquées ⚪ sont des suggestions ouvertes à discussion avant implémentation.*