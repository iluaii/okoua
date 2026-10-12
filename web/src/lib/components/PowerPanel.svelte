<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Power outage schedule for one region: pick a queue, see now/today/tomorrow and all queues. -->
<script lang="ts">
	import type { MapRegion } from '#lib/map/regions.ts';
	import type { PowerDay, PowerSchedule } from '#lib/power/types.ts';
	import { hhmm, hoursOff, kyivNow, powerAt } from '#lib/power/schedule.ts';
	import PowerTimeline from './PowerTimeline.svelte';

	interface Props {
		region: MapRegion;
		schedule: PowerSchedule | null;
		now: number;
		onback: () => void;
		onclose: () => void;
	}
	let { region, schedule, now, onback, onclose }: Props = $props();

	// The chosen queue is a per-browser convenience; storage may be unavailable.
	const storageKey = $derived(`power.queue.${region.uid}`);
	let queue = $state<string | null>(null);
	$effect(() => {
		try {
			queue = localStorage.getItem(storageKey);
		} catch {
			queue = null;
		}
	});
	function pick(q: string) {
		queue = q;
		try {
			localStorage.setItem(storageKey, q);
		} catch {
			// Not saved; the choice still works for this visit.
		}
	}

	const nowKyiv = $derived(kyivNow(now));
	const today = $derived(schedule?.today);
	const mine = $derived(queue && today?.status === 'schedule' ? (today.queues[queue] ?? []) : null);
	const power = $derived(mine ? powerAt(mine, nowKyiv.minutes) : null);

	function dayNote(day: PowerDay): string | null {
		if (day.status === 'waiting') return 'Графік ще не опублікований';
		if (day.status === 'none') return 'Відключень не заплановано';
		return null;
	}

	const updated = $derived(
		schedule ? new Intl.DateTimeFormat('uk-UA', { hour: '2-digit', minute: '2-digit', timeZone: 'Europe/Kyiv' }).format(new Date(schedule.updatedAt)) : ''
	);
</script>

<section class="panel" aria-labelledby="power-title">
	<header>
		<button class="icon" onclick={onback} aria-label="Назад">←</button>
		<h2 id="power-title">Світло · {region.name}</h2>
		<button class="icon" onclick={onclose} aria-label="Закрити">✕</button>
	</header>

	{#if !schedule}
		<p class="empty">Графіки для цієї області поки не підтримуються. Дивіться сайт або Telegram-канал вашого обленерго.</p>
	{:else}
		{#if schedule.demo}
			<p class="demo">Демо: приклад графіка, не справжні дані</p>
		{/if}

		<div class="queues" role="radiogroup" aria-label="Черга">
			{#each schedule.queues as q (q)}
				<button role="radio" aria-checked={queue === q} class:active={queue === q} onclick={() => pick(q)}>{q}</button>
			{/each}
		</div>

		{#if !queue}
			<p class="hint">Оберіть свою чергу, щоб бачити свій графік.</p>
		{:else if today && mine && power}
			<div class="now" class:off={!power.on}>
				<span class="bulb" aria-hidden="true">{power.on ? '💡' : '🌑'}</span>
				<div>
					{#if !power.on}
						<strong>Світла немає</strong>
						<span>увімкнуть о {hhmm(power.until)}</span>
					{:else if power.nextOff !== null}
						<strong>Світло є</strong>
						<span>відключення о {hhmm(power.nextOff)}–{hhmm(power.nextOffTo ?? 1440)}</span>
					{:else}
						<strong>Світло є</strong>
						<span>сьогодні відключень більше немає</span>
					{/if}
				</div>
			</div>

			<h3>Сьогодні <span class="sub">· без світла {hoursOff(mine).toLocaleString('uk-UA')} год</span></h3>
			<PowerTimeline outages={mine} now={nowKyiv.minutes} />

			<h3>Завтра</h3>
			{#if schedule.tomorrow.status === 'schedule'}
				<PowerTimeline outages={schedule.tomorrow.queues[queue] ?? []} />
			{:else}
				<p class="note">{dayNote(schedule.tomorrow)}</p>
			{/if}
		{:else if today}
			<p class="note">{dayNote(today)}</p>
		{/if}

		{#if today?.status === 'schedule'}
			<details open={!queue}>
				<summary>Усі черги на сьогодні</summary>
				<ul class="all">
					{#each schedule.queues as q (q)}
						<li>
							<button class:active={queue === q} onclick={() => pick(q)}>
								<span class="q">{q}</span>
								<PowerTimeline outages={today.queues[q] ?? []} now={nowKyiv.minutes} compact />
							</button>
						</li>
					{/each}
				</ul>
			</details>
		{/if}

		<footer>
			{#if schedule.lookupUrl}
				<a href={schedule.lookupUrl} target="_blank" rel="noopener">Як дізнатися свою чергу →</a>
			{/if}
			<span>
				Джерело:
				{#if schedule.source.url}
					<a href={schedule.source.url} target="_blank" rel="noopener">{schedule.source.name}</a>
				{:else}
					{schedule.source.name}
				{/if}
				· оновлено о {updated}
			</span>
		</footer>
	{/if}
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
	.demo {
		margin: 0 0 10px;
		padding: 5px 10px;
		border-radius: 8px;
		background: var(--warn-bg);
		color: var(--warn-text);
		font-size: 0.8rem;
		font-weight: 600;
	}
	.queues {
		display: grid;
		grid-template-columns: repeat(6, 1fr);
		gap: 6px;
	}
	.queues button {
		padding: 8px 0;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.9rem;
		font-variant-numeric: tabular-nums;
		cursor: pointer;
	}
	.queues button.active {
		background: var(--accent);
		border-color: var(--accent);
		color: #fff;
		font-weight: 600;
	}
	.hint,
	.empty {
		margin: 12px 0 4px;
		color: var(--muted);
	}
	.now {
		display: flex;
		align-items: center;
		gap: 12px;
		margin: 14px 0 4px;
		padding: 12px 14px;
		border-radius: 14px;
		background: color-mix(in srgb, var(--power-on) 55%, var(--surface));
	}
	.now.off {
		background: var(--surface-2);
	}
	.bulb {
		font-size: 1.6rem;
	}
	.now div {
		display: grid;
	}
	.now strong {
		font-size: 1.15rem;
	}
	.now span {
		color: var(--muted);
	}
	h3 {
		margin: 16px 0 8px;
		font-size: 0.95rem;
	}
	.sub {
		font-weight: 400;
		color: var(--muted);
	}
	.note {
		margin: 0;
		color: var(--muted);
		font-size: 0.9rem;
	}
	details {
		margin-top: 18px;
	}
	summary {
		font-weight: 600;
		font-size: 0.95rem;
		cursor: pointer;
	}
	.all {
		list-style: none;
		margin: 10px 0 0;
		padding: 0;
		display: grid;
		gap: 4px;
	}
	.all button {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 5px 8px;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		color: var(--text);
		font: inherit;
		cursor: pointer;
	}
	.all button:hover {
		background: var(--surface-2);
	}
	.all button.active {
		border-color: var(--accent);
	}
	.q {
		flex: none;
		width: 2.4em;
		font-size: 0.85rem;
		font-variant-numeric: tabular-nums;
		text-align: left;
	}
	footer {
		display: grid;
		gap: 4px;
		margin-top: 16px;
		font-size: 0.8rem;
		color: var(--muted);
	}
</style>
