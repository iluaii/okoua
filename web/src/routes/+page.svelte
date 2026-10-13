<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import UkraineMap from '#lib/components/UkraineMap.svelte';
	import RegionCard from '#lib/components/RegionCard.svelte';
	import Sheet from '#lib/components/Sheet.svelte';
	import PowerPanel from '#lib/components/PowerPanel.svelte';
	import NewsPanel from '#lib/components/NewsPanel.svelte';
	import { loadPower, type PowerLoad } from '#lib/power/types.ts';
	import { feed } from '#lib/feed.svelte.ts';
	import { regions } from '#lib/map/regions.ts';
	import type { RegionState } from '#lib/types.ts';

	let selected = $state<number | null>(null);
	let now = $state(Date.now());
	let map = $state<ReturnType<typeof UkraineMap>>();

	onMount(() => {
		// Opened from a notification: /?region=25 shows that region.
		const fromLink = Number(new URLSearchParams(location.search).get('region'));
		if (regions.some((r) => r.uid === fromLink)) selected = fromLink;
		const tick = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(tick);
	});

	const states = $derived(new Map<number, RegionState>(feed.state?.regions.map((r) => [r.uid, r]) ?? []));
	const selectedRegion = $derived(regions.find((r) => r.uid === selected));

	let sheetHeight = $state(0);

	// What the sheet shows for the selected region.
	let mode = $state<'info' | 'power' | 'news'>('info');

	let power = $state<PowerLoad>({ kind: 'loading' });
	let powerController: AbortController | undefined;

	function select(uid: number) {
		selected = selected === uid ? null : uid;
		mode = 'info';
	}

	async function openPower() {
		if (selected === null) return;
		const uid = selected;
		mode = 'power';
		powerController?.abort();
		powerController = new AbortController();
		power = { kind: 'loading' };
		const result = await loadPower(uid, powerController.signal);
		if (selected === uid) power = result;
	}

	// The sheet must not cover the selected region: pan the map once the sheet is open.
	$effect(() => {
		const uid = selected;
		const h = sheetHeight;
		if (uid === null || h === 0) return;
		// reveal reads the current map view; without untrack this effect would rerun on every pan.
		untrack(() => {
			const navTop = document.querySelector('nav')?.getBoundingClientRect().top ?? window.innerHeight;
			map?.reveal(uid, h + (window.innerHeight - navTop) + 16);
		});
	});
</script>

<svelte:head>
	<title>Карта тривог</title>
</svelte:head>

<!-- Full-screen map under the header; the menu and sheet float over it. -->
<div class="stage">
	<UkraineMap bind:this={map} {states} {selected} onselect={select} onclear={() => (selected = null)} />

	{#if feed.status.detail}
		<p class="warning" role="status">{feed.status.detail}</p>
	{/if}

	<div class="legend" aria-hidden="true">
		<span><i class="sw full"></i>Тривога</span>
		<span><i class="sw partial"></i>Окремі громади</span>
		<span><i class="sw calm"></i>Немає</span>
	</div>

	<div class="zoom">
		<button onclick={() => map?.zoomBy(1.6)} aria-label="Наблизити">+</button>
		<button onclick={() => map?.zoomBy(1 / 1.6)} aria-label="Віддалити">−</button>
		<button onclick={() => map?.reset()} aria-label="Показати всю карту">
			<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5" /></svg>
		</button>
	</div>
</div>

{#if selectedRegion}
	<Sheet label={selectedRegion.name} onclose={() => (selected = null)} bind:height={sheetHeight} tall={mode !== 'info'}>
		{#if mode === 'power'}
			<PowerPanel
				region={selectedRegion}
				load={power}
				{now}
				onback={() => (mode = 'info')}
				onclose={() => (selected = null)}
				onretry={openPower}
			/>
		{:else if mode === 'news'}
			<NewsPanel region={selectedRegion} {now} onback={() => (mode = 'info')} onclose={() => (selected = null)} />
		{:else}
			<RegionCard
				region={selectedRegion}
				state={states.get(selectedRegion.uid)}
				{now}
				onclose={() => (selected = null)}
				onpower={openPower}
				onnews={() => (mode = 'news')}
			/>
		{/if}
	</Sheet>
{/if}

<style>
	.stage {
		position: fixed;
		inset: calc(var(--header-h) + env(safe-area-inset-top)) 0 0 0;
		overflow: hidden;
		background: var(--bg);
	}
	.warning {
		position: absolute;
		top: 12px;
		left: 50%;
		transform: translateX(-50%);
		width: max-content;
		max-width: calc(100% - 2 * var(--gutter));
		margin: 0;
		padding: 8px 12px;
		border-radius: 12px;
		background: var(--warn-bg);
		color: var(--warn-text);
		font-size: 0.85rem;
		box-shadow: var(--shadow);
	}
	.legend {
		position: absolute;
		left: var(--gutter);
		top: 12px;
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 8px 10px;
		border-radius: 12px;
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: blur(8px);
		border: 1px solid var(--border);
		font-size: 0.75rem;
		color: var(--muted);
		pointer-events: none;
	}
	.legend span {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.sw {
		flex: none;
		display: inline-block;
		width: 11px;
		height: 11px;
		border-radius: 3px;
		background: var(--map-calm);
		border: 1px solid var(--border);
	}
	.sw.full {
		background: var(--alert);
		border-color: transparent;
	}
	.sw.partial {
		background: repeating-linear-gradient(45deg, var(--alert) 0 2px, var(--map-calm) 2px 4px);
	}
	.zoom {
		position: absolute;
		right: var(--gutter);
		top: 12px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		border-radius: 12px;
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: blur(8px);
		border: 1px solid var(--border);
	}
	.zoom button {
		width: 40px;
		height: 40px;
		border: 0;
		background: transparent;
		color: var(--text);
		font: inherit;
		font-size: 1.25rem;
		line-height: 1;
		cursor: pointer;
	}
	.zoom button + button {
		border-top: 1px solid var(--border);
	}
	.zoom button:hover {
		background: var(--surface-2);
	}
	.zoom svg {
		width: 18px;
		height: 18px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
		vertical-align: middle;
	}
	/* When a warning is shown at the top, move the legend and buttons down. */
	.warning ~ .legend,
	.warning ~ .zoom {
		top: 64px;
	}
</style>
