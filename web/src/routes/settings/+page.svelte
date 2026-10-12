<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { follows } from '#lib/push.svelte.ts';
	import { install } from '#lib/install.svelte.ts';
	import { regions } from '#lib/map/regions.ts';

	const names = new Map(regions.map((r) => [r.uid, r.name]));
</script>

<svelte:head>
	<title>Налаштування</title>
</svelte:head>

<div class="page">
	<h1>Налаштування</h1>

	<h2>Сповіщення</h2>
	{#if follows.regions.length === 0}
		<p class="muted">Ви ще не стежите за жодною областю. Натисніть на область на карті й оберіть «Стежити».</p>
	{:else}
		<ul class="follows">
			{#each follows.regions as uid (uid)}
				<li>
					<span>{names.get(uid) ?? uid}</span>
					<button disabled={follows.busy} onclick={() => follows.toggle(uid)}>Не стежити</button>
				</li>
			{/each}
		</ul>
	{/if}
	{#if follows.error}
		<p class="err">{follows.error}</p>
	{/if}
	<p class="muted small">
		Сповіщення в браузері можуть запізнюватися і не пробивають беззвучний режим. Вони доповнюють офіційний
		застосунок «Повітряна тривога», а не замінюють його.
	</p>

	<h2>Застосунок</h2>
	{#if install.installed}
		<p class="muted">Сайт встановлено як застосунок.</p>
	{:else if install.available}
		<button class="primary" onclick={() => install.prompt()}>Встановити на пристрій</button>
	{:else if follows.support === 'ios-install'}
		<p class="muted">На iPhone: «Поділитися» → «На початковий екран». Сповіщення на iPhone працюють тільки так.</p>
	{:else}
		<p class="muted">Сайт можна встановити як застосунок через меню браузера («Встановити» або «Додати на головний екран»).</p>
	{/if}

	<h2>Про сервіс</h2>
	<p>
		Сервіс доповнює офіційні джерела тривог, а не замінює їх. Сповіщення в браузері можуть
		запізнюватися, тому завжди реагуйте на сирени та офіційний застосунок «Повітряна тривога».
	</p>

	<h2>Приватність</h2>
	<p>
		Сайт не просить реєстрації і не збирає даних про вас. Відгуки анонімні: зберігаються лише категорія, текст,
		сторінка й версія сайту, без IP-адреси. Відгуки видаляються через 12 місяців. Обрана черга світла зберігається
		тільки у вашому браузері, як і розмови з ШІ. Для сповіщень сервер зберігає анонімну адресу сповіщень, яку видає браузер, і
		список обраних областей; вони видаляються, щойно ви перестаєте стежити.
	</p>

	<h2>Джерела даних</h2>
	<ul>
		<li>Тривоги: <a href="https://alerts.in.ua" rel="noopener">alerts.in.ua</a></li>
		<li>
			Межі областей: © учасники <a href="https://www.openstreetmap.org/copyright" rel="noopener">OpenStreetMap</a>
			(ODbL), через <a href="https://www.geoboundaries.org" rel="noopener">geoBoundaries</a>
		</li>
	</ul>

	<h2>Ліцензія</h2>
	<p>Відкритий код під ліцензією AGPL-3.0.</p>
</div>

<style>
	h2 {
		font-size: 1.05rem;
		margin: 20px 0 6px;
	}
	.small {
		font-size: 0.85rem;
	}
	.err {
		color: var(--alert-text);
	}
	.follows {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 6px;
	}
	.follows li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--surface);
	}
	.follows button,
	.primary {
		padding: 7px 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}
	.primary {
		border-color: var(--accent);
		background: var(--accent);
		color: #fff;
		font-weight: 600;
	}
	.muted {
		color: var(--muted);
	}
</style>
