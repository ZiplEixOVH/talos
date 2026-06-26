// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
	interface Window {
		talosAPI?: {
			getChats: () => Promise<Array<{ id: string; title: string; created_at: number }>>;
			createChat: (id: string, title: string) => Promise<void>;
			deleteChat: (id: string) => Promise<void>;
			renameChat: (id: string, title: string) => Promise<void>;
			getDbPath: () => Promise<string>;
			
			getProviders: () => Promise<Array<{ id: string; name: string; base_url: string; api_key: string }>>;
			saveProvider: (id: string, name: string, baseUrl: string, apiKey: string) => Promise<void>;
			deleteProvider: (id: string) => Promise<void>;
			
			getModels: (providerId: string) => Promise<Array<{ id: string; name: string }>>;
			addModel: (id: string, providerId: string, name: string) => Promise<void>;
			deleteModel: (id: string) => Promise<void>;
			
			getMessages: (chatId: string) => Promise<Array<{ id: string; role: string; content: string }>>;
			addMessage: (id: string, chatId: string, role: string, content: string) => Promise<void>;
			
			getSetting: (key: string, defaultValue: string) => Promise<string>;
			setSetting: (key: string, value: string) => Promise<void>;
			
			getCwd: () => Promise<string>;
			selectCwd: () => Promise<string | null>;
			
			chat: (providerId: string, model: string, chatMessages: any[]) => Promise<{ role: string; content: string }>;
			startChatStream: (providerId: string, model: string, chatMessages: any[], chatId: string, requestId: string) => void;
			onChatStreamChunk: (callback: (data: { chatId: string; requestId: string; text: string }) => void) => () => void;
			onChatStreamEnd: (callback: (data: { chatId: string; requestId: string }) => void) => () => void;
			onChatStreamError: (callback: (data: { chatId: string; requestId: string; error: string }) => void) => () => void;
			onChatToolMessage: (callback: (data: { id: string; chatId: string; role: string; content: string }) => void) => () => void;
		};
	}
}

export {};
