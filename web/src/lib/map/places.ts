// SPDX-License-Identifier: AGPL-3.0-or-later
// Where alerts are: raions on the map and place names for people.

import { raions, type MapRaion } from './regions.ts';
import type { Alert, RegionState } from '../types.ts';

const raionById = new Map(raions.map((r) => [r.id, r]));

export function raionOf(a: Alert): MapRaion | undefined {
	return a.raionUid ? raionById.get(a.raionUid) : undefined;
}

/** Raions under alert: full when the raion itself has an alert, partial when only hromadas in it do. */
export function raionStatuses(states: Iterable<RegionState>): Map<string, 'full' | 'partial'> {
	const out = new Map<string, 'full' | 'partial'>();
	for (const s of states) {
		if (s.status !== 'partial') continue;
		for (const a of s.alerts) {
			const r = raionOf(a);
			if (!r) continue;
			if (a.locationType === 'raion') out.set(r.id, 'full');
			else if (!out.has(r.id)) out.set(r.id, 'partial');
		}
	}
	return out;
}

/** Whether every alert of a partial oblast is on a known raion, so the map can show where. */
export function placedOnRaions(s: RegionState): boolean {
	return s.status === 'partial' && s.alerts.every((a) => raionOf(a));
}

/** A place name for people: a hromada gets its raion, «Ніжинська громада (Ніжинський район)». */
export function placeName(a: Alert): string {
	const r = raionOf(a);
	if (a.locationType === 'hromada' && r && r.name !== a.locationTitle) return `${a.locationTitle} (${r.name})`;
	return a.locationTitle;
}

/** Distinct places of a partial oblast's alerts. */
export function placeNames(s: RegionState): string[] {
	return [...new Set(s.alerts.map(placeName).filter(Boolean))];
}
