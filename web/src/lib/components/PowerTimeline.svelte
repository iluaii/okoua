<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- A 24-hour bar: green when there is power, dark during outages, a marker for "now". -->
<script lang="ts">
	import type { Outage } from '#lib/power/types.ts';
	import { hhmm } from '#lib/power/schedule.ts';

	interface Props {
		outages: Outage[];
		/** Minutes since midnight to mark as "now", or null for another day. */
		now?: number | null;
		/** Thin bar without hour labels, for the all-queues list. */
		compact?: boolean;
	}
	let { outages, now = null, compact = false }: Props = $props();

	const pct = (m: number) => `${(m / 1440) * 100}%`;
	const label = $derived(
		outages.length === 0
			? 'Відключень немає'
			: 'Без світла: ' + outages.map((o) => `${hhmm(o.from)}–${hhmm(o.to)}`).join(', ')
	);
</script>

<div class="timeline" class:compact role="img" aria-label={label}>
	<div class="bar">
		{#each outages as o (o.from)}
			<span class="off" style:left={pct(o.from)} style:width={pct(o.to - o.from)}></span>
		{/each}
		{#if now !== null}
			<span class="now" style:left={pct(now)}></span>
		{/if}
	</div>
	{#if !compact}
		<div class="hours" aria-hidden="true">
			{#each [0, 6, 12, 18, 24] as h (h)}
				<span style:left={pct(h * 60)}>{String(h).padStart(2, '0')}</span>
			{/each}
		</div>
	{/if}
</div>

<style>
	.timeline {
		width: 100%;
	}
	.bar {
		position: relative;
		height: 28px;
		border-radius: 8px;
		overflow: hidden;
		background: var(--power-on);
	}
	.compact .bar {
		height: 14px;
		border-radius: 5px;
	}
	.off {
		position: absolute;
		top: 0;
		bottom: 0;
		background: var(--power-off);
	}
	.now {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 2px;
		margin-left: -1px;
		background: var(--alert);
	}
	.hours {
		position: relative;
		height: 16px;
		margin-top: 3px;
		font-size: 0.7rem;
		color: var(--muted);
	}
	.hours span {
		position: absolute;
		transform: translateX(-50%);
	}
	.hours span:first-child {
		transform: none;
	}
	.hours span:last-child {
		transform: translateX(-100%);
	}
</style>
