<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!--
	AI mode for one news post: the post as a card, then a chat made only of fixed questions:
	«Короткий переказ» and follow-up questions the AI suggests about this post. No free text.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { AiAnswer, NewsItem } from '#lib/news/types.ts';
	import { formatDuration } from '#lib/format.ts';
	import { feedbackDialog } from '#lib/feedback.svelte.ts';
	import { listChats, loadChat, removeChat, saveChat, type Chat, type ChatMessage as Message } from '#lib/chats.ts';

	let newsId = $state<string | null>(null);
	let item = $state<NewsItem | null>(null);
	let itemState = $state<'none' | 'loading' | 'ok' | 'missing'>('none');
	let messages = $state<Message[]>([]);
	let suggestions = $state<string[]>([]);
	let asked = $state<string[]>([]);
	let thinking = $state(false);
	let now = $state(Date.now());
	let chatEnd = $state<HTMLElement>();
	let saved = $state<Chat[]>([]);

	// The page is prerendered, so the query string is read in the browser only.
	function readQuery() {
		const id = new URLSearchParams(location.search).get('news');
		if (id === newsId && itemState !== 'none') return;
		newsId = id;
		messages = [];
		suggestions = [];
		asked = [];
		item = null;
		if (id) open(id);
		else {
			itemState = 'none';
			saved = listChats();
		}
	}

	function persist() {
		if (item) saveChat({ item, messages, suggestions, asked, updatedAt: Date.now() });
	}

	function forget(id: string) {
		removeChat(id);
		saved = listChats();
	}
	onMount(() => {
		const t = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(t);
	});
	afterNavigate(readQuery);

	async function open(id: string) {
		// A chat saved in this browser comes back as it was, without asking the AI again.
		const chat = loadChat(id);
		if (chat) {
			item = chat.item;
			messages = chat.messages;
			suggestions = chat.suggestions;
			asked = chat.asked;
			itemState = 'ok';
			// The last question may have gone unanswered (errors aren't saved): offer a retry.
			const last = messages.at(-1);
			if (last?.role === 'user') {
				const q = last.text === 'Короткий переказ' ? 'summary' : last.text;
				messages.push({ role: 'error', text: 'Відповіді на це запитання немає.', retry: q });
			}
			scrollDown();
			return;
		}
		itemState = 'loading';
		try {
			const res = await fetch(`/api/news-item?id=${encodeURIComponent(id)}`);
			if (!res.ok) {
				itemState = 'missing';
				return;
			}
			item = (await res.json()) as NewsItem;
			itemState = 'ok';
			await ask('summary');
		} catch {
			itemState = 'missing';
		}
	}

	async function ask(question: string, retry = false) {
		if (!newsId || thinking) return;
		const label = question === 'summary' ? 'Короткий переказ' : question;
		if (!retry) {
			messages.push({ role: 'user', text: label });
			asked.push(label);
		}
		suggestions = suggestions.filter((s) => s !== question);
		thinking = true;
		scrollDown();
		// The server gives up after 30 s; this is a safety net so the chat never hangs on «думає…».
		const ctrl = new AbortController();
		const timer = setTimeout(() => ctrl.abort(), 40_000);
		try {
			const res = await fetch('/api/ai/news', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ newsId, question }),
				signal: ctrl.signal
			});
			if (res.ok) {
				const a = (await res.json()) as AiAnswer;
				messages.push({ role: 'ai', text: a.answer, model: a.model });
				// Keep the suggestions fresh: the model avoids questions already asked.
				suggestions = a.questions.filter((q) => !asked.includes(q));
			} else {
				const body = (await res.json().catch(() => ({}))) as { error?: string };
				messages.push({ role: 'error', text: errorText(res.status, body.error), retry: res.status === 502 || res.status === 429 ? question : undefined });
			}
		} catch {
			messages.push({
				role: 'error',
				text: ctrl.signal.aborted ? 'ШІ не відповів вчасно. Спробуйте ще раз.' : "Немає зв'язку з сервером. Спробуйте пізніше.",
				retry: question
			});
		} finally {
			clearTimeout(timer);
			thinking = false;
			persist();
			scrollDown();
		}
	}

	function errorText(status: number, error?: string): string {
		if (status === 429) return 'Забагато запитів. Спробуйте за кілька хвилин.';
		if (status === 503 && error === 'daily limit') return 'Ліміт запитів до ШІ на сьогодні вичерпано. Читайте оригінал за посиланням.';
		if (status === 503 && error === 'ai disabled') return 'ШІ зараз вимкнено.';
		return 'ШІ зараз не відповідає. Читайте оригінал за посиланням.';
	}

	function scrollDown() {
		requestAnimationFrame(() => chatEnd?.scrollIntoView({ behavior: 'smooth', block: 'end' }));
	}
