// SPDX-License-Identifier: AGPL-3.0-or-later
// Demo data for the power schedule window until the server serves real data (stage 2).
// Intervals come from the real Chernihivoblenergo schedule for 2026-10-12.

import { kyivNow } from './schedule.ts';
import type { Outage, PowerSchedule } from './types.ts';

const t = (s: string) => {
	const [h, m] = s.split(':').map(Number);
	return h * 60 + m;
};
const o = (from: string, to: string): Outage => ({ from: t(from), to: to === '00:00' ? 1440 : t(to) });

const chernihiv: Record<string, Outage[]> = {
	'1.1': [o('06:00', '08:00'), o('22:00', '00:00')],
	'1.2': [o('22:00', '00:00')],
	'2.1': [o('18:30', '22:00')],
	'2.2': [o('18:30', '22:00')],
	'3.1': [o('15:00', '18:30')],
	'3.2': [o('18:30', '22:00')],
	'4.1': [o('15:00', '18:30')],
	'4.2': [o('15:00', '18:30')],
	'5.1': [o('08:00', '11:30')],
	'5.2': [o('08:00', '11:30')],
	'6.1': [o('11:30', '15:00')],
	'6.2': [o('11:30', '15:00')]
};

/** Regions that will have real data in stage 2. */
const sources: Record<number, { name: string; url?: string; lookupUrl?: string }> = {
	25: { name: 'АТ «Чернігівобленерго»', url: 'https://t.me/chernigivoblenergo', lookupUrl: 'https://chernihivoblenergo.com.ua/blackouts' },
	31: { name: 'YASNO', url: 'https://yasno.ua', lookupUrl: 'https://yasno.ua' },
	9: { name: 'YASNO', url: 'https://yasno.ua', lookupUrl: 'https://yasno.ua' },
	14: { name: 'ДТЕК Київські регіональні електромережі', url: 'https://www.dtek-krem.com.ua/ua/shutdowns' },
	22: { name: 'АТ «Харківобленерго»' },
	19: { name: 'АТ «Полтаваобленерго»' }
};

export function demoSchedule(region: number, now: number): PowerSchedule | null {
	const src = sources[region];
	if (!src) return null;
	const today = kyivNow(now).date;
	const tomorrow = kyivNow(now + 24 * 3600 * 1000).date;
	return {
		region,
		queues: Object.keys(chernihiv),
		today: { date: today, status: 'schedule', queues: chernihiv },
		tomorrow: { date: tomorrow, status: 'waiting', queues: {} },
		updatedAt: new Date(now - 17 * 60 * 1000).toISOString(),
		source: { name: src.name, url: src.url },
		lookupUrl: src.lookupUrl,
		demo: true
	};
}
