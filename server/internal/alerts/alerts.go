// SPDX-License-Identifier: AGPL-3.0-or-later

// Package alerts defines air-raid alerts and the sources the server reads them from.
package alerts

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// Alert is one active alert, normalized so it does not depend on the source.
type Alert struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"` // air_raid, artillery_shelling, urban_fights, chemical, nuclear
	OblastUID     int       `json:"oblastUid"`
	LocationUID   string    `json:"locationUid"`
	LocationTitle string    `json:"locationTitle"`
	LocationType  string    `json:"locationType"` // oblast, raion, city, hromada, unknown
	StartedAt     time.Time `json:"startedAt"`
}

// CoversWholeRegion reports whether the alert covers a whole map region.
// Kyiv and Sevastopol have location type city but are separate regions on the map.
func (a Alert) CoversWholeRegion() bool {
	return a.LocationType == "oblast" || a.LocationUID == strconv.Itoa(a.OblastUID)
}

// Source returns the list of active alerts.
type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]Alert, error)
}

// ErrNotModified means the source data has not changed since the previous request.
var ErrNotModified = errors.New("alerts: not modified")
