# Geodata

## `ukr-adm2.geojson`: raion boundaries

Boundaries of Ukraine's raions after the 2020 reform (ADM2, 139 units: 136 raions, the Chornobyl exclusion zone, Kyiv and Sevastopol). Oblasts are dissolved from them, so oblast and raion borders match.

- Source: [OCHA COD-AB Ukraine](https://data.humdata.org/dataset/cod-ab-ukr) on HDX (`ukr_admin2.geojson`, valid on 2025-09-01), from the State Scientific Production Enterprise «Kartographia».
- License: Creative Commons Attribution for Intergovernmental Organisations (CC BY-IGO). The attribution is shown on the site.
- Prepared with: `mapshaper -i ukr_admin2.geojson -filter-fields adm1_pcode,adm1_name1,adm2_pcode,adm2_name1 -simplify 5% keep-shapes -o ukr-adm2.geojson precision=0.0001`

## `siren-districts.json`: alert source district IDs

District (raion) IDs and names from `https://siren.pp.ua/api/v3/regions` (the Ukraine Alarm scheme, also used by alerts.in.ua), snapshot of 2026-10-13. The map generator matches them to raions by oblast and name; three raions renamed since the OCHA data are listed in `RENAMED` in the generator. Crimea and occupied parts of Luhansk oblast have no districts there.

## `land-ukr.geojson`: land

Boundaries extend 12 nautical miles into the sea (territorial waters), so they are clipped to land.

- Source: [Natural Earth](https://www.naturalearthdata.com/) 10m land, clipped to the bounding box of Ukraine.
- License: public domain.

## Generating the map

`web/scripts/gen-map.mjs` turns these files into SVG paths in `web/src/lib/map/regions.ts`:

```
cd web && npm run gen:map
```
