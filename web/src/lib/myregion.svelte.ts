// SPDX-License-Identifier: AGPL-3.0-or-later

// «Моя область»: one region the quick answers are about. Kept in this browser only.
// Until the user picks one, the first followed region (if any) is used.

import { follows } from './push.svelte.ts';
import { regions } from './map/regions.ts';

const KEY = 'my.region';

class MyRegion {
	#chosen = $state<number | null>(null);

	init() {
		try {
			const v = Number(localStorage.getItem(KEY));
			if (regions.some((r) => r.uid === v)) this.#chosen = v;
		} catch {
			// Storage unavailable: fall back to the followed region.
		}
	}

	/** The chosen region, else the first followed one, else null. */
	get uid(): number | null {
		return this.#chosen ?? follows.regions[0] ?? null;
	}

	set(uid: number | null) {
		this.#chosen = uid;
		try {
			if (uid === null) localStorage.removeItem(KEY);
			else localStorage.setItem(KEY, String(uid));
		} catch {
			// Not saved; works for this visit.
		}
	}
}

export const myRegion = new MyRegion();

/** Regions sorted by name, for pickers. */
export const regionsByName = regions.toSorted((a, b) => a.name.localeCompare(b.name, 'uk'));
