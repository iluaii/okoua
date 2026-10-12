<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- News of one region as a list of sticker cards. -->
<script lang="ts">
	import type { MapRegion } from '#lib/map/regions.ts';
	import type { NewsItem } from '#lib/news/types.ts';
	import NewsCard from './NewsCard.svelte';

	interface Props {
		region: MapRegion;
		now: number;
		onback: () => void;
		onclose: () => void;
	}
	let { region, now, onback, onclose }: Props = $props();

	let load = $state<{ kind: 'loading' } | { kind: 'ok'; items: NewsItem[] } | { kind: 'unsupported' } | { kind: 'error' }>({
		kind: 'loading'
	});

	$effect(() => {
		const uid = region.uid;
		const ctrl = new AbortController();
		load = { kind: 'loading' };
		fetch(`/api/news/${uid}`, { signal: ctrl.signal })
			.then(async (res) => {
				if (res.status === 404) load = { kind: 'unsupported' };
				else if (!res.ok) load = { kind: 'error' };
				else load = { kind: 'ok', items: ((await res.json()) as { items: NewsItem[] }).items };
			})
			.catch(() => {
				if (!ctrl.signal.aborted) load = { kind: 'error' };
			});
		return () => ctrl.abort();
	});
</script>

<section aria-labelledby="news-title">
	<header>
		<button class="icon" onclick={onback} aria-label="Назад">←</button>
		<h2 id="news-title">Новини · {region.name}</h2>
		<button class="icon" onclick={onclose} aria-label="Закрити">✕</button>
	</header>

	{#if load.kind === 'loading'}
		<p class="muted">Завантаження…</p>
	{:else if load.kind === 'unsupported'}
		<p class="muted">Новини для цієї області поки не підтримуються.</p>
	{:else if load.kind === 'error'}
		<p class="muted">Не вдалося завантажити новини. Перевірте зв'язок.</p>
	{:else if load.items.length === 0}
		<p class="muted">Поки що новин немає.</p>
	{:else}
		<div class="list">
			{#each load.items as item (item.id)}
				<NewsCard {item} {now} />
			{/each}
		</div>
	{/if}

	<p class="foot">
		Лише перевірені джерела: обласна адміністрація, обленерго, місцеві медіа. Ми не публікуємо, куди летять дрони чи
		ракети, а тривоги показує карта.
	</p>
</section>

<style>
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: -4px -6px 8px;
	}
	h2 {
		flex: 1;
		margin: 0;
		font-size: 1.05rem;
		line-height: 1.3;
	}
	.icon {
		flex: none;
		width: 34px;
		height: 34px;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 1.05rem;
		cursor: pointer;
	}
	.icon:hover {
		background: var(--surface-2);
	}
	.muted {
		margin: 12px 0;
		color: var(--muted);
	}
	.list {
		display: grid;
		gap: 10px;
	}
	.foot {
		margin: 14px 0 0;
		font-size: 0.75rem;
		color: var(--muted);
	}
</style>
