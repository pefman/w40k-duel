# W40K CSV-backed Go API

This API serves Warhammer 40K data from the CSV files under `src/`.

## Endpoints

- GET `/api/factions` — list factions (from `Factions.csv`).
- GET `/api/{faction}/units` — list units (datasheets) for a faction by faction_id (e.g., `AC`, `ORK`).
- GET `/api/{faction}/{unit_id}` — unit (datasheet) basic info by ID.
- GET `/api/{faction}/{unit_id}/weapons` — weapons for that datasheet id.
- GET `/api/{faction}/{unit_id}/models` — models and statlines for that datasheet id.
- GET `/api/{faction}/{unit_id}/keywords` — keywords for that datasheet id (marks faction keywords).
- GET `/api/{faction}/{unit_id}/abilities` — abilities grouped by type with HTML stripped.
- GET `/api/{faction}/{unit_id}/options` — wargear/options text for that datasheet id.

Notes:
- `{unit_id}` here is the datasheet `id` column.
- Simple HTML tags in descriptions are stripped.

## Run locally

```
go run ./cmd/api
```

API will listen on `:${API_PORT:-8080}`. Set `API_PORT` to change.

## CSVs used
- `src/Factions.csv`
- `src/Datasheets.csv`
- `src/Datasheets_wargear.csv`
- `src/Datasheets_models.csv`
- `src/Datasheets_keywords.csv`
- `src/Datasheets_abilities.csv`
- `src/Datasheets_options.csv`

Extend `cmd/api/main.go` to include more files (`Abilities`, `Keywords`, etc.).
