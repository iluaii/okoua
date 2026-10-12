<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Push warnings before the chosen queue's planned outages start and end. -->
<script lang="ts">
	import { follows } from '#lib/push.svelte.ts';

	interface Props {
		region: number;
		queue: string;
	}
	let { region, queue }: Props = $props();

	const offChoices = [5, 15, 30, 60, 120];
	const onChoices: [number, string][] = [
		[-1, 'Не сповіщати'],
		[0, 'Коли мають увімкнути'],
		[15, 'За 15 хв до увімкнення'],
		[30, 'За 30 хв до увімкнення']
	];

	const rule = $derived(follows.powerRule(region));

	function enable() {
		follows.setPower(region, { region, queue, beforeOff: 30, beforeOn: 0 });
	}
	function change(field: 'beforeOff' | 'beforeOn', e: Event) {
		if (!rule) return;
		const v = Number((e.currentTarget as HTMLSelectElement).value);
		follows.setPower(region, { ...rule, queue, [field]: v });
	}
</script>

<div class="alerts">
	{#if rule}
		<div class="head">
			<strong>🔔 Сповіщення для черги {rule.queue}</strong>
			<button class="link" disabled={follows.busy} onclick={() => follows.setPower(region, null)}>Вимкнути</button>
		</div>
		<label>
			<span>Попередити про відключення</span>
			<select value={rule.beforeOff} disabled={follows.busy} onchange={(e) => change('beforeOff', e)}>
				{#each offChoices as m (m)}
					<option value={m}>за {m < 60 ? `${m} хв` : `${m / 60} год`}</option>
				{/each}
			</select>
		</label>
		<label>
			<span>Про увімкнення</span>
			<select value={rule.beforeOn} disabled={follows.busy} onchange={(e) => change('beforeOn', e)}>
				{#each onChoices as [m, label] (m)}
					<option value={m}>{label}</option>
				{/each}
			</select>
		</label>
		<p class="hint">За графіком: аварійні відключення й зміни, яких немає в графіку, сповіщення не врахують.</p>
	{:else}
		<button class="enable" disabled={follows.busy || follows.support !== 'supported'} onclick={enable}>
			🔔 Попереджати про відключення
		</button>
		{#if follows.support === 'ios-install'}
			<p class="hint">На iPhone: «Поділитися» → «На початковий екран», потім відкрийте сайт з іконки.</p>
		{:else if follows.support === 'denied'}
			<p class="hint">Сповіщення заблоковані в налаштуваннях браузера для цього сайту.</p>
		{:else if follows.support === 'unsupported'}
			<p class="hint">Цей браузер не підтримує сповіщення.</p>
		{/if}
	{/if}
	{#if follows.error}
		<p class="hint err">{follows.error}</p>
	{/if}
</div>

<style>
	.alerts {
		display: grid;
		gap: 8px;
		margin: 10px 0 4px;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 14px;
	}
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		font-size: 0.9rem;
	}
	label {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		font-size: 0.85rem;
	}
	select {
		flex: none;
		max-width: 60%;
		padding: 6px 8px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
	}
	.enable {
		justify-self: start;
		padding: 8px 14px;
		border: 1px solid var(--accent);
		border-radius: 999px;
		background: var(--surface);
		color: var(--accent);
		font: inherit;
		font-size: 0.9rem;
		cursor: pointer;
	}
	.enable:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.link {
		padding: 0;
		border: 0;
		background: transparent;
		color: var(--muted);
		font: inherit;
		font-size: 0.8rem;
		text-decoration: underline;
		cursor: pointer;
	}
	.hint {
		margin: 0;
		font-size: 0.75rem;
		color: var(--muted);
	}
	.err {
		color: var(--warn-text);
	}
</style>
