// SPDX-License-Identifier: AGPL-3.0-or-later

const alertTypes: Record<string, string> = {
	air_raid: 'Повітряна тривога',
	artillery_shelling: 'Загроза артобстрілу',
	urban_fights: 'Загроза вуличних боїв',
	chemical: 'Хімічна загроза',
	nuclear: 'Радіаційна загроза'
};

export function alertTypeLabel(type: string): string {
	return alertTypes[type] ?? 'Тривога';
}

const tz = 'Europe/Kyiv';
const timeFmt = new Intl.DateTimeFormat('uk-UA', { hour: '2-digit', minute: '2-digit', timeZone: tz });
const dateFmt = new Intl.DateTimeFormat('uk-UA', { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: tz });

const DAY = 24 * 60 * 60 * 1000;

/** «з 14:05» for recent alerts, «з 04.04.2022» for ones lasting over a day. */
export function formatStart(iso: string, now: number): string {
	const t = new Date(iso);
	return now - t.getTime() < DAY ? `з ${timeFmt.format(t)}` : `з ${dateFmt.format(t)}`;
}

/** «15 хв», «1 год 20 хв», «3 дн 4 год» (Ukrainian UI strings). */
export function formatDuration(iso: string, now: number): string {
	const minutes = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 60000));
	if (minutes < 1) return 'щойно';
	const d = Math.floor(minutes / 1440);
	const h = Math.floor((minutes % 1440) / 60);
	const m = minutes % 60;
	if (d > 0) return h > 0 ? `${d} дн ${h} год` : `${d} дн`;
	if (h > 0) return m > 0 ? `${h} год ${m} хв` : `${h} год`;
	return `${m} хв`;
}

/** The region's earliest alert: duration is counted from it. */
export function earliestStart(alerts: { startedAt: string }[]): string {
	return alerts.reduce((min, a) => (a.startedAt < min ? a.startedAt : min), alerts[0].startedAt);
}
