// SPDX-License-Identifier: AGPL-3.0-or-later

// Generates SVG paths for the oblasts and raions from data/geo/ukr-adm2.geojson into src/lib/map/regions.ts and raions.ts.
// Oblasts are dissolved from raions, so their borders match exactly.
// Projection: Lambert conformal conic (as on familiar maps of Ukraine); simplification keeps shared borders.
// Boundaries include territorial waters (hence circles around capes and Zmiinyi Island),
// so they are clipped to land from Natural Earth.

import { readFile, writeFile } from 'node:fs/promises';
import mapshaper from 'mapshaper';

const SRC = new URL('../../data/geo/ukr-adm2.geojson', import.meta.url);
const LAND = new URL('../../data/geo/land-ukr.geojson', import.meta.url);
const DISTRICTS = new URL('../../data/geo/siren-districts.json', import.meta.url);
const OUT = new URL('../src/lib/map/regions.ts', import.meta.url);
// Raions are a separate module: the map loads them in the background after the first paint.
const OUT_RAIONS = new URL('../src/lib/map/raions.ts', import.meta.url);
const WIDTH = 1000;
const PAD = 6;

// OCHA oblast pcode → UID and name from alerts.in.ua.
const REGIONS = {
	UA68: [3, 'Хмельницька область'],
	UA05: [4, 'Вінницька область'],
	UA56: [5, 'Рівненська область'],
	UA07: [8, 'Волинська область'],
	UA12: [9, 'Дніпропетровська область'],
	UA18: [10, 'Житомирська область'],
	UA21: [11, 'Закарпатська область'],
	UA23: [12, 'Запорізька область'],
	UA26: [13, 'Івано-Франківська область'],
	UA32: [14, 'Київська область'],
	UA35: [15, 'Кіровоградська область'],
	UA44: [16, 'Луганська область'],
	UA48: [17, 'Миколаївська область'],
	UA51: [18, 'Одеська область'],
	UA53: [19, 'Полтавська область'],
	UA59: [20, 'Сумська область'],
	UA61: [21, 'Тернопільська область'],
	UA63: [22, 'Харківська область'],
	UA65: [23, 'Херсонська область'],
	UA71: [24, 'Черкаська область'],
	UA74: [25, 'Чернігівська область'],
	UA73: [26, 'Чернівецька область'],
	UA46: [27, 'Львівська область'],
	UA14: [28, 'Донецька область'],
	UA01: [29, 'Автономна Республіка Крим'],
	UA85: [30, 'м. Севастополь'],
	UA80: [31, 'м. Київ']
};
// Cities are drawn on top of the oblasts around them.
const ON_TOP = new Set([30, 31]);
// Raions renamed after the OCHA data was made: alert source name → OCHA name.
const RENAMED = {
	'володимир-волинський': 'володимирський',
	сєвєродонецький: 'сіверськодонецький',
	красноградський: 'берестинський'
};

const input = await readFile(SRC, 'utf8');
const land = await readFile(LAND, 'utf8');
const out = await mapshaper.applyCommands(
	[
		'-i in.json name=raions',
		'-clip land.json',
		// Clipping leaves tiny slivers along the coast.
		'-filter-slivers min-area=2km2',
		'-proj +proj=lcc +lat_1=45 +lat_2=50 +lat_0=48 +lon_0=31 +datum=WGS84',
		'-simplify 20% keep-shapes',
		"-each 'cx=this.innerX, cy=this.innerY'",
		'-dissolve adm1_pcode + name=oblasts',
		"-each 'cx=this.innerX, cy=this.innerY'",
		'-innerlines target=raions + name=mesh',
		'-o target=* format=geojson'
	].join(' '),
	{ 'in.json': input, 'land.json': land }
);
// A layer without attributes comes out as a GeometryCollection.
const layer = (name) => {
	const json = JSON.parse(out[`${name}.json`]);
	return json.features ?? json.geometries.map((geometry) => ({ geometry }));
};

const rings = (g) => (g.type === 'Polygon' ? [g.coordinates] : g.coordinates).flat();
const lines = (g) => (g.type === 'LineString' ? [g.coordinates] : g.coordinates);
let [x0, y0, x1, y1] = [Infinity, Infinity, -Infinity, -Infinity];
for (const f of layer('oblasts')) {
	for (const [x, y] of rings(f.geometry).flat()) {
		x0 = Math.min(x0, x);
		x1 = Math.max(x1, x);
		y0 = Math.min(y0, y);
		y1 = Math.max(y1, y);
	}
}
const k = (WIDTH - 2 * PAD) / (x1 - x0);
const height = Math.ceil((y1 - y0) * k + 2 * PAD);
const px = (x) => +((x - x0) * k + PAD).toFixed(1);
const py = (y) => +((y1 - y) * k + PAD).toFixed(1);
const line = (pts) => 'M' + pts.map(([x, y]) => `${px(x)} ${py(y)}`).join('L');
const path = (g) => rings(g).map((ring) => line(ring) + 'Z').join('');

