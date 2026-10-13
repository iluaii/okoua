// SPDX-License-Identifier: AGPL-3.0-or-later

package alerts

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

// Mock generates fake alerts for development without an API token.
// Each Fetch call randomly starts or ends alerts.
type Mock struct {
	mu     sync.Mutex
	rng    *rand.Rand
	now    func() time.Time
	alerts []Alert
	nextID int
}

// Regions with a permanent alert, as in reality.
var mockPermanent = []int{16, 29}

// Real district IDs, so partial alerts land on raions of the map.
var mockDistricts = map[int][][2]string{
	9:  {{"44", "Дніпровський район"}, {"45", "Павлоградський район"}, {"47", "Нікопольський район"}},
	20: {{"114", "Сумський район"}, {"115", "Шосткинський район"}, {"117", "Конотопський район"}},
	22: {{"122", "Чугуївський район"}, {"124", "Харківський район"}, {"125", "Ізюмський район"}},
	25: {{"141", "Новгород-Сіверський район"}, {"142", "Ніжинський район"}, {"144", "Корюківський район"}},
}

func NewMock() *Mock {
	m := &Mock{rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0)), now: time.Now}
	start := m.now().Add(-72 * time.Hour)
	for _, uid := range mockPermanent {
		m.add(uid, true, "air_raid", start)
	}
	m.add(22, false, "air_raid", m.now().Add(-25*time.Minute))
	m.add(25, false, "air_raid", m.now().Add(-10*time.Minute))
	m.add(9, true, "air_raid", m.now().Add(-70*time.Minute))
	return m
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Fetch(ctx context.Context) ([]Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.step()
	return slices.Clone(m.alerts), nil
}

func (m *Mock) step() {
	// Rare enough that push notifications during testing stay readable (with 15s polling,
	// a change every couple of minutes).
	if m.rng.Float64() < 0.1 {
		free := m.freeRegions()
		if len(free) > 0 {
			uid := free[m.rng.IntN(len(free))]
			whole := m.rng.Float64() < 0.75
			typ := "air_raid"
			if !whole && m.rng.Float64() < 0.3 {
				typ = "artillery_shelling"
			}
			m.add(uid, whole, typ, m.now())
		}
	}
	if m.rng.Float64() < 0.08 {
		var removable []int
		for i, a := range m.alerts {
			if !slices.Contains(mockPermanent, a.OblastUID) {
				removable = append(removable, i)
			}
		}
		if len(removable) > 0 {
			i := removable[m.rng.IntN(len(removable))]
			m.alerts = slices.Delete(m.alerts, i, i+1)
		}
	}
}

func (m *Mock) freeRegions() []int {
	var free []int
	for uid := range Regions {
		if !slices.ContainsFunc(m.alerts, func(a Alert) bool { return a.OblastUID == uid }) {
			free = append(free, uid)
		}
	}
	slices.Sort(free)
	return free
}

func (m *Mock) add(uid int, whole bool, typ string, at time.Time) {
	m.nextID++
	a := Alert{
		ID:        fmt.Sprintf("mock-%d", m.nextID),
		Type:      typ,
		OblastUID: uid,
		StartedAt: at.UTC().Truncate(time.Second),
	}
	districts := mockDistricts[uid]
	switch {
	case whole || len(districts) == 0:
		a.LocationUID = fmt.Sprint(uid)
		a.LocationTitle = Regions[uid]
		a.LocationType = "oblast"
	case m.rng.Float64() < 0.3:
		d := districts[m.rng.IntN(len(districts))]
		a.LocationUID = fmt.Sprintf("%s-h%d", d[0], m.nextID)
		a.LocationTitle = "Тестова громада" // shown in the UI, so Ukrainian
		a.LocationType = "hromada"
		a.RaionUID = d[0]
	default:
		d := districts[m.rng.IntN(len(districts))]
		a.LocationUID, a.LocationTitle, a.LocationType, a.RaionUID = d[0], d[1], "raion", d[0]
	}
	m.alerts = append(m.alerts, a)
}
