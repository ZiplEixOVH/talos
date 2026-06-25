import sqlite3 from 'sqlite3';
import path from 'path';
import { app } from 'electron';

let db: sqlite3.Database;

export function initDb(): Promise<void> {
  return new Promise((resolve, reject) => {
    const dbPath = path.join(app.getPath('userData'), 'talos.db');
    
    const sqlite = sqlite3.verbose();
    
    db = new sqlite.Database(dbPath, (err) => {
      if (err) {
        console.error('Failed to open SQLite database:', err);
        reject(err);
      } else {
        console.log('SQLite database opened at:', dbPath);
        
        // Activer la contrainte de clé étrangère et la suppression en cascade
        db.run("PRAGMA foreign_keys = ON;", (err) => {
          if (err) console.error('Failed to enable foreign keys:', err);
        });
        
        // Créer les tables de la base de données de manière sérialisée
        db.serialize(() => {
          // Table des Chats
          db.run(
            `CREATE TABLE IF NOT EXISTS chats (
              id TEXT PRIMARY KEY,
              title TEXT NOT NULL,
              created_at INTEGER NOT NULL
            )`,
            (err) => {
              if (err) console.error('Failed to create table chats:', err);
            }
          );
          
          // Table des Messages rattachés aux chats
          db.run(
            `CREATE TABLE IF NOT EXISTS messages (
              id TEXT PRIMARY KEY,
              chat_id TEXT NOT NULL,
              role TEXT NOT NULL,
              content TEXT NOT NULL,
              created_at INTEGER NOT NULL,
              FOREIGN KEY(chat_id) REFERENCES chats(id) ON DELETE CASCADE
            )`,
            (err) => {
              if (err) console.error('Failed to create table messages:', err);
            }
          );
          
          // Table des Paramètres de l'application (modèle actif, provider actif, etc.)
          db.run(
            `CREATE TABLE IF NOT EXISTS app_settings (
              key TEXT PRIMARY KEY,
              value TEXT NOT NULL
            )`,
            (err) => {
              if (err) console.error('Failed to create table app_settings:', err);
            }
          );
          
          // Table des Providers
          db.run(
            `CREATE TABLE IF NOT EXISTS providers (
              id TEXT PRIMARY KEY,
              name TEXT NOT NULL,
              base_url TEXT NOT NULL,
              api_key TEXT
            )`,
            (err) => {
              if (err) console.error('Failed to create table providers:', err);
            }
          );

          // Table des Modèles
          db.run(
            `CREATE TABLE IF NOT EXISTS models (
              id TEXT PRIMARY KEY,
              provider_id TEXT NOT NULL,
              name TEXT NOT NULL,
              FOREIGN KEY(provider_id) REFERENCES providers(id) ON DELETE CASCADE
            )`,
            (err) => {
              if (err) {
                console.error('Failed to create table models:', err);
              } else {
                // Initialiser les données par défaut si vide
                initializeDefaultData();
              }
            }
          );
        });
        
        resolve();
      }
    });
  });
}

function initializeDefaultData() {
  db.serialize(() => {
    // Initialisation d'Ollama par défaut si vide
    db.get(`SELECT COUNT(*) as count FROM providers`, (err, row: any) => {
      if (!err && row && row.count === 0) {
        console.log('Populating default Ollama provider (with /v1 endpoint)');
        
        // Ajouter le provider Ollama
        db.run(
          `INSERT INTO providers (id, name, base_url, api_key) VALUES (?, ?, ?, ?)`,
          ['ollama', 'Ollama', 'http://localhost:11434/v1', '']
        );
      }
    });
  });
}

// ==========================================
// CHATS DATABASE METHODS
// ==========================================

export function getChats(): Promise<Array<{ id: string; title: string; created_at: number }>> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.all(
      `SELECT id, title, created_at FROM chats ORDER BY created_at DESC`,
      (err, rows) => {
        if (err) reject(err);
        else resolve(rows as any);
      }
    );
  });
}

export function createChat(id: string, title: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    const createdAt = Date.now();
    db.run(
      `INSERT INTO chats (id, title, created_at) VALUES (?, ?, ?)`,
      [id, title, createdAt],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

export function deleteChat(id: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(
      `DELETE FROM chats WHERE id = ?`,
      [id],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

export function renameChat(id: string, title: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(
      `UPDATE chats SET title = ? WHERE id = ?`,
      [title, id],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

// ==========================================
// MESSAGES DATABASE METHODS
// ==========================================

export function getMessages(chatId: string): Promise<Array<{ id: string; role: string; content: string }>> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.all(
      `SELECT id, role, content FROM messages WHERE chat_id = ? ORDER BY created_at ASC`,
      [chatId],
      (err, rows) => {
        if (err) reject(err);
        else resolve(rows as any);
      }
    );
  });
}

export function addMessage(id: string, chatId: string, role: string, content: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    const createdAt = Date.now();
    db.run(
      `INSERT INTO messages (id, chat_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
      [id, chatId, role, content, createdAt],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

// ==========================================
// APPLICATION SETTINGS DATABASE METHODS
// ==========================================

export function getSetting(key: string, defaultValue: string): Promise<string> {
  return new Promise((resolve) => {
    if (!db) return resolve(defaultValue);
    db.get(
      `SELECT value FROM app_settings WHERE key = ?`,
      [key],
      (err, row: any) => {
        if (err || !row) resolve(defaultValue);
        else resolve(row.value);
      }
    );
  });
}

export function setSetting(key: string, value: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(
      `INSERT OR REPLACE INTO app_settings (key, value) VALUES (?, ?)`,
      [key, value],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

// ==========================================
// PROVIDERS & MODELS DATABASE METHODS
// ==========================================

export function getProviders(): Promise<Array<{ id: string; name: string; base_url: string; api_key: string }>> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.all(`SELECT id, name, base_url, api_key FROM providers ORDER BY name ASC`, (err, rows) => {
      if (err) reject(err);
      else resolve(rows as any);
    });
  });
}

export function saveProvider(id: string, name: string, baseUrl: string, apiKey: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(
      `INSERT OR REPLACE INTO providers (id, name, base_url, api_key) VALUES (?, ?, ?, ?)`,
      [id, name, baseUrl, apiKey],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

export function deleteProvider(id: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(`DELETE FROM providers WHERE id = ?`, [id], (err) => {
      if (err) reject(err);
      else resolve();
    });
  });
}

export function getModels(providerId: string): Promise<Array<{ id: string; name: string }>> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.all(
      `SELECT id, name FROM models WHERE provider_id = ? ORDER BY name ASC`,
      [providerId],
      (err, rows) => {
        if (err) reject(err);
        else resolve(rows as any);
      }
    );
  });
}

export function addModel(id: string, providerId: string, name: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(
      `INSERT INTO models (id, provider_id, name) VALUES (?, ?, ?)`,
      [id, providerId, name],
      (err) => {
        if (err) reject(err);
        else resolve();
      }
    );
  });
}

export function deleteModel(id: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error('Database not initialized'));
    db.run(`DELETE FROM models WHERE id = ?`, [id], (err) => {
      if (err) reject(err);
      else resolve();
    });
  });
}
