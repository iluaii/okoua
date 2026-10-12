<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- A news item as a "sticker" card: title and source link on the left, a ⋮ menu on the right. -->
<script lang="ts">
	import type { NewsItem } from '#lib/news/types.ts';
	import { formatDuration } from '#lib/format.ts';
	import { feedbackDialog } from '#lib/feedback.svelte.ts';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	interface Props {
		item: NewsItem;
		now: number;
	}
	let { item, now }: Props = $props();

	// Very short posts don't need the AI.
	const canAsk = $derived(item.title.length + item.text.length >= 120);

	let menuOpen = $state(false);
	let copied = $state(false);
	let card = $state<HTMLElement>();

	function askAI() {
		menuOpen = false;
		goto(`${resolve('/ai')}?news=${encodeURIComponent(item.id)}`);
	}

	async function copyLink() {
		menuOpen = false;
		try {
			await navigator.clipboard.writeText(item.url);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch {
			// Clipboard can be unavailable (insecure origin, permissions); nothing else to do.
		}
	}

	function report() {
		menuOpen = false;
		feedbackDialog.open?.('other', 'other');
	}

	function onwindowclick(e: MouseEvent) {
		if (menuOpen && card && !card.contains(e.target as Node)) menuOpen = false;
	}
	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') menuOpen = false;
	}
</script>

<svelte:window onclick={onwindowclick} {onkeydown} />

<article class="sticker {item.source.kind}" bind:this={card}>
	<header>
		<div class="head">
			<h3>{item.title}</h3>
			<a href={item.url} target="_blank" rel="noopener">{item.source.name}</a>
			<span class="time">{formatDuration(item.publishedAt, now)} тому</span>
		</div>
		<div class="menu-wrap">
			<button class="dots" aria-label="Дії" aria-haspopup="menu" aria-expanded={menuOpen} onclick={() => (menuOpen = !menuOpen)}>⋮</button>
			{#if menuOpen}
				<div class="menu" role="menu">
					{#if canAsk}
						<button role="menuitem" onclick={askAI}>✨ Переказ ШІ</button>
					{/if}
					<a role="menuitem" href={item.url} target="_blank" rel="noopener" onclick={() => (menuOpen = false)}>↗ Відкрити джерело</a>
					<button role="menuitem" onclick={copyLink}>🔗 Скопіювати посилання</button>
					<button role="menuitem" onclick={report}>⚑ Поскаржитися</button>
				</div>
			{/if}
		</div>
	</header>

	{#if item.text}
		<p class="text">{item.text}</p>
	{/if}

	{#if copied}
		<span class="toast" role="status">Посилання скопійовано</span>
	{/if}
</article>

<style>
	.sticker {
		position: relative;
		padding: 12px 12px 12px 14px;
		border-radius: 16px;
		background: var(--sticker, var(--surface-2));
		border: 1px solid color-mix(in srgb, var(--sticker-edge, var(--border)) 60%, transparent);
		box-shadow: 0 1px 0 color-mix(in srgb, var(--text) 6%, transparent);
	}
	/* Each kind of source gets its own sticker colour. */
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
	header {
		display: flex;
		align-items: flex-start;
		gap: 8px;
	}
	.head {
		flex: 1;
		min-width: 0;
		display: grid;
		gap: 2px;
	}
	h3 {
		margin: 0;
		font-size: 1rem;
		line-height: 1.3;
	}
	.head a {
		width: fit-content;
		font-size: 0.85rem;
		color: var(--accent);
		text-decoration: none;
	}
	.head a:hover {
		text-decoration: underline;
	}
	.time {
		font-size: 0.75rem;
		color: var(--muted);
	}
	.menu-wrap {
		position: relative;
		flex: none;
	}
	.dots {
		width: 32px;
		height: 32px;
		margin: -4px -4px 0 0;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 1.3rem;
		line-height: 1;
		cursor: pointer;
	}
	.dots:hover,
	.dots[aria-expanded='true'] {
		background: color-mix(in srgb, var(--text) 8%, transparent);
		color: var(--text);
	}
	.menu {
		position: absolute;
		right: 0;
		top: 32px;
		z-index: 5;
		display: grid;
		min-width: 210px;
		padding: 4px;
		border-radius: 12px;
		background: var(--surface);
		border: 1px solid var(--border);
		box-shadow: var(--shadow);
	}
	.menu button,
	.menu a {
		padding: 9px 10px;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--text);
		font: inherit;
		font-size: 0.9rem;
		text-align: left;
		text-decoration: none;
		cursor: pointer;
	}
	.menu button:hover,
	.menu a:hover {
		background: var(--surface-2);
	}
	.text {
		margin: 8px 0 0;
		font-size: 0.9rem;
		color: color-mix(in srgb, var(--text) 85%, transparent);
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.toast {
		position: absolute;
		right: 12px;
		bottom: 10px;
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--text);
		color: var(--bg);
		font-size: 0.75rem;
	}
</style>
