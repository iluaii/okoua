<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<svelte:head>
	<title>Налаштування</title>
</svelte:head>

<div class="page">
	<h1>Налаштування</h1>
	<p class="muted">Вибір області та сповіщення з'являться незабаром.</p>

	<h2>Про сервіс</h2>
	<p>
		Сервіс доповнює офіційні джерела тривог, а не замінює їх. Сповіщення в браузері можуть
		запізнюватися, тому завжди реагуйте на сирени та офіційний застосунок «Повітряна тривога».
	</p>

	<h2>Приватність</h2>
	<p>
		Сайт не просить реєстрації і не збирає даних про вас. Відгуки анонімні: зберігаються лише категорія, текст,
		сторінка й версія сайту, без IP-адреси. Відгуки видаляються через 12 місяців. Обрана черга світла зберігається
		тільки у вашому браузері.
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
	.muted {
		color: var(--muted);
	}
</style>
