package main

import (
	"net/http"
)

// HTTP handlers
func (gs *GameServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Warhammer 40K Duel</title>
        <style>
        @import url('https://fonts.googleapis.com/css2?family=Cinzel:wght@400;600;700&family=Rajdhani:wght@300;400;500;600;700&display=swap');
        
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        /* Touch-friendly improvements */
        button, .faction-card, .unit-card, .weapon-item, .quantity-btn {
            touch-action: manipulation;
            -webkit-tap-highlight-color: rgba(212, 175, 55, 0.3);
        }
        
        /* Prevent text selection on interactive elements */
        .faction-card, .unit-card, .weapon-item, button {
            -webkit-user-select: none;
            -moz-user-select: none;
            -ms-user-select: none;
            user-select: none;
        }
        
        body {
            font-family: 'Rajdhani', sans-serif;
            background: #0a0a0a;
            color: #e5e5e5;
            min-height: 100vh;
            background-image: 
                radial-gradient(circle at 20% 20%, rgba(212, 175, 55, 0.1) 0%, transparent 50%),
                radial-gradient(circle at 80% 80%, rgba(139, 0, 0, 0.1) 0%, transparent 50%),
                linear-gradient(135deg, #0a0a0a 0%, #1a1a1a 50%, #0a0a0a 100%);
            overflow-x: hidden;
            -webkit-font-smoothing: antialiased;
            -moz-osx-font-smoothing: grayscale;
        }
        
        .main-header {
            background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
            border-bottom: 2px solid #d4af37;
            padding: 20px 0;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
            position: relative;
        }
        
        .main-header::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><polygon fill="%23d4af37" fill-opacity="0.05" points="50,10 90,90 10,90"/></svg>') repeat;
            opacity: 0.1;
        }
        
        .header-content {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 20px;
            position: relative;
            z-index: 2;
        }
        
        h1 {
            font-family: 'Cinzel', serif;
            font-size: 3.5em;
            font-weight: 700;
            text-align: center;
            background: linear-gradient(135deg, #d4af37 0%, #ffd700 50%, #d4af37 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            text-shadow: 0 0 30px rgba(212, 175, 55, 0.3);
            margin-bottom: 10px;
            letter-spacing: 2px;
        }
        
        .subtitle {
            text-align: center;
            font-size: 1.2em;
            color: #b8860b;
            font-weight: 500;
            letter-spacing: 1px;
            text-transform: uppercase;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 40px 20px;
        }
        
        .step-indicator {
            display: flex;
            justify-content: center;
            margin-bottom: 50px;
            gap: 20px;
        }
        
        .step {
            background: linear-gradient(135deg, #2d2d2d 0%, #1a1a1a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 12px;
            padding: 15px 30px;
            color: #666;
            font-weight: 600;
            font-size: 1.1em;
            text-transform: uppercase;
            letter-spacing: 1px;
            position: relative;
            transition: all 0.3s ease;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.3);
        }
        
        .step.active {
            background: linear-gradient(135deg, #d4af37 0%, #b8860b 100%);
            border-color: #ffd700;
            color: #000;
            box-shadow: 0 6px 25px rgba(212, 175, 55, 0.4);
            transform: translateY(-2px);
        }
        
        .step.completed {
            background: linear-gradient(135deg, #228b22 0%, #006400 100%);
            border-color: #32cd32;
            color: #fff;
        }
        
        .section {
            background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
            border: 2px solid #3d3d3d;
            border-radius: 15px;
            padding: 40px;
            margin-bottom: 30px;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
            position: relative;
            overflow: hidden;
        }
        
        .section::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 3px;
            background: linear-gradient(90deg, #d4af37 0%, #ffd700 50%, #d4af37 100%);
        }
        
        h2 {
            font-family: 'Cinzel', serif;
            font-size: 2.5em;
            font-weight: 600;
            text-align: center;
            margin-bottom: 30px;
            color: #d4af37;
            text-shadow: 0 0 20px rgba(212, 175, 55, 0.3);
            letter-spacing: 1px;
        }
        
        .faction-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 25px;
            margin-top: 30px;
        }
        
        .faction-card {
            background: linear-gradient(135deg, #2d2d2d 0%, #1a1a1a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 12px;
            padding: 25px;
            cursor: pointer;
            transition: all 0.3s ease;
            text-align: center;
            position: relative;
            overflow: hidden;
        }
        
        .faction-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: -100%;
            width: 100%;
            height: 100%;
            background: linear-gradient(90deg, transparent, rgba(212, 175, 55, 0.1), transparent);
            transition: left 0.5s ease;
        }
        
        .faction-card:hover {
            border-color: #d4af37;
            transform: translateY(-5px);
            box-shadow: 0 10px 30px rgba(212, 175, 55, 0.2);
        }
        
        .faction-card:hover::before {
            left: 100%;
        }
        
        .faction-name {
            font-family: 'Cinzel', serif;
            font-size: 1.5em;
            font-weight: 600;
            color: #d4af37;
            margin-bottom: 10px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .faction-subtitle {
            color: #b8860b;
            font-size: 1.1em;
            font-weight: 500;
        }
        
        .unit-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 20px;
            margin-top: 30px;
        }
        
        .unit-card {
            background: linear-gradient(135deg, #2d2d2d 0%, #1a1a1a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 10px;
            padding: 20px;
            transition: all 0.3s ease;
            position: relative;
        }
        
        .unit-card:hover {
            border-color: #d4af37;
            box-shadow: 0 8px 25px rgba(212, 175, 55, 0.15);
        }
        
        .unit-name {
            font-family: 'Cinzel', serif;
            font-size: 1.3em;
            font-weight: 600;
            color: #d4af37;
            margin-bottom: 15px;
        }
        
        .unit-stats {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 10px;
            margin-bottom: 15px;
            font-size: 0.95em;
        }
        
        .stat {
            background: rgba(212, 175, 55, 0.1);
            padding: 8px 12px;
            border-radius: 6px;
            border: 1px solid rgba(212, 175, 55, 0.3);
        }
        
        .stat-label {
            font-size: 0.8em;
            color: #b8860b;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        .stat-value {
            font-size: 1.1em;
            font-weight: 700;
            color: #d4af37;
            margin-top: 2px;
        }
        
        .weapon-configurator {
            background: linear-gradient(135deg, #1a1a1a 0%, #0a0a0a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 10px;
            padding: 25px;
            margin: 20px 0;
        }
        
        .weapon-type-buttons {
            display: flex;
            gap: 15px;
            margin-bottom: 20px;
            justify-content: center;
        }
        
        .weapon-type-btn {
            background: linear-gradient(135deg, #3d3d3d 0%, #2d2d2d 100%);
            border: 2px solid #4d4d4d;
            color: #e5e5e5;
            padding: 12px 25px;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 1px;
            transition: all 0.3s ease;
            font-family: 'Rajdhani', sans-serif;
        }
        
        .weapon-type-btn:hover {
            border-color: #d4af37;
            box-shadow: 0 4px 15px rgba(212, 175, 55, 0.2);
        }
        
        .weapon-type-btn.selected {
            background: linear-gradient(135deg, #d4af37 0%, #b8860b 100%);
            border-color: #ffd700;
            color: #000;
        }
        
        .weapon-list {
            display: grid;
            gap: 12px;
        }
        
        .weapon-item {
            background: linear-gradient(135deg, #2d2d2d 0%, #1a1a1a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 8px;
            padding: 15px;
            cursor: pointer;
            transition: all 0.3s ease;
        }
        
        .weapon-item:hover {
            border-color: #d4af37;
            box-shadow: 0 4px 15px rgba(212, 175, 55, 0.2);
        }
        
        .weapon-item.selected {
            background: linear-gradient(135deg, rgba(212, 175, 55, 0.2) 0%, rgba(212, 175, 55, 0.1) 100%);
            border-color: #d4af37;
        }
        
        .weapon-stats {
            display: flex;
            gap: 15px;
            font-size: 0.9em;
            color: #ccc;
            margin-top: 8px;
            flex-wrap: wrap;
        }
        
        .weapon-stats span {
            background: rgba(212, 175, 55, 0.1);
            padding: 4px 8px;
            border-radius: 4px;
            border: 1px solid rgba(212, 175, 55, 0.2);
        }
        
        .no-weapons {
            text-align: center;
            padding: 30px;
            color: #888;
            font-style: italic;
            font-size: 1.1em;
        }
        
        .primary-btn {
            background: linear-gradient(135deg, #d4af37 0%, #b8860b 100%);
            color: #000;
            border: 2px solid #ffd700;
            padding: 15px 35px;
            border-radius: 8px;
            font-size: 1.2em;
            font-weight: 700;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 1px;
            font-family: 'Rajdhani', sans-serif;
            box-shadow: 0 4px 15px rgba(212, 175, 55, 0.3);
        }
        
        .primary-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 25px rgba(212, 175, 55, 0.4);
        }
        
        .primary-btn:disabled {
            background: #3d3d3d;
            color: #666;
            border-color: #4d4d4d;
            cursor: not-allowed;
            transform: none;
            box-shadow: none;
        }
        
        .game-mode-buttons {
            display: flex;
            gap: 20px;
            justify-content: center;
            flex-wrap: wrap;
            margin-bottom: 20px;
        }
        
        .ai-btn {
            background: linear-gradient(135deg, #8b0000 0%, #dc143c 100%);
            border-color: #dc143c;
        }
        
        .ai-btn:hover {
            box-shadow: 0 6px 25px rgba(220, 20, 60, 0.4);
        }
        
        .difficulty-buttons {
            display: flex;
            gap: 15px;
            justify-content: center;
            flex-wrap: wrap;
            margin-top: 15px;
        }
        
        .difficulty-btn {
            background: linear-gradient(135deg, #2c2c2c 0%, #1a1a1a 100%);
            color: #e5e5e5;
            border: 2px solid #555;
            padding: 10px 25px;
            border-radius: 6px;
            font-size: 1em;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: capitalize;
            font-family: 'Rajdhani', sans-serif;
        }
        
        .difficulty-btn:hover, .difficulty-btn.selected {
            background: linear-gradient(135deg, #d4af37 0%, #b8860b 100%);
            color: #000;
            border-color: #ffd700;
            transform: translateY(-1px);
        }
        
        #aiDifficultySection {
            margin-top: 20px;
            padding: 20px;
            background: rgba(139, 0, 0, 0.1);
            border: 1px solid rgba(220, 20, 60, 0.3);
            border-radius: 8px;
        }
        
        #aiDifficultySection h3 {
            color: #dc143c;
            margin-bottom: 15px;
            text-align: center;
            font-weight: 600;
        }
        
        .army-summary {
            background: linear-gradient(135deg, #1a1a1a 0%, #0a0a0a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 10px;
            padding: 25px;
            margin-top: 30px;
        }
        
        .army-unit {
            background: rgba(212, 175, 55, 0.05);
            border: 1px solid rgba(212, 175, 55, 0.2);
            border-radius: 6px;
            padding: 15px;
            margin-bottom: 10px;
        }
        
        .army-unit.invalid {
            border-color: rgba(220, 20, 60, 0.5);
            background: rgba(220, 20, 60, 0.05);
        }
        
        .player-info {
            background: rgba(212, 175, 55, 0.1);
            border: 1px solid rgba(212, 175, 55, 0.3);
            text-align: center;
        }
        
        .online-players {
            background: rgba(212, 175, 55, 0.05);
            border: 1px solid rgba(212, 175, 55, 0.2);
            border-radius: 8px;
            padding: 15px;
            margin-top: 20px;
        }
        
        .player-list {
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
            margin-top: 10px;
        }
        
        .player-item {
            background: rgba(212, 175, 55, 0.1);
            border: 1px solid rgba(212, 175, 55, 0.3);
            border-radius: 6px;
            padding: 8px 12px;
            font-size: 0.9em;
            color: #d4af37;
        }
        
        .hidden {
            display: none !important;
        }
        
        .text-center {
            text-align: center;
        }
        
        .loading {
            text-align: center;
            padding: 50px;
            font-size: 1.2em;
            color: #d4af37;
        }
        
        .loading::after {
            content: '';
            display: inline-block;
            width: 20px;
            height: 20px;
            border: 2px solid #3d3d3d;
            border-top: 2px solid #d4af37;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin-left: 10px;
        }
        
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        
        .quantity-controls {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 15px;
            margin-top: 15px;
        }
        
        .quantity-btn {
            background: linear-gradient(135deg, #3d3d3d 0%, #2d2d2d 100%);
            border: 2px solid #4d4d4d;
            color: #e5e5e5;
            width: 40px;
            height: 40px;
            border-radius: 8px;
            cursor: pointer;
            font-weight: bold;
            font-size: 1.2em;
            transition: all 0.3s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            touch-action: manipulation;
            user-select: none;
        }
        
        .quantity-btn:hover {
            border-color: #d4af37;
            transform: scale(1.05);
        }
        
        .quantity-btn:active {
            transform: scale(0.95);
        }
        
        .quantity-display {
            background: rgba(212, 175, 55, 0.1);
            border: 1px solid rgba(212, 175, 55, 0.3);
            padding: 10px 18px;
            border-radius: 8px;
            font-weight: 600;
            font-size: 1.1em;
            min-width: 60px;
            text-align: center;
        }
        
        @media (max-width: 768px) {
            h1 {
                font-size: 2.2em;
                line-height: 1.2;
            }
            
            .subtitle {
                font-size: 1em;
            }
            
            .container {
                padding: 15px 10px;
            }
            
            .faction-grid,
            .unit-grid {
                grid-template-columns: 1fr;
                gap: 15px;
            }
            
            .step-indicator {
                flex-wrap: wrap;
                gap: 8px;
                justify-content: center;
            }
            
            .step {
                padding: 8px 16px;
                font-size: 0.85em;
                min-width: auto;
                flex: none;
            }
            
            .faction-card,
            .unit-card,
            .weapon-configurator {
                margin: 0;
                padding: 15px;
            }
            
            .unit-stats {
                grid-template-columns: 1fr;
                gap: 8px;
            }
            
            .weapon-type-buttons {
                flex-wrap: wrap;
                gap: 10px;
            }
            
            .weapon-type-btn {
                padding: 10px 20px;
                font-size: 0.9em;
                flex: 1;
                min-width: 120px;
            }
            
            .weapon-stats {
                gap: 8px;
                font-size: 0.8em;
            }
            
            .primary-btn {
                width: 100%;
                padding: 12px 25px;
                font-size: 1.1em;
                margin: 15px 0;
            }
            
            .army-summary {
                padding: 15px;
                margin-top: 20px;
            }
            
            .battle-log {
                height: 200px;
                font-size: 0.9em;
            }
            
            .unit-controls {
                flex-direction: column;
                gap: 10px;
            }
            
            .unit-controls button {
                width: 100%;
                padding: 10px;
            }
            
            .quantity-controls {
                gap: 12px;
                margin-top: 20px;
            }
            
            .quantity-btn {
                width: 45px;
                height: 45px;
                font-size: 1.3em;
            }
            
            .quantity-display {
                padding: 12px 20px;
                font-size: 1.2em;
                min-width: 70px;
            }
        }
        
        @media (max-width: 480px) {
            h1 {
                font-size: 1.8em;
            }
            
            .main-header {
                padding: 15px 0;
            }
            
            .container {
                padding: 10px 8px;
            }
            
            .step {
                font-size: 0.8em;
                padding: 6px 12px;
            }
            
            .faction-card,
            .unit-card {
                padding: 12px;
            }
            
            .faction-name,
            .unit-name {
                font-size: 1.2em;
            }
            
            .weapon-type-btn {
                padding: 8px 15px;
                font-size: 0.85em;
                min-width: 100px;
            }
            
            .primary-btn {
                font-size: 1em;
                padding: 10px 20px;
            }
            
            .quantity-btn {
                width: 50px;
                height: 50px;
                font-size: 1.4em;
            }
            
            .quantity-display {
                padding: 14px 22px;
                font-size: 1.3em;
                min-width: 80px;
            }
        }
        </style>
    </head>
<body>
    <div class="main-header">
        <div class="header-content">
            <h1>Warhammer 40K Duel Arena</h1>
            <div class="subtitle">In the grim darkness of the far future, there is only war</div>
        </div>
    </div>
    
    <div class="container">

        
        <div class="step-indicator">
            <div class="step" id="step1">Choose Faction</div>
            <div class="step" id="step2">Select Units</div>
            <div class="step" id="step3">Configure Weapons</div>
            <div class="step" id="step4">Deploy Army</div>
        </div>
        
        <div id="playerInfo" class="section player-info hidden">
            <h3>Player: <span id="playerName"></span></h3>
            <p>ID: <span id="playerId"></span></p>
        </div>

        <div id="matchmakingSection" class="section">
            <h2>Choose Game Mode</h2>
            <div class="game-mode-buttons">
                <button id="joinMatchmaking" class="primary-btn">Find Human Opponent</button>
                <button id="playVsAI" class="primary-btn ai-btn">Play vs AI</button>
            </div>
            <div id="aiDifficultySection" class="hidden">
                <h3>Select AI Difficulty</h3>
                <div class="difficulty-buttons">
                    <button class="difficulty-btn" data-difficulty="easy">Easy</button>
                    <button class="difficulty-btn" data-difficulty="medium">Medium</button>
                    <button class="difficulty-btn" data-difficulty="hard">Hard</button>
                </div>
            </div>
            <div id="matchmakingStatus"></div>
        </div>

        <div id="factionSelection" class="section hidden">
            <h2>Choose Your Faction</h2>
            <div id="factionGrid" class="faction-grid"></div>
        </div>

        <div id="unitSelection" class="section hidden">
            <h2>Select Units & Quantities</h2>
            <div id="unitGrid" class="unit-grid"></div>
            <button id="proceedToWeapons" class="primary-btn" disabled>Configure Weapons</button>
        </div>

        <div id="weaponSelection" class="section hidden">
            <h2>Choose Weapon Loadouts</h2>
            <div id="weaponConfigurator"></div>
            <div class="army-summary">
                <h3>Army Summary</h3>
                <div id="armySummary"></div>
            </div>
            <button id="confirmArmy" class="primary-btn" disabled>Deploy Army</button>
        </div>

        <div id="battleSection" class="section hidden">
            <h2>Battle Arena</h2>
            <div id="battleInfo"></div>
            
            <div class="dice-roller">
                <h3>Dice Roller</h3>
                <button class="dice-btn" onclick="rollDice(6)">D6</button>
                <button class="dice-btn" onclick="rollDice(3)">D3</button>
                <button class="dice-btn" onclick="rollDice(20)">D20</button>
                <div id="diceResults"></div>
            </div>

            <div id="battleLog" class="battle-log">
                <h3>Battle Log</h3>
                <div id="logContent"></div>
            </div>
        </div>

        <div id="onlinePlayersSection" class="section online-players">
            <h3>Players Online</h3>
            <div id="onlinePlayersList"></div>
        </div>
    </div>

    <script>
        let ws;
        let gameState = 'disconnected';
        let playerData = {};
        let selectedFaction = '';
        let selectedUnits = {}; // {unitName: quantity}
        let selectedWeapons = {}; // {unitName: {weaponType: 'Ranged'|'Melee', weapons: []}}
        let availableUnits = [];
        let availableWeapons = {};
        let savedPlayerName = '';
        let matchmakingTimer = null;
        let matchmakingSeconds = 0;
        let currentStep = 1;

        // Step management
        function updateStepIndicator(step) {
            // Mark previous steps as completed
            for (let i = 1; i < step; i++) {
                document.getElementById('step' + i).className = 'step completed';
            }
            // Mark current step as active
            document.getElementById('step' + step).className = 'step active';
            // Reset future steps
            for (let i = step + 1; i <= 4; i++) {
                document.getElementById('step' + i).className = 'step';
            }
            currentStep = step;
        }

        // Load saved name from localStorage on page load
        function loadSavedName() {
            savedPlayerName = localStorage.getItem('w40k_player_name') || '';
        }

        // Save name to localStorage when we get a new random name
        function saveRandomName(name) {
            localStorage.setItem('w40k_player_name', name);
            savedPlayerName = name;
        }

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');
            
            ws.onopen = function() {
                console.log('Connected to game server');
                gameState = 'connected';
                
                // Send saved name if we have one
                if (savedPlayerName) {
                    console.log('Sending saved name:', savedPlayerName);
                    ws.send(JSON.stringify({
                        type: 'set_name',
                        name: savedPlayerName
                    }));
                }
            };

            ws.onmessage = function(event) {
                const message = JSON.parse(event.data);
                console.log('Received message:', message);
                handleMessage(message);
            };

            ws.onclose = function() {
                console.log('Disconnected from server');
                gameState = 'disconnected';
                resetMatchmaking();
                setTimeout(connect, 3000);
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }

        function handleMessage(message) {
            console.log('Received:', message);

            switch(message.type) {
                case 'player_info':
                    playerData = message;
                    document.getElementById('playerName').textContent = message.name;
                    document.getElementById('playerId').textContent = message.player_id;
                    document.getElementById('playerInfo').classList.remove('hidden');
                    
                    if (!savedPlayerName) {
                        saveRandomName(message.name);
                    }
                    break;

                case 'matchmaking_status':
                    document.getElementById('matchmakingStatus').innerHTML = '<p>' + message.message + '</p>';
                    break;

                case 'match_found':
                    if (matchmakingTimer) {
                        clearInterval(matchmakingTimer);
                        matchmakingTimer = null;
                    }
                    
                    document.getElementById('matchmakingStatus').innerHTML = 
                        '<p style="color: #00ff00; font-weight: bold;">Match Found! Opponent: ' + message.opponent + '</p>';
                    
                    setTimeout(() => {
                        document.getElementById('matchmakingSection').classList.add('hidden');
                        updateStepIndicator(1);
                    }, 1500);
                    break;

                case 'factions_available':
                    populateFactions(message.factions);
                    document.getElementById('factionSelection').classList.remove('hidden');
                    updateStepIndicator(1);
                    break;

                case 'faction_selected':
                    availableUnits = message.units;
                    populateUnits(message.units);
                    document.getElementById('factionSelection').classList.add('hidden');
                    document.getElementById('unitSelection').classList.remove('hidden');
                    updateStepIndicator(2);
                    break;

                case 'unit_weapons':
                    displayUnitWeapons(message);
                    break;

                case 'army_selected':
                    document.getElementById('weaponSelection').classList.add('hidden');
                    document.getElementById('battleSection').classList.remove('hidden');
                    updateStepIndicator(4);
                    showWaitingForBattle();
                    break;

                case 'battle_started':
                    startBattle(message);
                    break;

                case 'initiative_roll':
                    showInitiativeRoll(message);
                    break;

                case 'initiative_tie':
                    showInitiativeTie(message);
                    break;

                case 'initiative_resolved':
                    showInitiativeResolved(message);
                    break;

                case 'combat_round':
                    updateBattleLog(message);
                    break;

                case 'battle_finished':
                    finishBattle(message);
                    break;

                case 'dice_result':
                    showDiceResult(message.dice, message.result);
                    break;

                case 'opponent_dice_roll':
                    showOpponentDiceRoll(message);
                    break;

                case 'opponent_disconnected':
                    resetMatchmaking();
                    alert('Your opponent has disconnected');
                    location.reload();
                    break;

                case 'players_online':
                    updateOnlinePlayersList(message.players);
                    break;
            }
        }

        function sendMessage(message) {
            console.log('Sending message:', message);
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify(message));
                console.log('Message sent successfully');
            } else {
                console.log('WebSocket not ready. State:', ws ? ws.readyState : 'null');
            }
        }

        function joinMatchmaking() {
            console.log('Join matchmaking clicked');
            console.log('WebSocket state:', ws ? ws.readyState : 'null');
            sendMessage({type: 'join_matchmaking'});
            const button = document.getElementById('joinMatchmaking');
            button.disabled = true;
            
            matchmakingSeconds = 0;
            button.textContent = 'Searching for Opponent (0)';
            
            matchmakingTimer = setInterval(() => {
                matchmakingSeconds++;
                button.textContent = 'Searching for Opponent (' + matchmakingSeconds + ')';
            }, 1000);
        }
        
        function playVsAI() {
            console.log('Play vs AI clicked');
            const difficultySection = document.getElementById('aiDifficultySection');
            difficultySection.classList.remove('hidden');
            
            // Hide the game mode buttons
            document.querySelector('.game-mode-buttons').style.display = 'none';
        }
        
        function selectAIDifficulty(difficulty) {
            console.log('AI difficulty selected:', difficulty);
            sendMessage({
                type: 'play_vs_ai',
                difficulty: difficulty
            });
            
            // Update UI to show AI match starting
            document.getElementById('matchmakingStatus').innerHTML = 
                '<p>Starting AI match on <strong>' + difficulty + '</strong> difficulty...</p>';
            
            // Hide difficulty selection
            document.getElementById('aiDifficultySection').classList.add('hidden');
        }

        function resetMatchmaking() {
            if (matchmakingTimer) {
                clearInterval(matchmakingTimer);
                matchmakingTimer = null;
            }
            const button = document.getElementById('joinMatchmaking');
            button.disabled = false;
            button.textContent = 'Find Human Opponent';
            matchmakingSeconds = 0;
            
            // Reset AI UI
            document.querySelector('.game-mode-buttons').style.display = 'flex';
            document.getElementById('aiDifficultySection').classList.add('hidden');
            document.getElementById('matchmakingStatus').innerHTML = '';
            
            // Reset difficulty button selection
            document.querySelectorAll('.difficulty-btn').forEach(btn => {
                btn.classList.remove('selected');
            });
        }

        function populateFactions(factions) {
            const grid = document.getElementById('factionGrid');
            grid.innerHTML = '';
            
            // Sort factions alphabetically
            const sortedFactions = factions.slice().sort();
            
            sortedFactions.forEach(faction => {
                const card = document.createElement('div');
                card.className = 'faction-card';
                card.onclick = () => selectFaction(faction);
                
                const name = faction.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                
                card.innerHTML = 
                    '<div class="faction-name">' + name + '</div>' +
                    '<div class="faction-subtitle">Faction</div>';
                grid.appendChild(card);
            });
        }

        function getFactionEmoji(faction) {
            if (faction.includes('space-marines') || faction.includes('adeptus-astartes')) return '[SM]';
            if (faction.includes('chaos')) return '☠️';
            if (faction.includes('ork')) return '[ORK]';
            if (faction.includes('eldar') || faction.includes('aeldari')) return '✨';
            if (faction.includes('tau')) return '🎯';
            if (faction.includes('necron')) return '💀';
            if (faction.includes('tyranid')) return '👹';
            if (faction.includes('guard') || faction.includes('militarum')) return '🎖️';
            return '⚔️';
        }

        function selectFaction(faction) {
            // Remove previous selection
            document.querySelectorAll('.faction-card').forEach(card => {
                card.classList.remove('selected');
            });
            
            // Select new faction
            event.target.classList.add('selected');
            selectedFaction = faction;
            
            // Automatically send faction selection
            sendMessage({
                type: 'select_faction',
                faction: selectedFaction
            });
        }

        function populateUnits(units) {
            const grid = document.getElementById('unitGrid');
            grid.innerHTML = '';

            // Sort units alphabetically by name
            const sortedUnits = units.slice().sort((a, b) => a.name.localeCompare(b.name));

            sortedUnits.forEach(unit => {
                const card = document.createElement('div');
                card.className = 'unit-card';
                
                card.innerHTML = 
                    '<div class="unit-header">' +
                        '<div class="unit-name">' + unit.name + '</div>' +
                    '</div>' +
                    '<div class="unit-stats">' +
                        '<div class="stat"><div class="stat-label">Wounds</div><div class="stat-value">' + unit.wounds + '</div></div>' +
                        '<div class="stat"><div class="stat-label">Attacks</div><div class="stat-value">' + unit.attacks + '</div></div>' +
                        '<div class="stat"><div class="stat-label">Strength</div><div class="stat-value">' + unit.strength + '</div></div>' +
                        '<div class="stat"><div class="stat-label">Toughness</div><div class="stat-value">' + unit.toughness + '</div></div>' +
                    '</div>' +
                    '<div class="quantity-controls">' +
                        '<button class="quantity-btn" onclick="adjustQuantity(\'' + unit.name + '\', -1)">-</button>' +
                        '<div class="quantity-display" id="qty-' + unit.name.replace(/\s+/g, '-') + '">0</div>' +
                        '<button class="quantity-btn" onclick="adjustQuantity(\'' + unit.name + '\', 1)">+</button>' +
                    '</div>';
                
                grid.appendChild(card);
            });
        }

        function adjustQuantity(unitName, delta) {
            const current = selectedUnits[unitName] || 0;
            const newQty = Math.max(0, Math.min(10, current + delta));
            
            if (newQty === 0) {
                delete selectedUnits[unitName];
            } else {
                selectedUnits[unitName] = newQty;
            }
            
            const displayId = 'qty-' + unitName.replace(/\s+/g, '-');
            document.getElementById(displayId).textContent = newQty;
            
            updateProceedButton();
        }

        function updateProceedButton() {
            const hasUnits = Object.keys(selectedUnits).length > 0;
            document.getElementById('proceedToWeapons').disabled = !hasUnits;
        }

        function proceedToWeapons() {
            // Store available weapons for selected units
            availableWeapons = {};
            availableUnits.forEach(unit => {
                if (selectedUnits[unit.name]) {
                    availableWeapons[unit.name] = unit.weapon_types || [];
                }
            });
            
            populateWeaponSelection();
            document.getElementById('unitSelection').classList.add('hidden');
            document.getElementById('weaponSelection').classList.remove('hidden');
            updateStepIndicator(3);
        }

        function populateWeaponSelection() {
            const configurator = document.getElementById('weaponConfigurator');
            configurator.innerHTML = '';

            Object.keys(selectedUnits).forEach(unitName => {
                const quantity = selectedUnits[unitName];
                const weaponTypes = availableWeapons[unitName] || [];
                
                if (weaponTypes.length === 0) {
                    return; // Skip units with no weapons
                }

                const unitDiv = document.createElement('div');
                unitDiv.className = 'unit-card';
                
                // Determine available weapon categories
                const hasRanged = weaponTypes.includes('Ranged');
                const hasMelee = weaponTypes.includes('Melee');
                
                let weaponTypeButtons = '';
                if (hasRanged) {
                    weaponTypeButtons += '<button class="weapon-type-btn" onclick="selectWeaponType(\'' + unitName + '\', \'Ranged\')">Ranged Weapons</button>';
                }
                if (hasMelee) {
                    weaponTypeButtons += '<button class="weapon-type-btn" onclick="selectWeaponType(\'' + unitName + '\', \'Melee\')">Melee Weapons</button>';
                }
                
                unitDiv.innerHTML =
                    '<div class="unit-header">' +
                        '<div class="unit-name">' + unitName + ' ×' + quantity + '</div>' +
                    '</div>' +
                    '<div class="weapon-selection">' +
                        '<h4>Choose Weapon Type:</h4>' +
                        '<div class="weapon-type-selector">' + weaponTypeButtons + '</div>' +
                        '<div class="weapon-list" id="weapons-' + unitName.replace(/\s+/g, '-') + '"></div>' +
                    '</div>';
                
                configurator.appendChild(unitDiv);
            });
            
            updateArmySummary();
        }

        function selectWeaponType(unitName, weaponType) {
            // Update button selection
            const buttons = document.querySelectorAll('.weapon-type-btn');
            buttons.forEach(btn => btn.classList.remove('selected'));
            event.target.classList.add('selected');
            
            // Initialize weapon selection for this unit
            if (!selectedWeapons[unitName]) {
                selectedWeapons[unitName] = {};
            }
            selectedWeapons[unitName].weaponType = weaponType;
            selectedWeapons[unitName].weapons = [];
            
            // Show available weapons of this type
            showWeaponsForType(unitName, weaponType);
            updateArmySummary();
        }

        function showWeaponsForType(unitName, weaponType) {
            // Request weapon data from server
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'get_unit_weapons',
                    unit_name: unitName
                }));
                
                // Store the requested weapon type to filter results
                window.pendingWeaponType = weaponType;
                window.pendingUnitName = unitName;
            }
        }

        function displayUnitWeapons(message) {
            // Check if this response matches our pending request
            if (window.pendingUnitName !== message.unit_name) {
                return;
            }
            
            const weaponType = window.pendingWeaponType;
            const unitName = message.unit_name;
            const weaponsByType = message.weapons_by_type;
            
            const weaponList = document.getElementById('weapons-' + unitName.replace(/\s+/g, '-'));
            if (!weaponList) {
                return;
            }
            
            // Get weapons for the selected type
            const weapons = weaponsByType[weaponType] || [];
            
            weaponList.innerHTML = '';
            if (weapons.length === 0) {
                weaponList.innerHTML = '<div class="no-weapons">No ' + weaponType.toLowerCase() + ' weapons available for this unit</div>';
                return;
            }
            
            weapons.forEach(weapon => {
                const weaponDiv = document.createElement('div');
                weaponDiv.className = 'weapon-item';
                weaponDiv.onclick = () => selectWeapon(unitName, weapon);
                
                weaponDiv.innerHTML =
                    '<div><strong>' + sanitizeText(weapon.name) + '</strong></div>' +
                    '<div class="weapon-stats">' +
                        '<span>Range: ' + sanitizeText(weapon.range || 'N/A') + '</span>' +
                        '<span>A: ' + sanitizeText(weapon.attacks || 'N/A') + '</span>' +
                        '<span>S: ' + sanitizeText(weapon.strength || 'N/A') + '</span>' +
                        '<span>AP: ' + sanitizeText(weapon.ap || 'N/A') + '</span>' +
                        '<span>D: ' + sanitizeText(weapon.damage || 'N/A') + '</span>' +
                    '</div>';
                
                weaponList.appendChild(weaponDiv);
            });
            
            // Clear pending request
            window.pendingWeaponType = null;
            window.pendingUnitName = null;
        }

        function sanitizeText(text) {
            if (!text) return '';
            // Remove special Unicode characters that display as strange symbols
            return text.replace(/[^\x20-\x7E]/g, '').replace(/➤\s*/g, '').trim();
        }

        function updateOnlinePlayersList(players) {
            const list = document.getElementById('onlinePlayersList');
            if (!list) return;
            
            list.innerHTML = '';
            
            if (!players || players.length === 0) {
                list.innerHTML = '<div class="player-item">No other players online</div>';
                return;
            }
            
            // Sort players alphabetically
            const sortedPlayers = players.slice().sort((a, b) => a.name.localeCompare(b.name));
            
            const playerListDiv = document.createElement('div');
            playerListDiv.className = 'player-list';
            
            sortedPlayers.forEach(player => {
                const playerDiv = document.createElement('div');
                playerDiv.className = 'player-item';
                playerDiv.textContent = player.name || 'Anonymous';
                playerListDiv.appendChild(playerDiv);
            });
            
            list.appendChild(playerListDiv);
        }

        function selectWeapon(unitName, weapon) {
            // Toggle weapon selection
            if (!selectedWeapons[unitName].weapons) {
                selectedWeapons[unitName].weapons = [];
            }
            
            const index = selectedWeapons[unitName].weapons.findIndex(w => w.name === weapon.name);
            if (index >= 0) {
                selectedWeapons[unitName].weapons.splice(index, 1);
                event.target.classList.remove('selected');
            } else {
                selectedWeapons[unitName].weapons.push(weapon);
                event.target.classList.add('selected');
            }
            
            updateArmySummary();
        }

        function updateArmySummary() {
            const summary = document.getElementById('armySummary');
            summary.innerHTML = '';
            
            let isValid = true;
            
            Object.keys(selectedUnits).forEach(unitName => {
                const quantity = selectedUnits[unitName];
                const weapons = selectedWeapons[unitName];
                
                const unitDiv = document.createElement('div');
                unitDiv.className = 'army-unit';
                
                let weaponInfo = 'No weapons selected';
                let unitValid = false;
                
                if (weapons && weapons.weaponType && weapons.weapons.length > 0) {
                    weaponInfo = weapons.weaponType + ': ' + weapons.weapons.map(w => sanitizeText(w.name || w.Name)).join(', ');
                    unitValid = true;
                } else if (availableWeapons[unitName] && availableWeapons[unitName].length === 0) {
                    weaponInfo = 'No weapons available';
                    unitValid = true; // Units without weapons are valid
                }
                
                if (!unitValid) {
                    isValid = false;
                }
                
                unitDiv.innerHTML =
                    '<div>' +
                        '<strong>' + unitName + '</strong> ×' + quantity + '<br>' +
                        '<small style="color: ' + (unitValid ? '#aaa' : '#ff6666') + '">' + weaponInfo + '</small>' +
                    '</div>' +
                    '<div>' + (unitValid ? '✅' : '❌') + '</div>';
                
                summary.appendChild(unitDiv);
            });
            
            document.getElementById('confirmArmy').disabled = !isValid;
        }

        function confirmArmy() {
            const army = [];
            
            Object.keys(selectedUnits).forEach(unitName => {
                const quantity = selectedUnits[unitName];
                const weapons = selectedWeapons[unitName];
                
                army.push({
                    unit_name: unitName,
                    quantity: quantity,
                    weapon_type: weapons ? weapons.weaponType : ''
                });
            });
            
            sendMessage({
                type: 'select_army',
                army: army
            });
        }

        function showWaitingForBattle() {
            document.getElementById('battleInfo').innerHTML = '<p>Army deployed! Waiting for opponent...</p>';
        }

        function startBattle(message) {
            if (message.phase === 'initiative') {
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Battle vs ' + message.opponent + '</h3>' +
                    '<p style="color: #d4af37; font-weight: bold; font-size: 18px;">' + message.message + '</p>' +
                    '<div style="text-align: center; margin: 20px 0;">' +
                    '<button class="dice-btn" onclick="rollDice(6)" style="background: #d4af37; font-size: 16px; padding: 10px 20px;">Roll D6 for Initiative</button>' +
                    '</div>';
                addLogEntry('Battle begins: ' + message.message);
                updateStepIndicator(4);
            } else {
                document.getElementById('battleInfo').innerHTML = '<h3>Battle vs ' + message.opponent + '</h3><p>Battle has begun!</p>';
                addLogEntry('Battle started!');
            }
        }

        function showInitiativeRoll(message) {
            addLogEntry('Initiative: ' + message.player_name + ' rolled ' + message.result + ' for initiative');
        }

        function showInitiativeTie(message) {
            addLogEntry('Tie: ' + message.message);
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Initiative Roll</h3>' +
                '<p style="color: #ff6b6b; font-weight: bold;">' + message.message + '</p>' +
                '<div style="text-align: center; margin: 20px 0;">' +
                '<button class="dice-btn" onclick="rollDice(6)" style="background: #d4af37; font-size: 16px; padding: 10px 20px;">Roll D6 Again</button>' +
                '</div>';
        }

        function showInitiativeResolved(message) {
            addLogEntry('Initiative: ' + message.message);
            const turnIndicator = message.your_turn ? 
                '<p style="color: #00ff00; font-weight: bold; font-size: 18px;">YOUR TURN - Attack!</p>' :
                '<p style="color: #ff6b6b; font-weight: bold; font-size: 18px;">Opponent\'s Turn - Defend!</p>';
            
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Fight Phase</h3>' +
                '<p>' + message.message + '</p>' +
                turnIndicator +
                '<div style="text-align: center; margin: 20px 0;">' +
                '<p><strong>Combat Sequence:</strong></p>' +
                '<p>1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves</p>' +
                '</div>';
        }

        function updateBattleLog(message) {
            addLogEntry('Round ' + message.round + ': You dealt ' + message.damage_dealt + ' damage, received ' + message.damage_received + ' damage');
        }

        function finishBattle(message) {
            addLogEntry('Battle finished! Winner: ' + message.winner);
            document.getElementById('battleInfo').innerHTML += '<h3>Winner: ' + message.winner + '</h3>';
        }

        function addLogEntry(text) {
            const logContent = document.getElementById('logContent');
            const entry = document.createElement('p');
            entry.textContent = new Date().toLocaleTimeString() + ': ' + text;
            logContent.appendChild(entry);
            logContent.scrollTop = logContent.scrollHeight;
        }

        function rollDice(sides) {
            sendMessage({
                type: 'roll_dice',
                dice: sides
            });
        }

        function showDiceResult(dice, result) {
            document.getElementById('diceResults').innerHTML = 'Last roll: D' + dice + ' = ' + result;
        }

        function showOpponentDiceRoll(message) {
            addLogEntry(message.player_name + ' rolled D' + message.dice + ': ' + message.result);
        }

        // Event listeners
        document.getElementById('joinMatchmaking').onclick = joinMatchmaking;
        document.getElementById('playVsAI').onclick = playVsAI;
        document.getElementById('proceedToWeapons').onclick = proceedToWeapons;
        document.getElementById('confirmArmy').onclick = confirmArmy;
        
        // Add difficulty button listeners
        document.querySelectorAll('.difficulty-btn').forEach(btn => {
            btn.onclick = function() {
                // Remove selection from other buttons
                document.querySelectorAll('.difficulty-btn').forEach(b => b.classList.remove('selected'));
                // Add selection to clicked button
                this.classList.add('selected');
                // Select difficulty
                selectAIDifficulty(this.getAttribute('data-difficulty'));
            };
        });

        // Start the application
        loadSavedName();
        connect();
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
