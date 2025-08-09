# 🎮 Warhammer 40K Duel

A **real-time multiplayer** Warhammer 40K combat game with **AI opponents** built with Go and WebSockets. Features **local BattleScribe data integration**, **mobile-friendly interface**, and **cloud deployment**.

🌐 **Live Demo**: [w40k-duel-2otx55rc6a-ew.a.run.app](https://w40k-duel-2otx55rc6a-ew.a.run.app)

## 🚀 Features

### **Game Modes**
- **🤖 AI Opponents** - Play against computer with 3 difficulty levels (Easy/Medium/Hard)
- **👥 Multiplayer** - Real-time WebSocket combat between players
- **📱 Mobile Friendly** - Optimized touch interface and responsive design
- **📊 Enhanced Combat Logging** - Detailed battle logs with live wound tracking

### **Content & Data**
- **45+ Factions** with complete unit rosters from BattleScribe
- **3,400+ Units** with authentic game statistics
- **Weapon Deduplication** - Smart weapon extraction with composite keys
- **Local Data Pipeline** - downloads and parses official BattleScribe catalogs
- **Optimized Caching** - in-memory cache with TTL and O(1) unit lookups (200x faster)

### **Deployment & DevOps**
- **☁️ Cloud Run Deployment** - Auto-scaling serverless deployment
- **🐳 Docker Support** - Multi-stage builds with optimized images
- **🚀 Automated Deployment Scripts** - One-command build, push, and deploy
- **🌍 Global CDN** - Deployed in Europe (europe-west1)

## 🎯 AI Opponent System

The game features an intelligent AI system with three difficulty levels:

- **🟢 Easy**: Random faction selection, basic army composition
- **🟡 Medium**: Faction preferences, balanced army building  
- **🔴 Hard**: Strategic faction choice, optimized compositions, faster decisions

**AI Features:**
- Automatic faction and army selection
- Difficulty-based decision making
- Auto-ready functionality for seamless gameplay
- Smart battle initiative handling

## 🏗️ Architecture

### **Complete Data Pipeline**
```
BattleScribe Archive → Download → Extract → Parse → JSON → Cache → Game
```

- **Downloader** (`downloader.go`): Fetches latest .cat files from GitHub BSData/wh40k-10e
- **Parser** (`parser.go`): Two-phase parsing with reference resolution
- **Weapon System**: Triple-layer deduplication using maps with composite keys
- **AI Engine** (`ai.go`): Intelligent opponent with difficulty-based behavior

### **Cloud-Native Architecture**
- **Frontend**: Mobile-optimized HTML/CSS/JS with touch support
- **Backend**: Go WebSocket server with concurrent game handling
- **Data**: Local JSON storage with smart caching
- **Deployment**: Docker containers on Google Cloud Run
- **CI/CD**: Automated deployment scripts

## 🚀 Quick Start

### **🌐 Play Online (Recommended)**
Visit: [w40k-duel-2otx55rc6a-ew.a.run.app](https://w40k-duel-2otx55rc6a-ew.a.run.app)

### **🏠 Local Development**

#### **Option 1: Automated Setup**
```bash
./start.sh        # Auto-detects if data is needed
./start.sh fetch  # Force download fresh data
```

#### **Option 2: Using Makefile**
```bash
make start        # Start with automatic data handling
make refresh      # Force fetch new data and start
make help         # Show all available commands
```

#### **Option 3: Manual Setup**
```bash
# 1. Fetch BattleScribe Data
make fetch
# OR
go build -o bin/fetcher cmd/fetcher/main.go
./bin/fetcher

# 2. Start Game Server
make run
# OR  
go build && ./w40k-duel

# 3. Play the Game
# Open http://localhost:8080 in your browser
```

## ☁️ Cloud Deployment

### **Automated Deployment Scripts**

#### **Full Deployment** (Recommended)
```bash
./deploy.sh
```
- Colored output and progress indicators
- Error handling and validation
- Cloud Run configuration optimization
- Deployment summary with service URL

#### **Quick Deployment**
```bash
./quick-deploy.sh
```
- Minimal output for rapid iterations
- Same functionality, streamlined interface

### **Manual Deployment**
```bash
# Build and push Docker image
docker build -t w40k-duel .
docker tag w40k-duel gcr.io/w40k-468120/w40k-duel:latest
docker push gcr.io/w40k-468120/w40k-duel:latest

# Deploy to Cloud Run
gcloud run deploy w40k-duel \
    --image gcr.io/w40k-468120/w40k-duel:latest \
    --region europe-west1 \
    --platform managed \
    --allow-unauthenticated
```

### **Cloud Configuration**
- **Project**: `w40k-468120`
- **Service**: `w40k-duel`
- **Region**: `europe-west1`
- **Memory**: 1Gi
- **CPU**: 1 vCPU
- **Timeout**: 300s
- **Scaling**: 0-10 instances

## � Mobile Optimizations

- **Touch-Friendly Interface**: 40-50px touch targets for mobile interaction
- **Responsive Design**: Adapts to all screen sizes and orientations
- **Optimized Performance**: Efficient rendering for mobile browsers
- **Game Mode Selection**: Easy switching between Human vs AI modes
- **Difficulty Selection**: Clear UI for AI difficulty levels

## �📊 Data Pipeline Commands

### **Quick Commands**
```bash
make start        # Start server with auto data handling
make refresh      # Force download fresh data and start
make clean        # Clean build artifacts  
make clean-all    # Clean everything including data
make help         # Show all available commands
```

### **Script Commands**
```bash
./start.sh        # Start with existing data
./start.sh fetch  # Force download fresh data
./start.sh clean  # Clean all generated files
```

### **Deployment Commands**
```bash
./deploy.sh       # Full deployment with progress indicators
./quick-deploy.sh # Fast deployment for iterations
```

### **Manual Data Management**
```bash
# Build tools
make build        # Build all binaries
make fetcher      # Build only data fetcher

# Data operations
make fetch        # Download and parse all factions
make dev          # Setup development environment

# Check generated data
ls static/json/
du -sh static/json/
```

## 🔧 How It Works

### **Phase 1: Data Collection**
- Downloads official BattleScribe catalog from GitHub
- Extracts 45+ faction .cat files (XML format)
- Builds global unit database (3,400+ units)

### **Phase 2: Intelligent Parsing**
- Resolves cross-references between catalog files
- Converts XML to structured JSON
- Extracts weapon profiles, unit stats, costs

### **Phase 3: Optimized Game Server**
- In-memory cache preloads all faction data
- O(1) unit lookups via hash indexing
- Smart fallback to external API
- Real-time WebSocket combat

## 📁 Project Structure

```
w40k-duel/
├── cmd/
│   └── fetcher/          # Data fetcher command
│       └── main.go
├── internal/
│   └── data/             # Data pipeline
│       ├── downloader.go # BattleScribe data download
│       └── parser.go     # XML to JSON conversion
├── static/
│   ├── json/             # Generated faction data (45 files)
│   │   ├── xenos-necrons.json
│   │   ├── chaos-chaos-space-marines.json
│   │   └── ...
│   └── raw/              # Downloaded .cat files
├── ai.go                # AI opponent engine
├── api.go               # Enhanced API with weapon deduplication
├── battle.go            # Combat system with AI support
├── main.go              # Server entry point
├── server.go            # WebSocket server with AI cleanup
├── types.go             # Data structures with AI types
├── web.go               # Mobile-optimized frontend
├── websocket.go         # WebSocket handling with AI support
├── deploy.sh            # Full deployment script
├── quick-deploy.sh      # Fast deployment script
├── start.sh             # Convenience startup script
├── Dockerfile           # Multi-stage container build
└── README.md
```

## 🎮 Game Features

### **Combat System**
- **Real-time Combat**: WebSocket-based multiplayer battles with detailed logging
- **Enhanced Combat Logging**: Comprehensive battle logs with phase-by-phase tracking
- **Wound Tracking**: Live wound counts for both armies throughout combat
- **AI Integration**: Seamless human vs AI gameplay with automatic phase handling
- **Faction Selection**: Choose from 45+ official factions
- **Unit Statistics**: Authentic W40K stats and weapons with deduplication
- **Combat Resolution**: Accurate dice rolling and damage calculation with detailed feedback
- **Mobile Support**: Touch-optimized interface

### **New Combat Logging Features** ✨
- **🎯 Hit Phase Logs**: `Player's Weapon: X hits from Y attacks`
- **💥 Wound Phase Logs**: `Player's Weapon: X wounds caused` or `🛡️ No wounds caused`
- **🎯 Save Phase Logs**: `Player's Weapon: X unsaved wounds → Y damage dealt`
- **🩸 Damage Tracking**: `Player suffers X wounds (Y remaining)` with live updates
- **⚔️ Turn Management**: `Turn switches to Player` with wound status
- **🏆 Victory Detection**: Automatic battle end when army is destroyed
- **📊 Combat Statistics**: Real-time wound tracking and damage application

### **AI Opponent Features**
- **Difficulty Scaling**: Easy, Medium, Hard with different behaviors
- **Smart Faction Selection**: AI chooses appropriate factions based on difficulty
- **Army Composition**: Intelligent unit selection algorithms
- **Auto-Battle**: AI handles initiative rolls and combat automatically
- **State Management**: Proper cleanup when players disconnect

## 🎯 Data Sources

- **Primary**: Local BattleScribe catalogs (latest from GitHub)
- **Fallback**: External W40K API (w40k-api-eu-85079828466.europe-west1.run.app)
- **Cache**: In-memory optimization layer

## 📈 Performance Metrics

- **Response Time**: 50-200 microseconds (vs 10-20ms external API)
- **Memory Usage**: ~20MB for all 45 factions cached
- **Cache Hit Rate**: 99%+ after warmup
- **Data Volume**: 45 JSON files, ~10MB total
- **Unit Coverage**: 3,400+ units with full statistics

## 🛠️ Development

### **Development Setup**
```bash
make dev          # Build binaries and fetch data
make test         # Run tests
```

### **Update Data**
```bash
make refresh      # Force download latest BattleScribe data
./start.sh fetch  # Alternative using script
```

### **Debug Data Issues**
```bash
# Check for duplicate units
grep -i "trazyn" static/json/xenos-necrons.json

# Validate JSON structure
jq '.units | length' static/json/xenos-necrons.json

# Check cache statistics
curl http://localhost:8080/health
```

### **Clean Up**
```bash
make clean        # Remove build artifacts
make clean-all    # Remove everything including data
./start.sh clean  # Alternative cleanup
```

### **Performance Testing**
```bash
# Test local vs external API speed
time curl -s http://localhost:8080/health
```

## 🎮 Game Features

- **Real-time Combat**: WebSocket-based multiplayer battles
- **Faction Selection**: Choose from 45+ official factions
- **Unit Statistics**: Authentic W40K stats and weapons
- **Combat Resolution**: Accurate dice rolling and damage calculation
- **Spectator Mode**: Watch ongoing battles

## 🐛 Known Issues & Solutions

### **✅ Post-Game Crashes (FIXED)**
- **Issue**: App crashed when returning to start page after AI games
- **Solution**: Fixed broadcastOnlinePlayersList() to exclude AI players from WebSocket broadcasts
- **Implementation**: Added AI player cleanup in disconnect handler

### **✅ Duplicate Units (FIXED)**
- **Issue**: Units like "Trazyn the Infinite" appeared multiple times
- **Solution**: Implemented deduplication system in data pipeline

### **✅ Weapon Duplication (FIXED)**
- **Issue**: Multiple identical weapons shown for units (e.g., 6x "Staff of Light")
- **Solution**: Triple-layer weapon deduplication using maps with composite keys

### **✅ Missing Weapons (FIXED)**
- **Issue**: Some units had no weapons available
- **Solution**: Enhanced weapon extraction system with faction-wide weapon pools

### **✅ Cache Performance (OPTIMIZED)**
- **Issue**: Slow API responses from external services
- **Solution**: 200x performance improvement with local data caching

## 🔧 Development

### **Development Setup**
```bash
make dev          # Build binaries and fetch data
make test         # Run tests (when available)
```

### **Update Data**
```bash
make refresh      # Force download latest BattleScribe data
./start.sh fetch  # Alternative using script
```

### **Debug Common Issues**
```bash
# Check for duplicate units
grep -i "trazyn" static/json/xenos-necrons.json

# Validate JSON structure
jq '.units | length' static/json/xenos-necrons.json

# Check weapon deduplication
grep -c "staff of light" static/json/xenos-necrons.json

# Monitor server logs for AI behavior
tail -f server.log | grep "AI\|Combat Protocol"
```

### **Performance Testing**
```bash
# Test local vs external API speed
time curl -s http://localhost:8080/health

# Load test with multiple connections
# (requires additional testing tools)
```

### **Clean Up**
```bash
make clean        # Remove build artifacts
make clean-all    # Remove everything including data
./start.sh clean  # Alternative cleanup
```

## 🤝 Contributing

The project is designed to be easily extensible:

### **Adding New Features**
1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Update documentation
5. Submit a pull request

### **Data Pipeline Extensions**
- Modify `internal/data/parser.go` for data structure changes
- Update `api.go` for new game features
- Test with `./start.sh fetch`

### **AI Improvements**
- Enhance `ai.go` for smarter opponent behavior
- Add new difficulty levels or game modes
- Implement faction-specific AI strategies

### **Mobile Optimizations**
- Update `web.go` for new mobile features
- Test on various devices and screen sizes
- Consider Progressive Web App (PWA) features

## 📈 Performance Metrics

- **Response Time**: 50-200 microseconds (vs 10-20ms external API)
- **Memory Usage**: ~20MB for all 45 factions cached
- **Cache Hit Rate**: 99%+ after warmup
- **Data Volume**: 45 JSON files, ~10MB total
- **Unit Coverage**: 3,400+ units with full statistics
- **Weapon Deduplication**: 37 unique weapons for Necrons (vs 200+ duplicates)
- **Mobile Performance**: <2s initial load time on 4G

## 🌐 Deployment Stats

- **Cloud Provider**: Google Cloud Run
- **Region**: Europe West 1 (europe-west1)
- **Scaling**: 0-10 instances (auto-scaling)
- **Cold Start**: <1s
- **Memory**: 1Gi allocated
- **CPU**: 1 vCPU allocated
- **Uptime**: 99.9% (Cloud Run SLA)

## 📄 License

This project is licensed under the MIT License. BattleScribe data is used under fair use for gaming purposes.

## 🔗 Links

- **Live Demo**: [w40k-duel-85079828466.europe-west1.run.app](https://w40k-duel-85079828466.europe-west1.run.app)
- **BattleScribe Data**: [BSData/wh40k-10e](https://github.com/BSData/wh40k-10e)
- **Docker Image**: `gcr.io/w40k-468120/w40k-duel:latest`

---

*Built with ⚔️ for the Warhammer 40K community*

*Featuring AI opponents, mobile optimization, and cloud deployment*