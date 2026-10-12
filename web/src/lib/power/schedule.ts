// SPDX-License-Identifier: AGPL-3.0-or-later

import type { Outage } from './types.ts';

const kyivParts = new Intl.DateTimeFormat('en-GB', {
	timeZone: 'Europe/Kyiv',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
	hourCycle: 'h23'
});

/** Date (YYYY-MM-DD) and minutes since midnight, Kyiv time. */
export function kyivNow(now: number): { date: string; minutes: number } {
	const p = Object.fromEntries(kyivParts.formatToParts(new Date(now)).map((x) => [x.type, x.value]));
	return { date: `${p.year}-${p.month}-${p.day}`, minutes: Number(p.hour) * 60 + Number(p.minute) };
}

/** 930 → «15:30», 1440 → «00:00». */
export function hhmm(minutes: number): string {
	const m = minutes % 1440;
	return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
}

export type PowerNow =
	| { on: false; until: number }
	| { on: true; nextOff: number | null; nextOffTo: number | null };

/** Whether there is power at `minutes` and what comes next (within the day only). */
export function powerAt(outages: Outage[], minutes: number): PowerNow {
	const sorted = outages.toSorted((a, b) => a.from - b.from);
	const current = sorted.find((o) => o.from <= minutes && minutes < o.to);
	if (current) {
		// Merge back-to-back outages: «немає до» must show the real end.
		let until = current.to;
		for (const o of sorted) if (o.from <= until && o.to > until) until = o.to;
		return { on: false, until };
	}
	const next = sorted.find((o) => o.from > minutes);
	return { on: true, nextOff: next?.from ?? null, nextOffTo: next?.to ?? null };
}

/** Hours without power in a day. */
export function hoursOff(outages: Outage[]): number {
	return outages.reduce((sum, o) => sum + (o.to - o.from), 0) / 60;
}
