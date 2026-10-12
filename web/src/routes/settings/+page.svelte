<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { follows } from '#lib/push.svelte.ts';
	import { install } from '#lib/install.svelte.ts';
	import { regions } from '#lib/map/regions.ts';
	import { project } from '#lib/project.ts';
	import { myRegion, regionsByName } from '#lib/myregion.svelte.ts';

	const names = new Map(regions.map((r) => [r.uid, r.name]));
</script>

<svelte:head>
	<title>Налаштування</title>
</svelte:head>

<div class="page">
	<h1>Налаштування</h1>

	<section class="about" aria-labelledby="about-title">
		<h2 id="about-title">Про проєкт</h2>
		<p>
			Концепт збирає в одному місці те, що зазвичай розкидано по десятках каналів і сайтів: карту повітряних тривог,
			графіки відключень світла та новини вашої області з коротким переказом від ШІ.
		</p>
		<p>
			Це некомерційний проєкт з відкритим кодом. Тут немає реклами, ми не збираємо персональних даних і нічого не продаємо.
			Сервіс доповнює офіційні джерела, а не замінює їх: завжди реагуйте на сирени та офіційний застосунок
			«Повітряна тривога».
		</p>
		{#if project.repoUrl}
			<p><a href={project.repoUrl} target="_blank" rel="noopener">Відкритий код проєкту →</a></p>
		{/if}

		<div class="support">
			<h3>Підтримати проєкт</h3>
			<p class="muted small">
				Сервіс безкоштовний для всіх, і підтримка не дає жодних переваг: усі бачать те саме. Внески покривають:
			</p>
			<ul class="costs">
				{#each project.costs as c (c)}
					<li>{c}</li>
				{/each}
			</ul>
			{#if project.support.length > 0}
				<div class="links">
					{#each project.support as link (link.url)}
						<a class="support-link" href={link.url} target="_blank" rel="noopener">
							<span>{link.label}</span>
							{#if link.note}<small>{link.note}</small>{/if}
						</a>
					{/each}
				</div>
			{:else}
				<p class="muted small">Можливість підтримати з'явиться найближчим часом.</p>
			{/if}
			{#if project.reportUrl}
				<p class="small"><a href={project.reportUrl} target="_blank" rel="noopener">Звіт: скільки зібрано і на що витрачено →</a></p>
			{/if}
		</div>

		{#if project.thanks.length > 0}
			<div class="thanks">
				<h3>Дякуємо</h3>
				<p class="muted small">Тим, хто підтримав проєкт і погодився, щоб ми назвали їхнє ім'я.</p>
				<ul>
					{#each project.thanks as t, i (i)}
						<li>{t.name}{#if t.note}&nbsp;<span class="muted">· {t.note}</span>{/if}</li>
					{/each}
				</ul>
				<p class="muted small">І всім, хто підтримав анонімно.</p>
			</div>
		{/if}
	</section>

	<h2>Моя область</h2>
	<select
		class="region-select"
		value={myRegion.uid ?? ''}
		onchange={(e) => myRegion.set(Number(e.currentTarget.value) || null)}
		aria-label="Моя область"
	>
		<option value="" disabled>Оберіть область</option>
		{#each regionsByName as r (r.uid)}
			<option value={r.uid}>{r.name}</option>
		{/each}
	</select>
	<p class="muted small">Про неї відповідають швидкі запитання на вкладці «ШІ». Зберігається лише в цьому браузері.</p>

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
	.about {
		padding: 4px 16px 16px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--surface);
	}
	.about p {
		margin: 8px 0;
	}
	.support {
		margin-top: 14px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.thanks {
		margin-top: 14px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.thanks ul {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin: 8px 0;
		padding: 0;
		list-style: none;
	}
	.thanks li {
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--surface-2);
		font-size: 0.85rem;
	}
	h3 {
		margin: 0 0 4px;
		font-size: 1rem;
	}
	.costs {
		margin: 6px 0 10px;
		padding-left: 20px;
		font-size: 0.9rem;
	}
	.links {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 8px;
		margin: 10px 0;
	}
	.support-link {
		display: grid;
		gap: 2px;
		padding: 10px 14px;
		border-radius: 12px;
		background: var(--accent);
		color: #fff;
		font-weight: 600;
		text-decoration: none;
	}
	.support-link small {
		font-weight: 400;
		opacity: 0.85;
	}
	.region-select {
		width: 100%;
		padding: 9px 10px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
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
