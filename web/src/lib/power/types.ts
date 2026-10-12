// SPDX-License-Identifier: AGPL-3.0-or-later
// Power outage schedules, as served by GET /api/power/{uid} (see server/internal/power).

/** An outage: minutes since midnight Kyiv time, [from, to). */
export interface Outage {
	from: number;
	to: number;
	/** A possible outage rather than a definite one. */
	possible?: boolean;
}

export interface PowerDay {
	/** YYYY-MM-DD, Kyiv time. */
	date: string;
	/** schedule: a schedule applies; none: no outages; waiting: not published yet; unknown: no usable data. */
	status: 'schedule' | 'none' | 'waiting' | 'unknown';
	/** Outages per queue («3.1» → intervals). */
	queues: Record<string, Outage[]>;
}

export interface Link {
	name: string;
	url?: string;
}

export interface PowerSchedule {
	region: number;
	queues: string[];
	today: PowerDay;
	tomorrow: PowerDay;
	/** When the source last published changes (may be empty). */
	updatedAt: string;
	/** Our last successful fetch. */
	checkedAt: string;
	/** Data may be outdated. */
	stale: boolean;
	/** The grid operator that publishes the schedule. */
	source: Link;
	/** A community dataset the schedule is read through, if any. */
	via?: Link;
	/** Where people can look up their queue (by address or account number). */
	lookupUrl?: string;
}

export type PowerLoad =
	| { kind: 'loading' }
	| { kind: 'ok'; schedule: PowerSchedule }
	| { kind: 'unsupported' }
	| { kind: 'error' };

export async function loadPower(region: number, signal?: AbortSignal): Promise<PowerLoad> {
	try {
		const res = await fetch(`/api/power/${region}`, { signal });
		if (res.status === 404) return { kind: 'unsupported' };
		if (!res.ok) return { kind: 'error' };
		return { kind: 'ok', schedule: (await res.json()) as PowerSchedule };
	} catch {
		return { kind: 'error' };
	}
}
