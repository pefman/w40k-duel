# Warhammer 40K Duel Simulator

A lightweight API and static UI to simulate 40K 10th Edition shooting exchanges, track stats, and browse unit data.

## 🎮 Features

### Core Gameplay
- **Real-time multiplayer combat** via WebSockets
- **Multi-weapon attack sequences** with animated dice rolls
- **Manual save mechanics** with persistent dice visualization
- **AI opponents** with smart unit/points matching
- **Phase-based combat**: Attacks → Hit → Wound → Save → Damage
- **Weapon abilities**: Lethal Hits, Devastating Wounds, Twin-Linked, etc.

### Data & Units
- **10th Edition datasheet integration** from CSV exports
- **Full faction/unit/weapon loadouts** with points costs
- **Smart weapon categorization** (melee vs ranged)
- **Invulnerable saves, Feel No Pain, damage reduction**
- **Keywords and special abilities** support

### User Experience
- **Lobby system** showing online players and their loadouts
- **Daily leaderboards** tracking top damage and worst saves
- **Responsive design** for desktop and mobile
- **Auto-generated player names** with Warhammer flavor
- **Real-time status updates** and match notifications

### Technical Features
- **Embedded single-binary deployment** (no external files)
- **Docker and Cloud Run ready** with environment configuration
- **CORS-enabled API** for external integrations
- **Graceful WebSocket handling** with reconnection support
- **Development scripts** for local testing

## 🏗️ Architecture

### Services
1. **API Service** (`cmd/api/`): CSV-backed REST API serving faction/unit data and static UI

### Data Flow
```
CSV Data → API Service → Game Service → WebSocket → Browser Client
```

### Key Components
- **Matchmaker**: Pairs players or matches with AI
- **Room System**: Isolated game instances with state management  
- **Combat Engine**: Handles dice rolling, damage calculation, special rules
- **Lobby Manager**: Tracks online players and game status

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker (optional, for containerized deployment)

### Local Development
```bash
# Clone the repository
git clone <repository-url>
cd w40k-duel

# Start the API locally
scripts/dev.sh restart     # API on :8080

# View logs
scripts/dev.sh logs

# Open game in browser
open http://localhost:8081
```

Notes:
- Local API persists match logs to JSON under `tmp/matches/` for debugging (controlled by `MATCH_LOG_DIR`, enabled by `scripts/api.sh`). Production stays memory-only.

### Game Usage
1. **Select faction and unit** from dropdowns
2. **Choose weapons** (must be same type: all melee or all ranged)
3. **Lock in** your selection
4. **Choose opponent**: "Play vs AI" or "Play vs Player"
5. **Battle**: Take turns attacking, opponent rolls saves
6. **Win condition**: Reduce opponent to 0 wounds

## 📁 Repository Structure

```
├── cmd/
│   └── api/           # CSV-backed API server
├── scripts/           # Development helper scripts
│   ├── api.sh         # API service management
│   ├── game.sh        # Game service management
│   └── dev.sh         # Combined development workflow
├── src/               # CSV data files (10th edition exports)
│   ├── Datasheets.csv
│   ├── Datasheets_weapons.csv
│   └── ...
├── Dockerfile.api     # API container
└── README.md          # Documentation
```

## ⚙️ Configuration

### Environment Variables

#### API Service
- `API_PORT` or `PORT`: Listen port (default: 8080)
- `DATA_DIR`: CSV data directory (default: ./src)

