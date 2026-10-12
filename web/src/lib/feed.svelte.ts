// SPDX-License-Identifier: AGPL-3.0-or-later

import type { AlertsState } from './types.ts';

export type Connection = 'connecting' | 'live' | 'offline';

export interface FeedStatus {
	kind: 'ok' | 'warn' | 'muted';
	/** Short text for the header. */
	short: string;
	/** Explanation when something is wrong. */
	detail?: string;
}

/**
 * Live alert stream from the server (SSE). The server sends a full snapshot
 * on connect and after every change, so after a dropped connection
 * reconnecting is enough.
 */
export class AlertsFeed {
	state = $state<AlertsState | null>(null);
	connection = $state<Connection>('connecting');

	get status(): FeedStatus {
		if (this.connection !== 'live') {
			return this.state
				? { kind: 'warn', short: "Немає зв'язку", detail: "Немає зв'язку з сервером, перепідключення… Показано останні відомі дані." }
				: { kind: 'muted', short: 'Підключення…' };
		}
		if (this.state?.stale) {
			return { kind: 'warn', short: 'Дані застаріли', detail: 'Джерело тривог не відповідає. Дані можуть бути застарілими.' };
		}
		return { kind: 'ok', short: 'Наживо' };
	}

	#es: EventSource | null = null;
	#retry: ReturnType<typeof setTimeout> | undefined;
	#delay = 2000;

	/** Connects and returns a function that disconnects. */
	start(): () => void {
		this.#connect();
		const onVisible = () => {
			// A phone may have put the connection to sleep in the background: reconnect right away when the tab is visible again.
			if (document.visibilityState === 'visible' && this.connection !== 'live') this.#reconnect(0);
		};
		document.addEventListener('visibilitychange', onVisible);
		window.addEventListener('online', onVisible);
		return () => {
			document.removeEventListener('visibilitychange', onVisible);
			window.removeEventListener('online', onVisible);
			clearTimeout(this.#retry);
			this.#es?.close();
			this.#es = null;
		};
	}

	#connect() {
		this.#es?.close();
		const es = new EventSource('/api/stream');
		this.#es = es;
		es.onmessage = (e) => {
			try {
				this.state = JSON.parse(e.data) as AlertsState;
				this.connection = 'live';
				this.#delay = 2000;
			} catch {
				// Skip a malformed message; the next one carries a full snapshot anyway.
			}
		};
		es.onerror = () => {
			this.connection = 'offline';
			// On a network error EventSource reconnects by itself, but after
			// an error response (e.g. a 502 from a proxy) it closes for good.
			if (es.readyState === EventSource.CLOSED) this.#reconnect(this.#delay);
		};
	}

	#reconnect(delay: number) {
		clearTimeout(this.#retry);
		this.#retry = setTimeout(() => this.#connect(), delay);
		this.#delay = Math.min(this.#delay * 2, 30000);
	}
}

/** One stream for the whole app; the root layout starts it. */
export const feed = new AlertsFeed();
