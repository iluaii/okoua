// SPDX-License-Identifier: AGPL-3.0-or-later

package power

// Community datasets. They have no license yet: ask the authors before a public launch.
var (
	viaOE          = &Link{Name: "OE_OUTAGE_DATA", URL: "https://github.com/yaroslav2901/OE_OUTAGE_DATA"}
	viaBaskerville = &Link{Name: "outage-data-ua", URL: "https://github.com/Baskerville42/outage-data-ua"}
)

const (
	oeRaw          = "https://raw.githubusercontent.com/yaroslav2901/OE_OUTAGE_DATA/main/data/"
	baskervilleRaw = "https://raw.githubusercontent.com/Baskerville42/outage-data-ua/main/data/"
)

// DefaultRegions are the regions with a working schedule source (checked 2026-10-12).
// Names are shown in the UI, so they are Ukrainian.
func DefaultRegions() []Region {
	return []Region{
		{
			UID:       31, // Kyiv city
			Source:    NewYasno(25, 902),
			Operator:  Link{Name: "YASNO (ДТЕК Київські електромережі)", URL: "https://yasno.ua"},
			LookupURL: "https://yasno.ua",
		},
		{
			UID:       9, // Dnipropetrovsk oblast
			Source:    NewYasno(3, 301),
			Operator:  Link{Name: "YASNO (ДТЕК Дніпровські електромережі)", URL: "https://yasno.ua"},
			LookupURL: "https://yasno.ua",
		},
		{
			UID:       14, // Kyiv oblast
			Source:    NewGPV(baskervilleRaw + "kyiv-region.json"),
			Operator:  Link{Name: "ДТЕК Київські регіональні електромережі", URL: "https://www.dtek-krem.com.ua/ua/shutdowns"},
			Via:       viaBaskerville,
			LookupURL: "https://www.dtek-krem.com.ua/ua/shutdowns",
		},
		{
			UID:       25, // Chernihiv oblast
			Source:    NewGPV(oeRaw + "Chernihivoblenergo.json"),
			Operator:  Link{Name: "АТ «Чернігівобленерго»", URL: "https://t.me/chernigivoblenergo"},
			Via:       viaOE,
			LookupURL: "https://chernihivoblenergo.com.ua/blackouts",
		},
		{
			UID:      22, // Kharkiv oblast
			Source:   NewGPV(oeRaw + "Kharkivoblenerho.json"),
			Operator: Link{Name: "АТ «Харківобленерго»"},
			Via:      viaOE,
		},
		{
			UID:      19, // Poltava oblast
			Source:   NewGPV(oeRaw + "Poltavaoblenergo.json"),
			Operator: Link{Name: "АТ «Полтаваобленерго»"},
			Via:      viaOE,
		},
	}
}
