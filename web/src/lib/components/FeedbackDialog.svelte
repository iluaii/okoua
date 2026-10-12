<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Anonymous feedback form in a modal dialog. Category IDs must match server/internal/feedback. -->
<script lang="ts">
	import { page } from '$app/state';
	import { version } from '$app/env';

	const MAX_MESSAGE = 1000;

	const categories: { id: string; label: string; subs: [string, string][] }[] = [
		{
			id: 'alerts',
			label: 'Тривоги',
			subs: [
				['missing', 'Тривогу не показано'],
				['false', 'Показано тривогу, якої немає'],
				['time', 'Неправильний час або тривалість'],
				['region', 'Не та область чи громада'],
				['delay', "Тривога з'являється із запізненням"],
				['other', 'Інше']
			]
		},
		{
			id: 'power',
			label: 'Графіки світла',
			subs: [
				['mismatch', 'Графік не збігається з офіційним'],
				['outdated', 'Застарілий графік'],
				['no_region', 'Немає моєї області'],
				['no_queue', 'Немає моєї черги'],
				['other', 'Інше']
			]
		},
		{
			id: 'ai',
			label: 'ШІ',
			subs: [
				['hallucination', 'Вигадані факти'],
				['russian', 'Відповідь російською мовою'],
				['wrong_region', 'Не та область'],
				['outdated', 'Застаріла інформація'],
				['other', 'Інше']
			]
		},
		{
			id: 'ui',
			label: 'Карта й інтерфейс',
			subs: [
				['broken', 'Щось не працює'],
				['inconvenient', 'Незручно користуватися'],
				['slow', 'Повільно працює'],
				['other', 'Інше']
			]
		},
		{
			id: 'other',
			label: 'Інше',
			subs: [
				['idea', 'Пропозиція або ідея'],
				['other', 'Інше']
			]
		}
	];

	let dialog = $state<HTMLDialogElement>();
	let category = $state<string | null>(null);
	let subcategory = $state<string | null>(null);
	let message = $state('');
	let website = $state(''); // honeypot, hidden from people
	let status = $state<'idle' | 'sending' | 'sent' | 'error' | 'limited'>('idle');

	const subs = $derived(categories.find((c) => c.id === category)?.subs ?? []);

	export function open(presetCategory?: string, presetSubcategory?: string) {
		const preset = categories.find((c) => c.id === presetCategory);
		category = preset?.id ?? null;
		subcategory = preset?.subs.some(([id]) => id === presetSubcategory) ? (presetSubcategory ?? null) : null;
		message = '';
		website = '';
		status = 'idle';
		dialog?.showModal();
	}

	function close() {
		dialog?.close();
	}

	function pickCategory(id: string) {
		if (category !== id) subcategory = null;
		category = id;
	}

	async function send(e: SubmitEvent) {
		e.preventDefault();
		if (!category || !subcategory || status === 'sending') return;
		status = 'sending';
		try {
			const res = await fetch('/api/feedback', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					category,
					subcategory,
					message: message.trim(),
					page: page.url.pathname,
					version,
					website
				})
			});
			status = res.ok ? 'sent' : res.status === 429 ? 'limited' : 'error';
		} catch {
			status = 'error';
		}
	}

	// Close when clicking the backdrop (the dialog element itself, outside its content).
	function onclick(e: MouseEvent) {
		if (e.target === dialog) close();
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog bind:this={dialog} aria-labelledby="feedback-title" {onclick}>
	<div class="content">
		<header>
			<h2 id="feedback-title">Відгук</h2>
			<button class="icon" onclick={close} aria-label="Закрити">✕</button>
		</header>

		{#if status === 'sent'}
			<p class="done">Дякуємо, надіслано!</p>
			<button class="primary" onclick={close}>Закрити</button>
		{:else}
			<form onsubmit={send}>
				<fieldset>
					<legend>Що саме?</legend>
					<div class="chips">
						{#each categories as c (c.id)}
							<button type="button" class:active={category === c.id} aria-pressed={category === c.id} onclick={() => pickCategory(c.id)}>
								{c.label}
							</button>
						{/each}
					</div>
				</fieldset>

				{#if category}
					<fieldset>
						<legend>Проблема</legend>
						<div class="chips">
							{#each subs as [id, label] (id)}
								<button type="button" class:active={subcategory === id} aria-pressed={subcategory === id} onclick={() => (subcategory = id)}>
									{label}
								</button>
							{/each}
						</div>
					</fieldset>
				{/if}

				<label class="message">
					<span>Опишіть проблему <span class="optional">(необов'язково, але дуже допоможе)</span></span>
					<textarea bind:value={message} maxlength={MAX_MESSAGE} rows="4" placeholder="Що сталося, де і коли?"></textarea>
					<span class="counter">{message.length}/{MAX_MESSAGE}</span>
				</label>

				<!-- Honeypot: invisible to people, bots tend to fill every field. -->
				<input class="trap" type="text" name="website" bind:value={website} tabindex="-1" autocomplete="off" aria-hidden="true" />

				{#if status === 'error'}
					<p class="err">Не вдалося надіслати. Перевірте зв'язок і спробуйте ще раз.</p>
				{:else if status === 'limited'}
					<p class="err">Забагато відгуків за короткий час. Спробуйте пізніше.</p>
				{/if}

				{#if category && subcategory}
					<button class="primary" type="submit" disabled={status === 'sending'}>
						{status === 'sending' ? 'Надсилаємо…' : 'Надіслати'}
					</button>
				{/if}

				<p class="privacy">
					Відгук анонімний: ми не зберігаємо вашу IP-адресу чи інші дані про вас. Разом із текстом надсилаються лише
					сторінка й версія сайту.
				</p>
			</form>
		{/if}
	</div>
</dialog>

<style>
	dialog {
		width: min(calc(100% - 2 * var(--gutter)), 560px);
		max-height: calc(100dvh - 32px);
		padding: 0;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--surface);
		color: var(--text);
		box-shadow: var(--shadow);
	}
	dialog::backdrop {
		background: rgb(0 0 0 / 0.4);
	}
	.content {
		padding: 14px 18px 18px;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 4px;
	}
	h2 {
		margin: 0;
		font-size: 1.15rem;
	}
	.icon {
		width: 34px;
		height: 34px;
		margin-right: -6px;
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
	fieldset {
		margin: 12px 0 0;
		padding: 0;
		border: 0;
	}
	legend {
		margin-bottom: 8px;
		font-weight: 600;
		font-size: 0.9rem;
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.chips button {
		padding: 7px 12px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.9rem;
		cursor: pointer;
	}
	.chips button.active {
		background: var(--accent);
		border-color: var(--accent);
		color: #fff;
	}
	.message {
		display: grid;
		gap: 6px;
		margin-top: 16px;
		font-weight: 600;
		font-size: 0.9rem;
	}
	.optional {
		font-weight: 400;
		color: var(--muted);
	}
	textarea {
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--bg);
		color: var(--text);
		font: inherit;
		font-weight: 400;
		resize: vertical;
	}
	textarea:focus {
		outline: 2px solid var(--focus);
		outline-offset: -1px;
	}
	.counter {
		justify-self: end;
		font-weight: 400;
		font-size: 0.75rem;
		color: var(--muted);
	}
	.trap {
		position: absolute;
		left: -10000px;
		width: 1px;
		height: 1px;
		opacity: 0;
	}
	.primary {
		width: 100%;
		margin-top: 14px;
		padding: 11px;
		border: 0;
		border-radius: 12px;
		background: var(--accent);
		color: #fff;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.primary:disabled {
		opacity: 0.6;
	}
	.err {
		margin: 12px 0 0;
		color: var(--alert-text);
		font-size: 0.9rem;
	}
	.done {
		margin: 16px 0 4px;
		font-size: 1.05rem;
	}
	.privacy {
		margin: 14px 0 0;
		font-size: 0.75rem;
		color: var(--muted);
	}
</style>
