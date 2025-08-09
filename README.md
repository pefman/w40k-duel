# go40k duel

A lightweight Warhammer 40K duel simulator:
- API service: serves factions, units, weapons, options, and points from CSVs.
- Game service: WebSocket-based duel with multi-weapon sequencing, manual saves, and animated rolls.

## Repo layout
- cmd/api: CSV-backed API server
- cmd/game: Game server + embedded UI
- scripts/: helper scripts (build/run/restart/logs for api and game)
- src/: CSV data exported from 10th edition datasheets

## Local dev
Use the scripts to build and run:

```bash
# Start API on :8080
scripts/api.sh restart
# Start Game on :8081 (reads DATA_API_BASE)
DATA_API_BASE=http://localhost:8080 scripts/game.sh restart
# Open game UI
xdg-open http://localhost:8081
```

## Deploy
See `README_DEPLOY.md` for Cloud Run steps. Both services include Dockerfiles and honor `PORT`.

## Configuration
- API: API_PORT (default 8080) or PORT
- Game: GAME_PORT (default 8081) or PORT; DATA_API_BASE (default http://localhost:8080)

## Features
- CSV ingestion with stable ordering and CORS
- Units list shows W/T and points
- Options-aware weapon selection (heuristic)
- Manual save rolls with one-click submission
- Multi-weapon per-turn attack sequencing
- Dice roll animations with short pacing pauses
- Fun random player names; no manual callsign

## License
For personal/experimental use. CSV content belongs to their respective owners; do not redistribute.
