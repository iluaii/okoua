// SPDX-License-Identifier: AGPL-3.0-or-later
// Server response types, see server/internal/hub/hub.go.

export interface Alert {
	id: string;
	type: string; // air_raid, artillery_shelling, urban_fights, chemical, nuclear
	oblastUid: number;
	locationUid: string;
	locationTitle: string;
	locationType: string; // oblast, raion, city, hromada, unknown
	/** District the alert is in (its own ID for a raion, the parent raion's for a hromada). */
	raionUid?: string;
	startedAt: string;
}

export interface RegionState {
	uid: number;
	status: 'full' | 'partial';
	alerts: Alert[];
}

export interface AlertsState {
	source: string;
	stale: boolean;
	updatedAt: string;
	regions: RegionState[];
}