const oblast = (pcode) => {
	const known = REGIONS[pcode];
	if (!known) throw new Error(`unknown region ${pcode}`);
	return known;
};

const regions = layer('oblasts').map((f) => {
	const [uid, name] = oblast(f.properties.adm1_pcode);
	return { uid, name, d: path(f.geometry), cx: px(f.properties.cx), cy: py(f.properties.cy) };
});
if (regions.length !== Object.keys(REGIONS).length) throw new Error(`got ${regions.length} regions`);
regions.sort((a, b) => Number(ON_TOP.has(a.uid)) - Number(ON_TOP.has(b.uid)) || a.uid - b.uid);

// Raions get the alert source's district ID and name. Raions the source doesn't list
// (Crimea, occupied parts of Luhansk oblast, the Chornobyl zone) are left out: they only
// ever have alerts as part of a whole-oblast alert.
const norm = (s) =>
	s
		.toLowerCase()
		.replace(/[ʼ’`]/g, "'")
		.replace(/ район$/, '')
		.trim();
const ochaRaions = new Map(layer('raions').map((f) => [`${oblast(f.properties.adm1_pcode)[0]}:${norm(f.properties.adm2_name1)}`, f]));
const districts = JSON.parse(await readFile(DISTRICTS, 'utf8'));
const raions = districts.map((d) => {
	const name = norm(d.name);
	const f = ochaRaions.get(`${d.oblast}:${RENAMED[name] ?? name}`);
	if (!f) throw new Error(`no geometry for district ${d.id} ${d.name}`);
	return { id: d.id, uid: Number(d.oblast), name: d.name, d: path(f.geometry), cx: px(f.properties.cx), cy: py(f.properties.cy) };
});
raions.sort((a, b) => Number(a.id) - Number(b.id));

const mesh = layer('mesh')
	.flatMap((f) => lines(f.geometry))
	.map(line)
	.join('');

// Neighbours: regions that share border points. Simplification keeps shared borders identical,
// so common vertices are exact; a few shared points are enough to call regions adjacent.
const pointsOf = (d) => new Set(d.match(/-?[\d.]+ -?[\d.]+/g));
const sets = new Map(regions.map((r) => [r.uid, pointsOf(r.d)]));
const neighbours = {};
for (const a of regions) {
	neighbours[a.uid] = regions
		.filter((b) => b.uid !== a.uid && [...sets.get(a.uid)].filter((p) => sets.get(b.uid).has(p)).length >= 2)
		.map((b) => b.uid)
		.sort((x, y) => x - y);
}

const ts = `// SPDX-License-Identifier: AGPL-3.0-or-later
// Generated by scripts/gen-map.mjs, do not edit by hand.
// Boundaries: OCHA / SSE «Kartographia» (CC BY-IGO), via HDX.

export interface MapRegion {
	uid: number;
	name: string;
	d: string;
	/** A point inside the region, for a label. */
	cx: number;
	cy: number;
}

export interface MapRaion {
	/** District ID of the alert source (siren.pp.ua, alerts.in.ua). */
	id: string;
	/** Oblast UID. */
	uid: number;
	name: string;
	d: string;
	cx: number;
	cy: number;
}

export const viewBox = '0 0 ${WIDTH} ${height}';

export const regions: MapRegion[] = ${JSON.stringify(regions, null, '\t')};

/** Bordering regions by UID. */
export const neighbours: Record<number, number[]> = ${JSON.stringify(neighbours)};
`;
const raionsTs = `// SPDX-License-Identifier: AGPL-3.0-or-later
// Generated by scripts/gen-map.mjs, do not edit by hand.
// Boundaries: OCHA / SSE «Kartographia» (CC BY-IGO), via HDX.

import type { MapRaion } from './regions.ts';

export const raions: MapRaion[] = ${JSON.stringify(raions, null, '\t')};

/** Borders between raions, one path. */
export const raionMesh = ${JSON.stringify(mesh)};
`;
await writeFile(OUT, ts);
await writeFile(OUT_RAIONS, raionsTs);
console.log(
	`regions.ts: ${regions.length} regions, ${(ts.length / 1024).toFixed(0)} KB; raions.ts: ${raions.length} raions, ${(raionsTs.length / 1024).toFixed(0)} KB; viewBox 0 0 ${WIDTH} ${height}`
);
