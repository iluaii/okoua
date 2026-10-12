// SPDX-License-Identifier: AGPL-3.0-or-later

// Following regions: Web Push subscription plus the list of followed regions.
// The list is also kept in localStorage so the UI knows it without asking the server.

const STORAGE_KEY = 'follow.regions';

export type PushSupport =
	| 'supported'
	| 'denied' // the user blocked notifications for this site
	| 'ios-install' // iPhone/iPad: push works only for a site added to the home screen
	| 'unsupported';

function detectSupport(): PushSupport {
	const capable = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;
	if (capable) return Notification.permission === 'denied' ? 'denied' : 'supported';
	const ios = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
	return ios ? 'ios-install' : 'unsupported';
}

function base64urlToBytes(s: string): Uint8Array<ArrayBuffer> {
	const b64 = s.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(s.length / 4) * 4, '=');
	const bin = atob(b64);
	const out = new Uint8Array(new ArrayBuffer(bin.length));
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

class Follows {
	regions = $state<number[]>([]);
	support = $state<PushSupport>('unsupported');
	busy = $state(false);
	error = $state<string | null>(null);

	/** Reads the saved list and re-syncs with the server (subscriptions can expire). */
	init() {
		this.support = detectSupport();
		try {
			const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]');
			if (Array.isArray(saved)) this.regions = saved.filter((n) => Number.isInteger(n));
		} catch {
			this.regions = [];
		}
		if (this.regions.length > 0 && this.support === 'supported' && Notification.permission === 'granted') {
			this.#sync(this.regions).catch(() => {});
		}
	}

	isFollowing(uid: number): boolean {
		return this.regions.includes(uid);
	}

	async toggle(uid: number) {
		const next = this.isFollowing(uid) ? this.regions.filter((r) => r !== uid) : [...this.regions, uid];
		await this.#update(next);
	}

	async #update(next: number[]) {
		if (this.busy) return;
		this.busy = true;
		this.error = null;
		try {
			if (next.length > 0 && Notification.permission !== 'granted') {
				const p = await Notification.requestPermission();
				if (p !== 'granted') {
					this.support = p === 'denied' ? 'denied' : this.support;
					this.error = 'Без дозволу на сповіщення стежити не вийде.';
					return;
				}
			}
			await this.#sync(next);
			this.regions = next;
			try {
				localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
			} catch {
				// The server still has the list; only this browser's copy is lost.
			}
		} catch {
			this.error = 'Не вдалося змінити підписку. Спробуйте ще раз.';
		} finally {
			this.busy = false;
		}
	}

	/** Makes the server's list for this browser's subscription equal to `regions`. */
	async #sync(regions: number[]) {
		const reg = await navigator.serviceWorker.ready;
		let sub = await reg.pushManager.getSubscription();

		if (regions.length === 0) {
			if (sub) {
				await post('/api/push/unsubscribe', { endpoint: sub.endpoint });
				await sub.unsubscribe();
			}
			return;
		}
		if (!sub) {
			const res = await fetch('/api/push/key');
			if (!res.ok) throw new Error('no key');
			const { key } = (await res.json()) as { key: string };
			sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: base64urlToBytes(key) });
		}
		await post('/api/push/subscribe', { subscription: sub.toJSON(), regions });
	}
}

async function post(url: string, body: unknown) {
	const res = await fetch(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!res.ok) throw new Error(`${url}: ${res.status}`);
}

/** One instance for the whole app; the root layout calls init(). */
export const follows = new Follows();
