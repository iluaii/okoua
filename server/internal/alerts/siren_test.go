// SPDX-License-Identifier: AGPL-3.0-or-later

package alerts

import (
	"encoding/json"
	"testing"
)

func TestConvertSiren(t *testing.T) {
	var tree []sirenRegion
	json.Unmarshal([]byte(`[
		{"regionId":"25","regionName":"Чернігівська область","regionType":"State","regionChildIds":[
			{"regionId":"101","regionName":"Ніжинський район","regionType":"District","regionChildIds":[
				{"regionId":"1001","regionName":"Ніжинська громада","regionType":"Community"}]}]},
		{"regionId":"9999","regionName":"Автономна Республіка Крим","regionType":"State"},
		{"regionId":"0","regionName":"Тестовий регіон","regionType":"State"}
	]`), &tree)
	parent := sirenParents(tree)

	var raw []sirenAlertRegion
	json.Unmarshal([]byte(`[
		{"regionId":"9999","regionName":"Автономна Республіка Крим","regionType":"State","activeAlerts":[
			{"regionId":"9999","regionType":"State","type":"AIR","lastUpdate":"2022-12-10T22:22:00Z","activeAlertLevels":[{"createdAt":"2022-12-10T22:22:00Z"}]}]},
		{"regionId":"101","regionName":"Ніжинський район","regionType":"District","activeAlerts":[
			{"regionId":"101","regionType":"District","type":"ARTILLERY","lastUpdate":"2026-10-12T10:00:00Z"}]},
		{"regionId":"1001","regionName":"Ніжинська громада","regionType":"Community","activeAlerts":[
			{"regionId":"101","regionType":"District","type":"ARTILLERY","lastUpdate":"2026-10-12T10:00:00Z"},
			{"regionId":"1001","regionType":"Community","type":"AIR","lastUpdate":"2026-10-12T10:05:00Z"}]},
		{"regionId":"0","regionName":"Тестовий регіон","regionType":"State","activeAlerts":[
			{"regionId":"0","regionType":"State","type":"AIR","lastUpdate":"2026-10-12T10:00:00Z"}]}
	]`), &raw)

	list := convertSiren(raw, parent)
	if len(list) != 3 {
		t.Fatalf("got %d alerts, want 3 (test region skipped): %+v", len(list), list)
	}
	crimea, district, community := list[0], list[1], list[2]
	if crimea.OblastUID != 29 || !crimea.CoversWholeRegion() {
		t.Errorf("crimea = %+v, want whole region 29", crimea)
	}
	if district.OblastUID != 25 || district.Type != "artillery_shelling" || district.LocationType != "raion" || district.LocationTitle != "Ніжинський район" || district.CoversWholeRegion() {
		t.Errorf("district = %+v", district)
	}
	if community.OblastUID != 25 || community.LocationType != "hromada" || community.LocationTitle != "Ніжинська громада" {
		t.Errorf("community = %+v", community)
	}
}
