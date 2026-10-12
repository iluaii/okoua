# Geodata

## `ukr-adm1.geojson`: oblast boundaries

Boundaries of Ukraine's oblasts (ADM1, 27 regions, including the Autonomous Republic of Crimea, Sevastopol and Kyiv).

- Source: [geoBoundaries](https://www.geoboundaries.org/) gbOpen UKR ADM1 (simplified), based on OpenStreetMap data.
- License: Open Data Commons Open Database License 1.0 (ODbL). The «© учасники OpenStreetMap» attribution is required and is shown on the site.

## `land-ukr.geojson`: land

OSM oblast boundaries extend 12 nautical miles into the sea (territorial waters), so oblasts are clipped to land.

- Source: [Natural Earth](https://www.naturalearthdata.com/) 10m land, clipped to the bounding box of Ukraine.
- License: public domain.

## Generating the map

`web/scripts/gen-map.mjs` turns these files into SVG paths in `web/src/lib/map/regions.ts`:

```
cd web && npm run gen:map
```
