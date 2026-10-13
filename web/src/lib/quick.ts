// SPDX-License-Identifier: AGPL-3.0-or-later

// Quick answers about «моя область», built from the service's own data, no AI involved:
// exact, instant and free. Texts are Ukrainian because they are shown to users.

import { regions, neighbours } from './map/regions.ts';
import { placeNames } from './map/places.ts';
import type { AlertsState, RegionState } from './types.ts';
import type { PowerSchedule } from './power/types.ts';
import type { NewsItem } from './news/types.ts';
import { alertTypeLabel, earliestStart, formatDuration, formatMinutes, plural } from './format.ts';
import { hhmm, kyivNow, powerAt } from './power/schedule.ts';

export type QuickId = 'now' | 'neighbours' | 'day';

export const quickQuestions: { id: QuickId; label: string }[] = [
	{ id: 'now', label: 'Що зараз у моїй області' },
	{ id: 'neighbours', label: 'Сусідні області' },
	{ id: 'day', label: 'Тривоги за добу' }
];

export interface Line {
	text: string;
	href?: string;
	tone?: 'alert' | 'ok' | 'muted';
}

export interface DataAnswer {
	lines: Line[];
}

const names = new Map(regions.map((r) => [r.uid, r.name]));
const short = (uid: number) => (names.get(uid) ?? String(uid)).replace(' область', '');

function regionLine(uid: number, state: RegionState | undefined, now: number): Line {
	if (!state) return { text: `🟢 ${short(uid)}: тривоги немає`, tone: 'ok' };
	const kind = alertTypeLabel(state.alerts[0]?.type ?? '').toLowerCase();
	const places = state.status === 'partial' ? placeNames(state) : [];
	const where = state.status !== 'partial' ? '' : places.length ? ` (${places.join(', ')})` : ' (частина області)';
	return { text: `🔴 ${short(uid)}: ${kind}${where}, ${formatDuration(earliestStart(state.alerts), now)}`, tone: 'alert' };
}

export async function answer(id: QuickId, uid: number, alerts: AlertsState | null, now: number): Promise<DataAnswer> {
	const states = new Map((alerts?.regions ?? []).map((r) => [r.uid, r]));
	switch (id) {
		case 'now':
			return regionNow(uid, states, now);
		case 'neighbours':
			return {
				lines: (neighbours[uid] ?? [])
					.map((n) => regionLine(n, states.get(n), now))
					.sort((a, b) => Number(b.tone === 'alert') - Number(a.tone === 'alert'))
			};
		case 'day':
			return day(uid);
	}
}

async function regionNow(uid: number, states: Map<number, RegionState>, now: number): Promise<DataAnswer> {
	const lines: Line[] = [regionLine(uid, states.get(uid), now)];

	// Power: only if the region has schedules and a queue was chosen in the power window.
	let queue: string | null = null;
	try {
		queue = localStorage.getItem(`power.queue.${uid}`);
	} catch {
		queue = null;
	}
	const power = await getJSON<PowerSchedule>(`/api/power/${uid}`);
	if (power && queue && power.today.status === 'schedule' && power.queues.includes(queue)) {
		const outages = (power.today.queues[queue] ?? []).filter((o) => !o.possible);
		const p = powerAt(outages, kyivNow(now).minutes);
		lines.push(
			!p.on
				? { text: `🌑 Світла немає (черга ${queue}), увімкнуть о ${hhmm(p.until)}`, tone: 'alert' }
				: p.nextOff !== null
					? { text: `💡 Світло є (черга ${queue}), відключення о ${hhmm(p.nextOff)}` }
					: { text: `💡 Світло є (черга ${queue}), сьогодні відключень більше немає`, tone: 'ok' }
		);
	} else if (power && power.today.status === 'none') {
		lines.push({ text: '💡 Сьогодні відключень світла не заплановано', tone: 'ok' });
	} else if (power) {
		lines.push({ text: '💡 Оберіть свою чергу у вікні «Світло» області, щоб бачити свій графік', tone: 'muted' });
	}

	const news = await getJSON<{ items: NewsItem[] }>(`/api/news/${uid}`);
	if (news?.items.length) {
		lines.push({ text: 'Останні новини:', tone: 'muted' });
		for (const n of news.items.slice(0, 3)) lines.push({ text: `• ${n.title}`, href: n.url });
	}
	return { lines };
}

async function day(uid: number): Promise<DataAnswer> {
	const stats = await getJSON<{ regions: Record<string, { count: number; totalMinutes: number; longestMinutes: number }> }>(
		'/api/stats/alerts?hours=24'
	);
	if (!stats) return { lines: [{ text: 'Не вдалося отримати статистику. Спробуйте пізніше.', tone: 'muted' }] };
	const mine = stats.regions[uid];
	const lines: Line[] = [];
	if (!mine || mine.count === 0) {
		lines.push({ text: `🟢 ${short(uid)}: за добу тривог не було`, tone: 'ok' });
	} else {
		lines.push({
			text: `${short(uid)}: ${mine.count} ${plural(mine.count, ['тривога', 'тривоги', 'тривог'])}, загалом ${formatMinutes(mine.totalMinutes)}`,
			tone: 'alert'
		});
		if (mine.count > 1) lines.push({ text: `Найдовша: ${formatMinutes(mine.longestMinutes)}` });
	}
	const around = (neighbours[uid] ?? [])
		.map((n) => ({ n, s: stats.regions[n] }))
		.filter((x) => x.s?.count)
		.sort((a, b) => b.s.totalMinutes - a.s.totalMinutes);
	if (around.length > 0) {
		lines.push({ text: 'Сусідні області:', tone: 'muted' });
		for (const { n, s } of around) {
			lines.push({ text: `• ${short(n)}: ${s.count} ${plural(s.count, ['тривога', 'тривоги', 'тривог'])}, ${formatMinutes(s.totalMinutes)}` });
		}
	}
	lines.push({ text: 'Статистика ведеться з моменту запуску сервісу.', tone: 'muted' });
	return { lines };
}

async function getJSON<T>(url: string): Promise<T | null> {
	try {
		const res = await fetch(url);
		return res.ok ? ((await res.json()) as T) : null;
	} catch {
		return null;
	}
}
