// SPDX-License-Identifier: AGPL-3.0-or-later

// Project info shown in «Налаштування → Про проєкт». Fill in the links before going public;
// empty values are simply not shown. Texts are Ukrainian because they are shown to users.

export interface SupportLink {
	/** Button text, e.g. «Банка Monobank». */
	label: string;
	url: string;
	/** Optional short note under the button. */
	note?: string;
}

export const project = {
	/** Public source code, once the repository is published. */
	repoUrl: 'https://github.com/iluaii/okoua',

	/** The server runs on the author's laptop at home: show a notice about downtime during outages. */
	homeHosted: true,

	/**
	 * Where people can support the project. Example:
	 * { label: 'Банка Monobank', url: 'https://send.monobank.ua/jar/…', note: 'Сервер та ШІ' }
	 */
	support: [{ label: 'Банка Monobank', url: 'https://send.monobank.ua/jar/AA9NNKgDfe', note: 'На сервер' }] as SupportLink[],

	/** What the money pays for; shown so supporters know where it goes. */
	costs: ['Сервер і домен', 'Запити до ШІ для переказів новин', 'Час на підтримку та нові області'],

	/** Optional public report of income and spending (a page, a table, a post). */
	reportUrl: '',

	/**
	 * Thanks wall: supporters who agreed to be named. Consent is the name they wrote in the jar
	 * comment (the page asks for it); never use the name from the bank statement. People who
	 * left no comment stay anonymous and are simply not listed. Example:
	 * { name: 'Олена з Ніжина' } or { name: 'Компанія X', note: 'сервер на рік' }
	 */
	thanks: [{ name: 'chesser' }] as { name: string; note?: string }[]
};
