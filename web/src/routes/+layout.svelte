<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { feed } from '#lib/feed.svelte.ts';

	let { children } = $props();

	onMount(() => feed.start());

	const trim = (p: string) => p.replace(/\/$/, '') || '/';

	const nav = [
		{ href: resolve('/'), label: 'Карта', icon: 'M3 6l6-3 6 3 6-3v15l-6 3-6-3-6 3zM9 3v15M15 6v15' },
		{ href: resolve('/ai'), label: 'ШІ', icon: 'M12 3l2.2 5.8L20 11l-5.8 2.2L12 19l-2.2-5.8L4 11l5.8-2.2z' },
		{
			href: resolve('/settings'),
			label: 'Налаштування',
			icon: 'M4 7h10M18 7h2M4 17h4M12 17h8M16 5v4M10 15v4'
		}
	];
</script>

<!-- Full-width header; below it either the full-screen map or a tablet-width column. -->
<header class="top">
	<a class="brand" href={resolve('/')}>Концепт</a>
	<div class="right">
		{#if feed.state?.source === 'mock'}
			<span class="mock" title="Сервер працює на тестових даних">Тестові дані</span>
		{/if}
		<span class="status {feed.status.kind}" title={feed.status.detail}>
			<span class="dot" aria-hidden="true"></span>{feed.status.short}
		</span>
	</div>
</header>

<main>
	{@render children()}
</main>

<nav aria-label="Головне меню">
	{#each nav as item (item.href)}
		<a href={item.href} aria-current={trim(page.url.pathname) === trim(item.href) ? 'page' : undefined}>
			<svg viewBox="0 0 24 24" aria-hidden="true"><path d={item.icon} /></svg>
			<span>{item.label}</span>
		</a>
	{/each}
</nav>

<style>
	.top {
		position: fixed;
		inset: 0 0 auto 0;
		z-index: 20;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		height: calc(var(--header-h) + env(safe-area-inset-top));
		padding: env(safe-area-inset-top) max(var(--gutter), env(safe-area-inset-right)) 0
			max(var(--gutter), env(safe-area-inset-left));
		background: color-mix(in srgb, var(--surface) 90%, transparent);
		backdrop-filter: blur(12px);
		border-bottom: 1px solid var(--border);
	}
	.brand {
		font-weight: 700;
		font-size: 1.1rem;
		letter-spacing: 0.01em;
		color: var(--text);
		text-decoration: none;
	}
	.right {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}
	main {
		padding-top: calc(var(--header-h) + env(safe-area-inset-top));
	}
	.status {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--surface-2);
		font-size: 0.8rem;
		color: var(--muted);
		white-space: nowrap;
	}
	.status.warn {
		background: var(--warn-bg);
		color: var(--warn-text);
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--muted);
	}
	.ok .dot {
		background: var(--ok);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--ok) 25%, transparent);
	}
	.warn .dot {
		background: #e0a100;
	}
	.mock {
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--warn-bg);
		color: var(--warn-text);
		font-size: 0.75rem;
		font-weight: 600;
		white-space: nowrap;
	}

	/* Pill menu at the bottom, the same on phone and desktop. */
	nav {
		position: fixed;
		left: 50%;
		bottom: calc(12px + env(safe-area-inset-bottom));
		z-index: 20;
		transform: translateX(-50%);
		display: flex;
		width: min(calc(100% - 2 * var(--gutter)), 420px);
		height: var(--nav-h);
		padding: 6px;
		border-radius: 20px;
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: blur(12px);
		border: 1px solid var(--border);
		box-shadow: var(--shadow);
	}
	nav a {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		border-radius: 14px;
		color: var(--muted);
		text-decoration: none;
		font-size: 0.72rem;
	}
	nav a:hover {
		color: var(--text);
	}
	nav a[aria-current='page'] {
		background: var(--surface-2);
		color: var(--accent);
		font-weight: 600;
	}
	nav svg {
		width: 22px;
		height: 22px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