</script>

<svelte:head>
	<title>ШІ</title>
</svelte:head>

<div class="page">
	<p class="disclaimer">ШІ може помилятися. Не сприймайте цю інформацію як єдине джерело.</p>

	{#if itemState === 'none'}
		<h1>ШІ</h1>
		<p>Тут ШІ допомагає розібратися в новинах вашої області: коротко переказує допис і відповідає на запитання про нього.</p>
		<p class="muted">Як почати: натисніть на область на карті → «Новини» → «⋮» на картці новини → «Переказ ШІ».</p>

		{#if saved.length > 0}
			<h2 class="saved-title">Ваші розмови</h2>
			<ul class="saved">
				{#each saved as c (c.item.id)}
					<li>
						<a href={`${resolve('/ai')}?news=${encodeURIComponent(c.item.id)}`}>
							<span class="saved-name">{c.item.title}</span>
							<span class="saved-meta">{c.item.source.name} · {formatDuration(new Date(c.updatedAt).toISOString(), now)} тому</span>
						</a>
						<button onclick={() => forget(c.item.id)} aria-label="Видалити розмову">✕</button>
					</li>
				{/each}
			</ul>
			<p class="muted small">Розмови зберігаються лише в цьому браузері, тиждень.</p>
		{/if}
	{:else if itemState === 'loading'}
		<p class="muted">Завантаження…</p>
	{:else if itemState === 'missing'}
		<p class="muted">Цієї новини вже немає: новини зберігаються тиждень.</p>
	{:else if item}
		<a class="back" href={`${resolve('/')}?region=${item.region}`}>← На карту</a>

		<article class="sticker {item.source.kind}">
			<h2>{item.title}</h2>
			<a href={item.url} target="_blank" rel="noopener">{item.source.name}</a>
			<span class="time">{formatDuration(item.publishedAt, now)} тому</span>
			{#if item.text}
				<p>{item.text}</p>
			{/if}
		</article>

		<div class="chat" aria-live="polite">
			{#each messages as m, i (i)}
				{#if m.role === 'user'}
					<p class="bubble user">{m.text}</p>
				{:else if m.role === 'ai'}
					<div class="bubble ai">
						<span class="who">✨ ШІ</span>
						<p>{m.text}</p>
						<button class="report" onclick={() => feedbackDialog.open?.('ai', 'hallucination')}>Поскаржитися на відповідь</button>
					</div>
				{:else}
					<div class="bubble error">
						<p>{m.text}</p>
						{#if m.retry && i === messages.length - 1}
							<button class="retry" disabled={thinking} onclick={() => ask(m.retry!, true)}>Спробувати ще раз</button>
						{/if}
					</div>
				{/if}
			{/each}
			{#if thinking}
				<p class="bubble ai thinking"><span class="who">✨ ШІ</span> думає… (до півхвилини)</p>
			{/if}
		</div>

		{#if suggestions.length > 0 && !thinking}
			<div class="suggestions" aria-label="Запитання">
				{#each suggestions as q (q)}
					<button onclick={() => ask(q)}>{q}</button>
				{/each}
			</div>
		{/if}
		<div class="chat-end" bind:this={chatEnd}></div>
	{/if}
</div>

<style>
	.saved-title {
		margin: 24px 0 8px;
		font-size: 1.05rem;
	}
	.saved {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 8px;
	}
	.saved li {
		display: flex;
		align-items: center;
		gap: 6px;
		border: 1px solid var(--border);
		border-radius: 14px;
		background: var(--surface);
	}
	.saved a {
		flex: 1;
		min-width: 0;
		display: grid;
		gap: 2px;
		padding: 10px 12px;
		color: var(--text);
		text-decoration: none;
	}
	.saved-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 600;
	}
	.saved-meta {
		font-size: 0.8rem;
		color: var(--muted);
	}
	.saved button {
		flex: none;
		width: 36px;
		height: 36px;
		margin-right: 6px;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
	}
	.saved button:hover {
		background: var(--surface-2);
	}
	.small {
		font-size: 0.8rem;
	}
	/* Scroll the last message above the floating menu. */
	.chat-end {
		scroll-margin-bottom: calc(var(--nav-h) + 40px);
	}
	.disclaimer {
		margin: 0 0 14px;
		padding: 6px 10px;
		border-radius: 10px;
		background: var(--warn-bg);
		color: var(--warn-text);
		font-size: 0.75rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		text-align: center;
	}
	.muted {
		color: var(--muted);
	}
	.back {
		display: inline-block;
		margin-bottom: 10px;
		font-size: 0.9rem;
		text-decoration: none;
	}
	.sticker {
		display: grid;
		gap: 2px;
		padding: 12px 14px;
		border-radius: 16px;
		background: var(--sticker, var(--surface-2));
		border: 1px solid color-mix(in srgb, var(--sticker-edge, var(--border)) 60%, transparent);
	}
	.official {
		--sticker: color-mix(in srgb, #3b82f6 12%, var(--surface));
		--sticker-edge: #3b82f6;
	}
	.energy {
		--sticker: color-mix(in srgb, #eab308 16%, var(--surface));
		--sticker-edge: #eab308;
	}
	.media {
		--sticker: color-mix(in srgb, #10b981 11%, var(--surface));
		--sticker-edge: #10b981;
	}
	.sticker h2 {
		margin: 0;
		font-size: 1rem;
		line-height: 1.3;
	}
	.sticker a {
		width: fit-content;
		font-size: 0.85rem;
		text-decoration: none;
	}
	.sticker .time {
		font-size: 0.75rem;
		color: var(--muted);
	}
	.sticker p {
		margin: 6px 0 0;
		font-size: 0.9rem;
		display: -webkit-box;
		-webkit-line-clamp: 4;
		line-clamp: 4;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.chat {
		display: grid;
		gap: 10px;
		margin-top: 16px;
	}
	.bubble {
		max-width: 85%;
		margin: 0;
		padding: 10px 14px;
		border-radius: 16px;
		font-size: 0.95rem;
	}
	.user {
		justify-self: end;
		border-bottom-right-radius: 4px;
		background: var(--accent);
		color: #fff;
	}
	.ai {
		justify-self: start;
		display: grid;
		gap: 4px;
		border-bottom-left-radius: 4px;
		background: var(--surface);
		border: 1px solid var(--border);
	}
	.ai p {
		margin: 0;
	}
	.who {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--accent);
	}
	.thinking {
		color: var(--muted);
	}
	.error {
		justify-self: start;
		background: var(--warn-bg);
		color: var(--warn-text);
	}
	.error p {
		margin: 0;
	}
	.retry {
		margin-top: 8px;
		padding: 6px 12px;
		border: 1px solid currentColor;
		border-radius: 999px;
		background: transparent;
		color: inherit;
		font: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}
	.report {
		justify-self: start;
		padding: 0;
		border: 0;
		background: transparent;
		color: var(--muted);
		font: inherit;
		font-size: 0.75rem;
		text-decoration: underline;
		cursor: pointer;
	}
	.suggestions {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 12px;
	}
	.suggestions button {
		padding: 8px 14px;
		border: 1px solid var(--accent);
		border-radius: 999px;
		background: var(--surface);
		color: var(--accent);
		font: inherit;
		font-size: 0.9rem;
		cursor: pointer;
	}
	.suggestions button:hover {
		background: color-mix(in srgb, var(--accent) 10%, var(--surface));
	}
</style>