#### Game Service  
- `GAME_PORT` or `PORT`: Listen port (default: 8081)
- `DATA_API_BASE`: API service URL (default: http://localhost:8080)

### Build-time Variables
```bash
# Inject version and build time
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildTime=$(date -u +%Y%m%d-%H%M%S)"
```

## 🐳 Deployment

### Local Docker
```bash
# Build images
docker build -f Dockerfile.api -t w40k-duel .
docker build -f Dockerfile.game -t w40k-game .

# Run with docker-compose
docker-compose up
```

### Cloud Run (Google Cloud)
```bash
# Build and push API image via Cloud Build (optional)
./scripts/deploy_cloud_run_cloudbuild.sh   # uses cloudbuild_api.yaml

# Or build locally and deploy (defaults to service name w40k-duel)
./scripts/deploy_cloud_run.sh

# Deploy using a stable service config (always same service/region/image path)
gcloud run services replace cloudrun_api.yaml --region europe-west1
```

## ✅ Post-deploy validation (Cloud Run)

### UI checks
- Open the live app in two browsers: https://w40k-duel-85079828466.europe-west1.run.app
- Click “Matchmake” in both; confirm the weapon selection hides instantly and stays hidden.
- In the lobby, queued players show a badge “queue · Npts” while waiting.
- The match proceeds turn-by-turn; both clients’ HP update in sync; a winner overlay is shown at the end.

### API sanity
```bash
# Health
curl -s https://w40k-duel-85079828466.europe-west1.run.app/api/healthz

# PvP debug (queue and active matches)
curl -s https://w40k-duel-85079828466.europe-west1.run.app/api/pvp/debug

# Lobby state
curl -s https://w40k-duel-85079828466.europe-west1.run.app/api/lobby
```

## 🎯 Game Rules Implementation

### Combat Sequence
1. **Attacks Phase**: Roll attack dice (supports expressions like "4D6", "D3+3")
2. **Hit Phase**: Roll to hit based on BS/WS, apply Lethal Hits
3. **Wound Phase**: Roll to wound based on S vs T, apply Devastating Wounds  
4. **Save Phase**: Defender rolls saves (armor or invuln), apply damage reduction
5. **Damage Phase**: Apply final damage, check for victory

### Special Rules Supported
- **Lethal Hits**: Critical hits automatically wound
- **Devastating Wounds**: Critical wounds bypass saves as mortal damage
- **Twin-Linked**: Re-roll failed wound rolls
- **Sustained Hits**: Generate additional hits on critical hit rolls
- **Anti-X**: Enhanced hit chances against specific keywords
- **Feel No Pain**: Ignore damage on successful rolls
- **Damage Reduction**: Reduce damage per attack

### AI Behavior
- **Points Matching**: AI selects units within ±5% of player's unit cost
- **Weapon Category Mirroring**: AI uses same weapon type (melee/ranged) as player
- **Smart Timing**: AI attacks after brief delays for realistic feel

## 📊 API Endpoints

### Faction Data
- `GET /api/factions` - List all factions
- `GET /api/{faction-slug}/units` - Units for faction
- `GET /api/{faction-slug}/{unit-id}/weapons` - Unit weapons
- `GET /api/{faction-slug}/{unit-id}/models` - Unit models/stats
- `GET /api/{faction-slug}/{unit-id}/keywords` - Unit keywords  
- `GET /api/{faction-slug}/{unit-id}/abilities` - Unit abilities
- `GET /api/{faction-slug}/{unit-id}/options` - Weapon options
- `GET /api/{faction-slug}/{unit-id}/costs` - Points costs

### Game Data
- `GET /lobby` - Online players and status
- `GET /leaderboard` - Overall player rankings  
- `GET /leaderboard/daily` - Daily records (damage/saves)
- `GET /debug` - Development endpoints
- `WebSocket /ws` - Real-time game connection

## 🛠️ Development

### Code Organization
- **Domain models**: Clean separation of data structures
- **API client**: Centralized external API communication
- **Game engine**: Isolated combat logic and state management
- **WebSocket layer**: Real-time communication handling
- **Embedded UI**: Single-binary deployment with embedded assets

### Key Optimizations
- **Faction caching**: Reduces redundant API calls
- **Efficient state broadcasting**: Minimizes WebSocket traffic
- **Connection pooling**: Reuses HTTP connections to data API
- **Graceful error handling**: Continues operation despite individual failures

### Testing Locally
```bash
# Start services
scripts/dev.sh restart

# Test API directly
curl http://localhost:8080/api/factions

# Test game WebSocket (requires wscat)
wscat -c ws://localhost:8081/ws

# View logs
scripts/dev.sh logs
tail -f logs/api.log logs/game.log
```

## 📝 Data Sources

CSV files in `src/` contain exported 10th Edition datasheet data:
- `Datasheets.csv` - Unit basic stats and info
- `Datasheets_weapons.csv` - Weapon profiles and abilities  
- `Datasheets_abilities.csv` - Special rules and keywords
- `Factions.csv` - Faction metadata
- `*.csv` - Additional reference data

**Note**: Warhammer 40,000 content belongs to Games Workshop. This project is for educational/personal use only.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with appropriate tests
4. Submit a pull request

### Code Style
- Follow Go standard formatting (`gofmt`)
- Use meaningful variable names
- Add comments for complex game logic
- Keep functions focused and testable

## 📄 License

For personal/experimental use only. CSV content and game rules belong to their respective owners. Do not redistribute commercially.

---

**Built with Go, WebSockets, and ⚔️ for the Emperor!**
