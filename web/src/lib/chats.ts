// SPDX-License-Identifier: AGPL-3.0-or-later

// AI chats saved in this browser only (localStorage), so leaving AI mode loses nothing.
// Storage can be unavailable or cleared at any time; everything here fails quietly.

import type { NewsItem } from './news/types.ts';

export type ChatMessage =
	| { role: 'user'; text: string }
	// question: 'summary' or the follow-up as asked; missing in chats saved before reports.
	| { role: 'ai'; text: string; model: string; question?: string; reported?: boolean }
	// An answer withheld after reports.
	| { role: 'hidden' }
	| { role: 'error'; text: string; retry?: string };

export interface Chat {
	item: NewsItem;
	messages: ChatMessage[];
	suggestions: string[];
	asked: string[];
	updatedAt: number;
}

const KEY = 'ai.chats';
const MAX_CHATS = 30;
const MAX_AGE = 7 * 24 * 3600 * 1000; // the news itself is kept for a week

function readAll(): Record<string, Chat> {
	try {
		const raw = JSON.parse(localStorage.getItem(KEY) ?? '{}');
		return raw && typeof raw === 'object' ? (raw as Record<string, Chat>) : {};
	} catch {
		return {};
	}
}

function writeAll(chats: Record<string, Chat>) {
	try {
		localStorage.setItem(KEY, JSON.stringify(chats));
	} catch {
		// Full or unavailable storage: the chat just won't be remembered.
	}
}

/** Saved chats, newest first, without old ones. */
export function listChats(): Chat[] {
	const now = Date.now();
	return Object.values(readAll())
		.filter((c) => c?.item?.id && now - c.updatedAt < MAX_AGE)
		.sort((a, b) => b.updatedAt - a.updatedAt);
}

export function loadChat(newsId: string): Chat | null {
	const c = readAll()[newsId];
	return c && Date.now() - c.updatedAt < MAX_AGE ? c : null;
}

export function saveChat(chat: Chat) {
	const all = readAll();
	// Errors are not worth restoring; drop them from the saved copy.
	all[chat.item.id] = { ...chat, messages: chat.messages.filter((m) => m.role !== 'error'), updatedAt: Date.now() };
	// Keep the newest chats only.
	const keep = Object.values(all)
		.filter((c) => Date.now() - c.updatedAt < MAX_AGE)
		.sort((a, b) => b.updatedAt - a.updatedAt)
		.slice(0, MAX_CHATS);
	writeAll(Object.fromEntries(keep.map((c) => [c.item.id, c])));
}

export function removeChat(newsId: string) {
	const all = readAll();
	delete all[newsId];
	writeAll(all);
}
