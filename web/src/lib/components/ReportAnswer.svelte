<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!--
	«Поскаржитися» under one AI answer: a reason in one tap, an optional comment.
	Reason IDs must match server/internal/ai/report.go. When enough people report an answer,
	the server hides it and onhidden is called.
-->
<script lang="ts">
	interface Props {
		newsId: string;
		/** 'summary' or the follow-up question as asked */
		question: string;
		reported: boolean;
		onreported: () => void;
		onhidden: () => void;
	}
	let { newsId, question, reported, onreported, onhidden }: Props = $props();

	const MAX_MESSAGE = 300;
	const reasons: [string, string][] = [
		['wrong_fact', 'Неправильний факт'],
		['wrong_region', 'Не та область'],
		['outdated', 'Застаріло'],
		['other', 'Інше']
	];

	let open = $state(false);
	let reason = $state<string | null>(null);
	let message = $state('');
	let status = $state<'idle' | 'sending' | 'error' | 'limited'>('idle');

	async function send(e: SubmitEvent) {
		e.preventDefault();
		if (!reason || status === 'sending') return;
		status = 'sending';
		try {
			const res = await fetch('/api/ai/report', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ newsId, question, reason, message: message.trim() })
			});
			if (!res.ok) {
				status = res.status === 429 ? 'limited' : 'error';
				return;
			}
			const body = (await res.json()) as { hidden: boolean };
			status = 'idle';
			open = false;
			onreported();
			if (body.hidden) onhidden();
		} catch {
			status = 'error';
		}
	}
</script>

{#if reported}
	<p class="done">Дякуємо, скаргу надіслано.</p>
{:else if !open}
	<button class="link" onclick={() => (open = true)}>Поскаржитися на відповідь</button>
{:else}
	<form onsubmit={send}>
		<span class="label">Що не так?</span>
		<div class="reasons">
			{#each reasons as [id, label] (id)}
				<button type="button" class:on={reason === id} aria-pressed={reason === id} onclick={() => (reason = id)}>{label}</button>
			{/each}
		</div>
		{#if reason}
			<textarea bind:value={message} maxlength={MAX_MESSAGE} rows="2" placeholder="Коментар (необов'язково)"></textarea>
			<p class="note">Скарга анонімна. Разом із нею надсилається лише ця відповідь і запитання.</p>
		{/if}
		{#if status === 'error'}
			<p class="err">Не вдалося надіслати. Спробуйте ще раз.</p>
		{:else if status === 'limited'}
			<p class="err">Забагато скарг. Спробуйте за кілька хвилин.</p>
		{/if}
		<div class="actions">
			<button type="button" class="link" onclick={() => (open = false)}>Скасувати</button>
			<button type="submit" class="send" disabled={!reason || status === 'sending'}>
				{status === 'sending' ? 'Надсилання…' : 'Надіслати'}
			</button>
		</div>
	</form>
{/if}

<style>
	.link {
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
	.done {
		margin: 0;
		font-size: 0.75rem;
		color: var(--muted);
	}
	form {
		display: grid;
		gap: 8px;
		margin-top: 6px;
		padding-top: 8px;
		border-top: 1px solid var(--border);
	}
	.label {
		font-size: 0.8rem;
		font-weight: 600;
	}
	.reasons {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.reasons button {
		padding: 5px 10px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.8rem;
		cursor: pointer;
	}
	.reasons button.on {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 12%, var(--surface));
		color: var(--accent);
	}
	textarea {
		width: 100%;
		box-sizing: border-box;
		padding: 8px 10px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.85rem;
		resize: vertical;
	}
	.note {
		margin: 0;
		font-size: 0.7rem;
		color: var(--muted);
	}
	.err {
		margin: 0;
		font-size: 0.8rem;
		color: var(--warn-text);
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		gap: 14px;
	}
	.send {
		padding: 6px 14px;
		border: 0;
		border-radius: 999px;
		background: var(--accent);
		color: #fff;
		font: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}
	.send:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
