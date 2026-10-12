// SPDX-License-Identifier: AGPL-3.0-or-later
// Power outage schedules. The server will serve the same shape (stage 2).

/** An outage: minutes since midnight Kyiv time, [from, to). */
export interface Outage {
	from: number;
	to: number;
}

export interface PowerDay {
	/** YYYY-MM-DD, Kyiv time. */
	date: string;
	/** schedule: a schedule applies; none: no outages; waiting: not published yet. */
	status: 'schedule' | 'none' | 'waiting';
	/** Outages per queue («3.1» → intervals). */
	queues: Record<string, Outage[]>;
}

export interface PowerSchedule {
	region: number;
	queues: string[];
	today: PowerDay;
	tomorrow: PowerDay;
	updatedAt: string;
	source: { name: string; url?: string };
	/** Where people can look up their queue (by address or account number). */
	lookupUrl?: string;
	/** Demo data until the server serves real data. */
	demo?: boolean;
}
