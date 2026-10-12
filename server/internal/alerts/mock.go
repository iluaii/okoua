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

func NewMock() *Mock {
	m := &Mock{rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0)), now: time.Now}
	start := m.now().Add(-72 * time.Hour)
	for _, uid := range mockPermanent {
		m.add(uid, true, "air_raid", start)
	}
	m.add(22, false, "air_raid", m.now().Add(-25*time.Minute))
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
	if m.rng.Float64() < 0.35 {
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
	if m.rng.Float64() < 0.3 {
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
	if whole {
		a.LocationUID = fmt.Sprint(uid)
		a.LocationTitle = Regions[uid]
		a.LocationType = "oblast"
	} else {
		a.LocationUID = fmt.Sprintf("%d-r%d", uid, m.nextID)
		a.LocationTitle = "Тестовий район" // shown in the UI, so Ukrainian
		a.LocationType = "raion"
	}
	m.alerts = append(m.alerts, a)
}
