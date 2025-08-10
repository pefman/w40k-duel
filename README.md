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

Alternatively, during development you can use the multiprocess dev script:

```bash
scripts/dev.sh restart   # rebuild + restart api and game
scripts/dev.sh logs      # tail logs
```

## Deploy
See `README_DEPLOY.md` for Cloud Run steps. Both services include Dockerfiles and honor `PORT`.

## Configuration
- API: API_PORT (default 8080) or PORT
- Game: GAME_PORT (default 8081) or PORT; DATA_API_BASE (default http://localhost:8080)

## Features
- CSV ingestion with stable ordering and CORS
- Units list shows W/T, invuln/FNP, and points
- Options-aware weapon selection (heuristic), melee/ranged quick-pick
- Multi-weapon per-turn attack sequencing with an Attacks pre-phase (supports expressions like 4D6, D6+3)
- Manual save rolls with consolidated “Roll Saves” action; persistent dice strip shows hits→wounds→saves with failures marked
- Weapon ability tags (e.g., Lethal Hits) rendered inline with weapon names
- Clear phase progress chips (Attacks → Hit → Wound → Save → Damage)
- Matchmaking: explicit “Play vs AI” or “Play vs Player”; queued players show as “Looking for match...”
- Lobby/Leaderboard and Daily Records are always visible at the bottom of the page
- Daily Records: track today’s highest single attack damage and worst single save roll
- Fun random player names; no manual callsign

## License
For personal/experimental use. CSV content belongs to their respective owners; do not redistribute.
