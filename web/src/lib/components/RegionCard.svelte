<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import type { MapRegion } from '#lib/map/regions.ts';
	import type { RegionState } from '#lib/types.ts';
	import { alertTypeLabel, formatDuration, formatStart } from '#lib/format.ts';

	interface Props {
		region: MapRegion;
		state: RegionState | undefined;
		now: number;
		onclose: () => void;
	}
	let { region, state, now, onclose }: Props = $props();
</script>

<section class="card" aria-labelledby="region-title">
	<header>
		<h2 id="region-title">{region.name}</h2>
		<button class="close" onclick={onclose} aria-label="Закрити">✕</button>
	</header>

	{#if !state}
		<p class="status calm">Тривоги немає</p>
	{:else}
		<p class="status" class:partial={state.status === 'partial'}>
			{state.status === 'full' ? 'Тривога' : 'Тривога в частині області'}
		</p>
		<ul class="alerts">
			{#each state.alerts as a (a.id)}
				<li>
					<span class="type">{alertTypeLabel(a.type)}</span>
					{#if state.status === 'partial' || a.locationType !== 'oblast'}
						<span class="where">{a.locationTitle}</span>
					{/if}
					<span class="when">{formatStart(a.startedAt, now)} · {formatDuration(a.startedAt, now)}</span>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="actions">
		<button disabled>Стежити</button>
		<button disabled>Новини</button>
		<button disabled>Світло</button>
	</div>
	<p class="soon">Сповіщення, новини та графіки світла з'являться незабаром.</p>
	<p class="soon">Сервіс доповнює офіційні джерела, а не замінює їх. Завжди реагуйте на сирени.</p>
</section>

<style>
	.card {
		padding: 4px 0 0;
	}
	header {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 8px;
	}
	h2 {
		margin: 0;
		font-size: 1.15rem;
		line-height: 1.3;
	}
	.close {
		flex: none;
		width: 32px;
		height: 32px;
		margin: -4px -6px 0 0;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 1rem;
		cursor: pointer;
	}
	.close:hover {
		background: var(--surface-2);
	}
	.status {
		margin: 6px 0 0;
		font-weight: 600;
		color: var(--alert-text);
	}
	.status.calm {
		color: var(--muted);
		font-weight: 500;
	}
	.alerts {
		list-style: none;
		margin: 10px 0 0;
		padding: 0;
		display: grid;
		gap: 8px;
	}
	.alerts li {
		display: grid;
		gap: 2px;
		padding: 8px 10px;
		border-radius: 10px;
		background: var(--surface-2);
	}
	.type {
		font-weight: 600;
	}
	.where,
	.when {
		font-size: 0.9rem;
		color: var(--muted);
	}
	.actions {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 8px;
		margin-top: 14px;
	}
	.actions button {
		padding: 9px 6px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.9rem;
	}
	.actions button:disabled {
		opacity: 0.55;
	}
	.soon {
		margin: 8px 0 0;
		font-size: 0.8rem;
		color: var(--muted);
	}
</style>
