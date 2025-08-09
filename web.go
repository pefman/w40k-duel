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
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .weapon-name {
            font-weight: 500;
            color: #e5e5e5;
        }
        
        .weapon-status {
            font-size: 0.9em;
            color: #d4af37;
            font-weight: 600;
        }
        
        .weapon-item:hover {
            border-color: #d4af37;
            box-shadow: 0 4px 15px rgba(212, 175, 55, 0.2);
        }
        
        .weapon-item.selected {
            background: linear-gradient(135deg, rgba(212, 175, 55, 0.2) 0%, rgba(212, 175, 55, 0.1) 100%);
            border-color: #d4af37;
        }
        
        .weapon-item.selected .weapon-status {
            color: #4CAF50;
        }
        
        .weapon-category {
            margin-bottom: 20px;
        }
        
        .weapon-category h5 {
            color: #d4af37;
            margin: 0 0 10px 0;
            padding: 8px 12px;
            background: linear-gradient(135deg, #1a1a1a 0%, #0f0f0f 100%);
            border-radius: 6px;
            border: 1px solid #3d3d3d;
            font-size: 1em;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .weapon-individual-choice {
            margin: 10px 0;
        }
        
        .weapon-checkbox-container {
            background: #2a2a2a;
            border: 1px solid #555;
            border-radius: 5px;
            margin: 5px 0;
            padding: 10px;
            transition: all 0.3s ease;
        }
        
        .weapon-checkbox-container:hover {
            background: #333;
            border-color: #777;
        }
        
        .weapon-checkbox-label {
            display: flex;
            align-items: center;
            cursor: pointer;
            font-weight: bold;
            margin-bottom: 5px;
        }
        
        .weapon-checkbox {
            margin-right: 10px;
            transform: scale(1.2);
        }
        
        .weapon-name {
            flex-grow: 1;
            color: #fff;
        }
        
        .weapon-type-badge {
            background: #555;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 12px;
            margin-left: 10px;
        }
        
        .weapon-stats-small {
            font-size: 12px;
            color: #ccc;
            margin-left: 30px;
        }

        .weapon-type-options {
            display: flex;
            gap: 15px;
            margin-bottom: 20px;
        }
        
        .weapon-type-option {
            flex: 1;
            background: linear-gradient(135deg, #2d2d2d 0%, #1a1a1a 100%);
            border: 2px solid #3d3d3d;
            border-radius: 8px;
            padding: 20px;
            cursor: pointer;
            transition: all 0.3s ease;
            text-align: center;
            position: relative;
        }
        
        .weapon-type-option:hover {
            border-color: #d4af37;
            box-shadow: 0 4px 15px rgba(212, 175, 55, 0.2);
            transform: translateY(-2px);
        }
        
        .weapon-type-option.selected {
            background: linear-gradient(135deg, rgba(212, 175, 55, 0.3) 0%, rgba(212, 175, 55, 0.1) 100%);
            border-color: #d4af37;
            box-shadow: 0 6px 20px rgba(212, 175, 55, 0.3);
        }
        
        .weapon-type-option h4 {
            color: #d4af37;
            margin: 0 0 10px 0;
            font-size: 1.2em;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .weapon-type-option.selected h4 {
            color: #ffd700;
        }
        
        .weapon-type-option p {
            color: #b0b0b0;
            margin: 0;
            font-size: 0.9em;
            line-height: 1.4;
        }
        
        .weapon-type-option.selected p {
            color: #e5e5e5;
        }
        
        .weapon-list-preview {
            text-align: left;
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid rgba(212, 175, 55, 0.3);
        }
        
        .weapon-stat-line {
            margin-bottom: 12px;
            padding: 8px;
            background: rgba(0, 0, 0, 0.3);
            border-radius: 4px;
            border-left: 3px solid #d4af37;
        }
        
        .weapon-stat-line:last-child {
            margin-bottom: 0;
        }
        
        .weapon-stat-line strong {
            color: #d4af37;
            font-size: 1em;
        }
        
        .weapon-stats {
            color: #b0b0b0;
            font-size: 0.85em;
            font-family: 'Courier New', monospace;
            line-height: 1.3;
        }
        
        .weapon-stats span {
            white-space: nowrap;
        }
        
        .weapon-stats {
            display: block;
            margin-top: 4px;
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

        /* Combat System Styles */
        .unit-selection, .weapon-selection {
            display: flex;
            flex-wrap: wrap;
            gap: 15px;
            justify-content: center;
            margin: 20px 0;
        }

        .unit-card, .weapon-card {
            background: linear-gradient(135deg, #2d2d2d 0%, #3d3d3d 100%);
            border: 2px solid #d4af37;
            border-radius: 10px;
            padding: 15px;
            cursor: pointer;
            transition: all 0.3s ease;
            min-width: 200px;
            text-align: center;
        }

        .unit-card:hover, .weapon-card:hover {
            background: linear-gradient(135deg, #3d3d3d 0%, #4d4d4d 100%);
            transform: translateY(-3px);
            box-shadow: 0 8px 25px rgba(212, 175, 55, 0.3);
        }

        .dice-rolling {
            text-align: center;
            margin: 20px 0;
        }

        /* Enhanced styling for opponent defense phase */
        .dice-rolling.opponent-rolling {
            background: rgba(139, 0, 0, 0.1);
            border: 2px solid #8b0000;
            border-radius: 12px;
            padding: 20px;
            margin: 20px 0;
        }

        .rolling-animation {
            display: flex;
            justify-content: center;
            gap: 15px;
            margin: 15px 0;
        }

        .dice-icon {
            font-size: 2em;
            animation: rollDice 1.5s infinite ease-in-out;
        }

        .dice-icon:nth-child(2) {
            animation-delay: 0.3s;
        }

        .dice-icon:nth-child(3) {
            animation-delay: 0.6s;
        }

        @keyframes rollDice {
            0%, 100% { transform: translateY(0) rotate(0deg); }
            25% { transform: translateY(-10px) rotate(90deg); }
            50% { transform: translateY(0) rotate(180deg); }
            75% { transform: translateY(-5px) rotate(270deg); }
        }

        .dice-btn {
            background: linear-gradient(135deg, #d4af37 0%, #b8860b 100%);
            color: #0a0a0a;
            border: none;
            padding: 15px 30px;
            font-size: 1.2em;
            font-weight: 600;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .dice-btn:hover {
            background: linear-gradient(135deg, #ffd700 0%, #d4af37 100%);
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(212, 175, 55, 0.4);
        }

        /* Dice roll result styling */
        .dice-results {
            background: rgba(0, 0, 0, 0.3);
            border: 1px solid #d4af37;
            border-radius: 8px;
            padding: 15px;
            margin: 10px 0;
        }

        .dice-display {
            display: flex;
            gap: 8px;
            justify-content: center;
            margin: 10px 0;
            flex-wrap: wrap;
        }

        .dice-result {
            display: inline-block;
            width: 35px;
            height: 35px;
            line-height: 35px;
            text-align: center;
            border: 2px solid;
            border-radius: 6px;
            font-weight: bold;
            font-size: 1.2em;
        }

        .dice-result.success {
            background: rgba(0, 255, 0, 0.2);
            border-color: #00ff00;
            color: #00ff00;
        }

        .dice-result.fail {
            background: rgba(255, 0, 0, 0.2);
            border-color: #ff0000;
            color: #ff0000;
        }

        .result-summary {
            font-weight: bold;
            text-align: center;
            color: #d4af37;
            margin-top: 10px;
        }

        .weapon-attack, .attack-summary, .combat-results, .no-weapons {
            background: rgba(0, 0, 0, 0.2);
            border-left: 4px solid #d4af37;
            padding: 10px 15px;
            margin: 10px 0;
        }

        .weapon-attack h4 {
            color: #d4af37;
            margin-bottom: 5px;
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
            
            <div class="dice-roller" id="diceRoller" style="display: none;">
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
        let currentArmy = []; // Store the current army for combat

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

                case 'combat_phase_start':
                    showCombatPhase(message);
                    break;

                case 'combat_start':
                    showCombatStart(message);
                    break;

                case 'hit_phase':
                    showHitRollPhase(message);
                    break;

                case 'wound_phase':
                    showWoundRollPhase(message);
                    break;

                case 'save_phase':
                    showSaveRollPhase(message);
                    break;

                case 'hit_rolls':
                    showHitRollResults(message);
                    break;

                case 'wound_rolls':
                    showWoundRollResults(message);
                    break;

                case 'save_rolls':
                    showSaveRollResults(message);
                    break;

                case 'weapon_attack':
                    showWeaponAttack(message);
                    break;

                case 'attack_summary':
                    showAttackSummary(message);
                    break;

                case 'combat_results':
                    showCombatResults(message);
                    break;

                case 'no_weapons_selected':
                    showNoWeapons(message);
                    break;

                case 'combat_state':
                    updateCombatState(message);
                    break;

                case 'combat_waiting':
                    showCombatWaiting(message);
                    break;

                case 'hit_results':
                    showHitResults(message);
                    break;

                case 'wound_results':
                    showWoundResults(message);
                    break;

                case 'save_results':
                    showSaveResults(message);
                    break;

                case 'match_finished':
                    finishMatch(message);
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
                        '<button class="quantity-btn" onclick="adjustQuantity(\'' + unit.name.replace(/'/g, "\\'") + '\', -1)">-</button>' +
                        '<div class="quantity-display" id="qty-' + unit.name.replace(/\s+/g, '-') + '">0</div>' +
                        '<button class="quantity-btn" onclick="adjustQuantity(\'' + unit.name.replace(/'/g, "\\'") + '\', 1)">+</button>' +
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
            console.log('proceedToWeapons - selectedUnits:', selectedUnits);
            console.log('proceedToWeapons - availableUnits:', availableUnits);
            
            availableUnits.forEach(unit => {
                console.log('Processing unit:', unit.name, 'selected:', !!selectedUnits[unit.name], 'weapon_categories:', unit.weapon_categories);
                if (selectedUnits[unit.name]) {
                    availableWeapons[unit.name] = unit.weapon_categories || {melee: [], ranged: []};
                    console.log('Added weapons for', unit.name, ':', availableWeapons[unit.name]);
                }
            });
            
            console.log('Final availableWeapons:', availableWeapons);
            populateWeaponSelection();
            document.getElementById('unitSelection').classList.add('hidden');
            document.getElementById('weaponSelection').classList.remove('hidden');
            updateStepIndicator(3);
        }

        function populateWeaponSelection() {
            const configurator = document.getElementById('weaponConfigurator');
            configurator.innerHTML = '';

            console.log('populateWeaponSelection - availableWeapons:', availableWeapons);
            console.log('populateWeaponSelection - selectedUnits:', selectedUnits);

            Object.keys(selectedUnits).forEach(unitName => {
                const quantity = selectedUnits[unitName];
                const weaponCategories = availableWeapons[unitName] || {melee: [], ranged: []};
                
                console.log('Unit: ' + unitName + ', Quantity: ' + quantity + ', WeaponCategories:', weaponCategories);
                
                if ((weaponCategories.melee || []).length === 0 && (weaponCategories.ranged || []).length === 0) {
                    console.log('Skipping ' + unitName + ' - no weapons available');
                    return; // Skip units with no weapons
                }

                const unitDiv = document.createElement('div');
                unitDiv.className = 'unit-card';
                
                // Initialize weapon selection for this unit
                if (!selectedWeapons[unitName]) {
                    selectedWeapons[unitName] = {
                        selectedType: null, // 'melee' or 'ranged'
                        weapons: []
                    };
                }
                
                let weaponChoiceHtml = '<div class="weapon-individual-choice">';
                
                // Create individual weapon selection checkboxes
                let allWeapons = [...(weaponCategories.melee || []), ...(weaponCategories.ranged || [])];
                
                // Deduplicate weapons by name
                const weaponMap = new Map();
                allWeapons.forEach(weapon => {
                    weaponMap.set(weapon.name, weapon);
                });
                allWeapons = Array.from(weaponMap.values());
                
                if (allWeapons.length > 0) {
                    weaponChoiceHtml += '<h5>Select Weapons for Combat:</h5>';
                    
                    allWeapons.forEach(weapon => {
                        const weaponId = unitName + '_' + weapon.name.replace(/\s+/g, '_');
                        const isSelected = selectedWeapons[unitName] && 
                                         selectedWeapons[unitName].weapons.some(w => w.name === weapon.name);
                        
                        weaponChoiceHtml += '<div class="weapon-checkbox-container">';
                        weaponChoiceHtml += '<label class="weapon-checkbox-label">';
                        weaponChoiceHtml += '<input type="checkbox" class="weapon-checkbox" ';
                        weaponChoiceHtml += 'data-unit="' + unitName + '" ';
                        weaponChoiceHtml += 'data-weapon-name="' + weapon.name + '" ';
                        weaponChoiceHtml += (isSelected ? 'checked' : '') + '>';
                        weaponChoiceHtml += '<span class="weapon-name">' + weapon.name + '</span>';
                        weaponChoiceHtml += '<span class="weapon-type-badge">' + (weapon.type === 'Melee' ? '⚔️' : '🏹') + '</span>';
                        weaponChoiceHtml += '</label>';
                        weaponChoiceHtml += '<div class="weapon-stats-small">';
                        weaponChoiceHtml += 'A:' + weapon.attacks + ' | ';
                        weaponChoiceHtml += (weapon.type === 'Melee' ? 'WS:' : 'BS:') + weapon.skill + ' | ';
                        weaponChoiceHtml += 'S:' + weapon.strength + ' | ';
                        weaponChoiceHtml += 'AP:' + weapon.ap + ' | ';
                        weaponChoiceHtml += 'D:' + weapon.damage;
                        weaponChoiceHtml += '</div>';
                        weaponChoiceHtml += '</div>';
                    });
                } else {
                    weaponChoiceHtml += '<p>No weapons available for this unit.</p>';
                }
                
                weaponChoiceHtml += '</div>';
                
                unitDiv.innerHTML =
                    '<div class="unit-header">' +
                        '<div class="unit-name">' + unitName + ' ×' + quantity + '</div>' +
                    '</div>' +
                    '<div class="weapon-selection">' +
                        '<h4>Choose Combat Style:</h4>' +
                        weaponChoiceHtml +
                    '</div>';
                
                configurator.appendChild(unitDiv);
            });
            
            // Add event listeners for weapon checkboxes
            document.querySelectorAll('.weapon-checkbox').forEach(checkbox => {
                checkbox.addEventListener('change', function() {
                    const unitName = this.getAttribute('data-unit');
                    const weaponName = this.getAttribute('data-weapon-name');
                    toggleWeaponSelection(unitName, weaponName, this.checked);
                });
            });
            
            updateArmySummary();
        }

        function toggleWeaponSelection(unitName, weaponName, isSelected) {
            console.log('toggleWeaponSelection called with:', unitName, weaponName, isSelected);
            
            // Initialize weapon selection for this unit if needed
            if (!selectedWeapons[unitName]) {
                selectedWeapons[unitName] = {
                    weapons: []
                };
            }
            
            // Find all weapons for this unit from available weapons
            const weaponCategories = availableWeapons[unitName] || {melee: [], ranged: []};
            const allWeapons = [...(weaponCategories.melee || []), ...(weaponCategories.ranged || [])];
            
            // Find the specific weapon
            const weaponData = allWeapons.find(w => w.name === weaponName);
            if (!weaponData) {
                console.error('Weapon not found:', weaponName);
                return;
            }
            
            if (isSelected) {
                // Add weapon if not already present
                const alreadySelected = selectedWeapons[unitName].weapons.some(w => w.name === weaponName);
                if (!alreadySelected) {
                    selectedWeapons[unitName].weapons.push(weaponData);
                }
            } else {
                // Remove weapon
                selectedWeapons[unitName].weapons = selectedWeapons[unitName].weapons.filter(w => w.name !== weaponName);
            }
            
            console.log('Updated selectedWeapons for', unitName, ':', selectedWeapons[unitName]);
            updateArmySummary();
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
                
                if (weapons && weapons.weapons && weapons.weapons.length > 0) {
                    weaponInfo = 'Weapons: ' + weapons.weapons.join(', ');
                    unitValid = true;
                } else if (availableWeapons[unitName] && 
                          ((availableWeapons[unitName].melee && availableWeapons[unitName].melee.length === 0) &&
                           (availableWeapons[unitName].ranged && availableWeapons[unitName].ranged.length === 0))) {
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
                    selected_weapons: weapons ? weapons.weapons : []
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
            hideDiceRoller(); // Hide general dice roller
            
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
            hideDiceRoller(); // Hide general dice roller
            
            addLogEntry('Tie: ' + message.message);
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Initiative Roll</h3>' +
                '<p style="color: #ff6b6b; font-weight: bold;">' + message.message + '</p>' +
                '<div style="text-align: center; margin: 20px 0;">' +
                '<button class="dice-btn" onclick="rollDice(6)" style="background: #d4af37; font-size: 16px; padding: 10px 20px;">Roll D6 Again</button>' +
                '</div>';
        }

        function showInitiativeResolved(message) {
            hideDiceRoller(); // Hide general dice roller
            
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
            showDiceRoller(); // Show dice roller again after battle ends
            
            addLogEntry('Battle finished! Winner: ' + message.winner);
            document.getElementById('battleInfo').innerHTML += '<h3>Winner: ' + message.winner + '</h3>';
        }

        // New turn-based combat system functions
        function showCombatStart(message) {
            hideDiceRoller(); // Hide general dice roller during combat
            
            addLogEntry(message.message);
            
            if (message.phase === 'attacking') {
                // Show attack interface - let player select unit and weapon type
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Fight Phase</h3>' +
                    '<p style="color: #00ff00; font-weight: bold; font-size: 18px;">YOUR TURN - Attack!</p>' +
                    '<div style="text-align: center; margin: 20px 0;">' +
                    '<p><strong>Combat Sequence:</strong></p>' +
                    '<p>' + (message.combat_info ? message.combat_info.sequence[0] : '1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves') + '</p>' +
                    '</div>' +
                    '<div style="text-align: center; margin: 20px 0;">' +
                    '<button class="dice-btn" onclick="startAttack()" style="background: #ff4444; font-size: 16px; padding: 10px 20px;">Begin Attack!</button>' +
                    '</div>';
            } else {
                // Defending player
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Fight Phase</h3>' +
                    '<p style="color: #ff6b6b; font-weight: bold; font-size: 18px;">Opponent\'s Turn - Defend!</p>' +
                    '<div style="text-align: center; margin: 20px 0;">' +
                    '<p><strong>Combat Sequence:</strong></p>' +
                    '<p>' + (message.combat_info ? message.combat_info.sequence[0] : '1. Choose attacks → 2. Roll to hit → 3. Roll to wound → 4. Opponent rolls saves') + '</p>' +
                    '</div>' +
                    '<p>Waiting for opponent to attack...</p>';
            }
        }

        function startAttack() {
            console.log('startAttack function called');
            // Send message to start the combat sequence
            sendMessage({
                type: 'start_attack'
            });
            console.log('start_attack message sent');
        }

        function showCombatPhase(message) {
            hideDiceRoller(); // Hide general dice roller during combat
            
            if (message.your_turn) {
                // Store the army data for weapon selection
                currentArmy = message.army;
                
                let armyHTML = '<h3>Select Unit & Weapon to Attack</h3>';
                armyHTML += '<div class="unit-selection">';
                
                message.army.forEach(unit => {
                    // Escape single quotes in unit names for JavaScript
                    const escapedUnitName = unit.unit_name.replace(/'/g, "\\'");
                    armyHTML += '<div class="unit-card" onclick="selectAttackingUnit(\'' + escapedUnitName + '\')">';
                    armyHTML += '<h4>' + unit.unit_name + ' (x' + unit.quantity + ')</h4>';
                    armyHTML += '</div>';
                });
                
                armyHTML += '</div>';
                document.getElementById('battleInfo').innerHTML = armyHTML;
            } else {
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Opponent\'s Turn</h3>' +
                    '<p>' + message.message + '</p>' +
                    '<p>Enemy wounds remaining: ' + message.your_wounds + '</p>';
            }
            addLogEntry(message.message);
        }

        function selectAttackingUnit(unitName) {
            console.log('selectAttackingUnit called with:', unitName);
            console.log('currentArmy:', currentArmy);
            console.log('currentArmy length:', currentArmy ? currentArmy.length : 'null');
            
            // Find the unit's weapons from the current army data
            let unitData = null;
            if (currentArmy) {
                unitData = currentArmy.find(u => {
                    console.log('Checking unit:', u.unit_name, 'against:', unitName);
                    console.log('Unit weapons:', u.weapons);
                    return u.unit_name === unitName;
                });
            }
            
            console.log('Found unitData:', unitData);
            if (unitData && unitData.weapons) {
                console.log('Unit weapons array:', unitData.weapons);
                console.log('Weapons length:', unitData.weapons.length);
            }
            
            if (!unitData || !unitData.weapons || unitData.weapons.length === 0) {
                alert('No weapons available for this unit');
                return;
            }

            // Show weapon selection
            let weaponHTML = '<h3>Select Weapon for ' + unitName + '</h3>';
            weaponHTML += '<div class="weapon-selection">';
            
            unitData.weapons.forEach(weapon => {
                weaponHTML += '<div class="weapon-card" onclick="confirmAttack(\'' + unitName + '\', \'' + weapon.name + '\')">';
                weaponHTML += '<h4>' + weapon.name + '</h4>';
                weaponHTML += '<p>Attacks: ' + weapon.attacks + ', Skill: ' + weapon.skill + '+</p>';
                weaponHTML += '<p>S' + weapon.strength + ' AP' + weapon.ap + ' D' + weapon.damage + '</p>';
                weaponHTML += '</div>';
            });
            
            weaponHTML += '</div>';
            document.getElementById('battleInfo').innerHTML = weaponHTML;
        }

        function confirmAttack(unitName, weaponName) {
            sendMessage({
                type: 'select_attacking_unit',
                unit_name: unitName,
                weapon_name: weaponName
            });
        }

        function showHitRollPhase(message) {
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Hit Roll Phase</h3>' +
                '<p>' + message.message + '</p>' +
                '<p>Need ' + message.hit_on + '+ to hit</p>' +
                '<div class="dice-rolling">' +
                '<p>Roll ' + message.attacks + ' dice:</p>' +
                '<button class="dice-btn" onclick="rollHitDice(' + message.attacks + ')">Roll to Hit</button>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showWoundRollPhase(message) {
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Wound Roll Phase</h3>' +
                '<p>' + message.message + '</p>' +
                '<p>Need ' + message.wound_on + '+ to wound</p>' +
                '<div class="dice-rolling">' +
                '<p>Roll ' + message.hits + ' dice:</p>' +
                '<button class="dice-btn" onclick="rollWoundDice(' + message.hits + ')">Roll to Wound</button>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showSaveRollPhase(message) {
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Save Roll Phase</h3>' +
                '<p>' + message.message + '</p>' +
                '<p>Need ' + message.save_on + '+ to save</p>' +
                '<div class="dice-rolling">' +
                '<p>Roll ' + message.wounds + ' dice:</p>' +
                '<button class="dice-btn" onclick="rollSaveDice(' + message.wounds + ')">Roll Saves</button>' +
                '</div>';
            addLogEntry(message.message);
        }

        function rollHitDice(count) {
            const rolls = [];
            for (let i = 0; i < count; i++) {
                rolls.push(Math.floor(Math.random() * 6) + 1);
            }
            
            document.getElementById('battleInfo').innerHTML += 
                '<p>Your rolls: ' + rolls.join(', ') + '</p>';
            
            sendMessage({
                type: 'submit_hit_rolls',
                rolls: rolls
            });
        }

        function rollWoundDice(count) {
            const rolls = [];
            for (let i = 0; i < count; i++) {
                rolls.push(Math.floor(Math.random() * 6) + 1);
            }
            
            document.getElementById('battleInfo').innerHTML += 
                '<p>Your rolls: ' + rolls.join(', ') + '</p>';
            
            sendMessage({
                type: 'submit_wound_rolls',
                rolls: rolls
            });
        }

        function rollSaveDice(count) {
            const rolls = [];
            for (let i = 0; i < count; i++) {
                rolls.push(Math.floor(Math.random() * 6) + 1);
            }
            
            document.getElementById('battleInfo').innerHTML += 
                '<p>Your rolls: ' + rolls.join(', ') + '</p>';
            
            sendMessage({
                type: 'submit_save_rolls',
                rolls: rolls
            });
        }

        function showCombatWaiting(message) {
            // Enhanced visual feedback for opponent defense phase
            if (message.phase === 'enemy_saving') {
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Opponent Defense Phase</h3>' +
                    '<p>' + message.message + '</p>' +
                    '<div class="dice-rolling opponent-rolling">' +
                    '<p>Opponent is rolling armor saves...</p>' +
                    '<div class="rolling-animation">' +
                    '<span class="dice-icon">🎲</span>' +
                    '<span class="dice-icon">🎲</span>' +
                    '<span class="dice-icon">🎲</span>' +
                    '</div>' +
                    '</div>';
            } else {
                // Default waiting display for other phases
                document.getElementById('battleInfo').innerHTML = 
                    '<h3>Waiting</h3>' +
                    '<p>' + message.message + '</p>';
            }
            addLogEntry(message.message);
        }

        function updateCombatState(message) {
            if (message.combat) {
                addLogEntry('Combat update: Phase ' + message.combat.phase);
            }
        }

        // New functions to display dice roll results from backend
        function showHitRollResults(message) {
            const rollsDisplay = message.rolls.map(roll => 
                '<span class="dice-result ' + (roll >= message.hit_on ? 'success' : 'fail') + '">' + roll + '</span>'
            ).join(' ');
            
            document.getElementById('battleInfo').innerHTML += 
                '<div class="dice-results">' +
                '<h4>🎯 Hit Rolls</h4>' +
                '<p><strong>' + message.weapon + '</strong> (need ' + message.hit_on + '+)</p>' +
                '<div class="dice-display">' + rollsDisplay + '</div>' +
                '<p class="result-summary">' + message.hits + ' out of ' + message.rolls.length + ' hits!</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showWoundRollResults(message) {
            const rollsDisplay = message.rolls.map(roll => 
                '<span class="dice-result ' + (roll >= message.wound_on ? 'success' : 'fail') + '">' + roll + '</span>'
            ).join(' ');
            
            document.getElementById('battleInfo').innerHTML += 
                '<div class="dice-results">' +
                '<h4>💀 Wound Rolls</h4>' +
                '<p><strong>' + message.weapon + '</strong> (S' + message.strength + ' vs T' + message.toughness + ', need ' + message.wound_on + '+)</p>' +
                '<div class="dice-display">' + rollsDisplay + '</div>' +
                '<p class="result-summary">' + message.wounds + ' out of ' + message.rolls.length + ' wounds!</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showSaveRollResults(message) {
            const rollsDisplay = message.rolls.map(roll => 
                '<span class="dice-result ' + (roll >= message.save_on ? 'success' : 'fail') + '">' + roll + '</span>'
            ).join(' ');
            
            document.getElementById('battleInfo').innerHTML += 
                '<div class="dice-results">' +
                '<h4>🛡️ Armor Save Rolls</h4>' +
                '<p><strong>' + message.weapon + '</strong> (need ' + message.save_on + '+, AP-' + message.ap + ')</p>' +
                '<div class="dice-display">' + rollsDisplay + '</div>' +
                '<p class="result-summary">' + message.saves + ' saves, ' + message.unsaved_wounds + ' unsaved wounds, ' + message.damage + ' damage!</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showWeaponAttack(message) {
            document.getElementById('battleInfo').innerHTML += 
                '<div class="weapon-attack">' +
                '<h4>⚔️ ' + message.weapon + '</h4>' +
                '<p>' + message.unit + ' attacks with ' + message.attacks + ' attacks</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showAttackSummary(message) {
            let html = '<div class="attack-summary">' +
                '<h3>' + message.message + '</h3>';
            
            // Show weapon details
            if (message.weapons && message.weapons.length > 0) {
                html += '<div class="weapons-list">';
                message.weapons.forEach((weapon, index) => {
                    html += '<div class="weapon-summary">' +
                        '<strong>' + weapon.weapon_name + '</strong> (' + weapon.unit_name + ')' +
                        ' - ' + weapon.attacks + ' attacks' +
                        '</div>';
                });
                html += '</div>';
            }
            
            // Add action buttons if this player can act
            if (message.show_hit_button && message.show_action_buttons) {
                let buttonText = '🎯 Roll Hits';
                let rollFunction = 'rollHits()';
                
                // Update button based on current phase
                if (message.current_phase === 'wound') {
                    buttonText = '⚔️ Roll Wounds';
                    rollFunction = 'rollWounds()';
                } else if (message.current_phase === 'save') {
                    buttonText = '🛡️ Roll Saves';
                    rollFunction = 'rollSaves()';
                }
                
                html += '<div class="combat-actions" style="margin-top: 20px;">' +
                    '<button onclick="' + rollFunction + '" class="action-btn primary" style="background: #d4af37; color: #000; padding: 12px 24px; border: none; border-radius: 5px; font-weight: bold; cursor: pointer;">' + buttonText + '</button>' +
                    '</div>';
            }
            
            html += '</div>';
            
            document.getElementById('battleInfo').innerHTML = html;
            addLogEntry(message.message);
        }

        function rollHits() {
            sendMessage({
                type: 'roll_hit'
            });
        }

        function rollWounds() {
            sendMessage({
                type: 'roll_wound'
            });
        }

        function rollSaves() {
            sendMessage({
                type: 'roll_save'
            });
        }

        function showHitResults(message) {
            let html = '<div class="hit-results">' +
                '<h3>🎯 Hit Results</h3>';
            
            // Show attack history
            if (message.attack_history && message.attack_history.length > 0) {
                html += '<div class="attack-history">';
                message.attack_history.forEach(entry => {
                    if (entry.phase === 'hit') {
                        const rollsDisplay = entry.rolls.map(roll => 
                            '<span class="dice-result ' + (roll >= entry.target ? 'success' : 'fail') + '">' + roll + '</span>'
                        ).join(' ');
                        
                        html += '<div class="dice-results">' +
                            '<h4>' + entry.weapon_name + ' (' + entry.unit_name + ')</h4>' +
                            '<p>Need ' + entry.target + '+: ' + rollsDisplay + '</p>' +
                            '<p class="result-summary">' + entry.successes + ' hits!</p>' +
                            '</div>';
                    }
                });
                html += '</div>';
            }
            
            // Add action buttons if this player can act
            if (message.show_wound_button) {
                html += '<div class="combat-actions" style="margin-top: 20px;">' +
                    '<button onclick="rollWounds()" class="action-btn primary" style="background: #d4af37; color: #000; padding: 12px 24px; border: none; border-radius: 5px; font-weight: bold; cursor: pointer;">⚔️ Roll Wounds</button>' +
                    '</div>';
            } else if (message.show_next_weapon) {
                html += '<div class="combat-actions" style="margin-top: 20px;">' +
                    '<button onclick="nextWeapon()" class="action-btn secondary" style="background: #666; color: #fff; padding: 12px 24px; border: none; border-radius: 5px; font-weight: bold; cursor: pointer;">Next Weapon</button>' +
                    '</div>';
            }
            
            html += '</div>';
            
            document.getElementById('battleInfo').innerHTML = html;
            addLogEntry(message.message);
        }

        function showWoundResults(message) {
            let html = '<div class="wound-results">' +
                '<h3>⚔️ Wound Results</h3>';
            
            // Show attack history
            if (message.attack_history && message.attack_history.length > 0) {
                html += '<div class="attack-history">';
                message.attack_history.forEach(entry => {
                    if (entry.phase === 'wound') {
                        const rollsDisplay = entry.rolls.map(roll => 
                            '<span class="dice-result ' + (roll >= entry.target ? 'success' : 'fail') + '">' + roll + '</span>'
                        ).join(' ');
                        
                        html += '<div class="dice-results">' +
                            '<h4>' + entry.weapon_name + ' (' + entry.unit_name + ')</h4>' +
                            '<p>Need ' + entry.target + '+: ' + rollsDisplay + '</p>' +
                            '<p class="result-summary">' + entry.successes + ' wounds!</p>' +
                            '</div>';
                    }
                });
                html += '</div>';
            }
            
            // Add action buttons if this player can act
            if (message.show_save_button) {
                html += '<div class="combat-actions" style="margin-top: 20px;">' +
                    '<button onclick="rollSaves()" class="action-btn primary" style="background: #d4af37; color: #000; padding: 12px 24px; border: none; border-radius: 5px; font-weight: bold; cursor: pointer;">🛡️ Roll Saves</button>' +
                    '</div>';
            }
            
            html += '</div>';
            
            document.getElementById('battleInfo').innerHTML = html;
            addLogEntry(message.message);
        }

        function showSaveResults(message) {
            let html = '<div class="save-results">' +
                '<h3>🛡️ Save Results</h3>';
            
            // Show attack history
            if (message.attack_history && message.attack_history.length > 0) {
                html += '<div class="attack-history">';
                message.attack_history.forEach(entry => {
                    if (entry.phase === 'save') {
                        const rollsDisplay = entry.rolls.map(roll => 
                            '<span class="dice-result ' + (roll >= entry.target ? 'success' : 'fail') + '">' + roll + '</span>'
                        ).join(' ');
                        
                        html += '<div class="dice-results">' +
                            '<h4>Saves against ' + entry.weapon_name + '</h4>' +
                            '<p>Need ' + entry.target + '+: ' + rollsDisplay + '</p>' +
                            '<p class="result-summary">' + entry.successes + ' saves, ' + (entry.rolls.length - entry.successes) + ' failed!</p>' +
                            '</div>';
                    }
                });
                html += '</div>';
            }
            
            html += '</div>';
            
            document.getElementById('battleInfo').innerHTML = html;
            addLogEntry(message.message);
        }

        function nextWeapon() {
            sendMessage({
                type: 'next_weapon'
            });
        }

        function showCombatResults(message) {
            document.getElementById('battleInfo').innerHTML += 
                '<div class="combat-results">' +
                '<h3>⚡ Combat Results</h3>' +
                '<p>' + message.message + '</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function showNoWeapons(message) {
            document.getElementById('battleInfo').innerHTML += 
                '<div class="no-weapons">' +
                '<h4>⚠️ No Weapons</h4>' +
                '<p>' + message.message + '</p>' +
                '</div>';
            addLogEntry(message.message);
        }

        function finishMatch(message) {
            document.getElementById('battleInfo').innerHTML = 
                '<h3>Match Finished!</h3>' +
                '<p>Winner: ' + message.winner + '</p>' +
                '<button onclick="location.reload()">Play Again</button>';
            addLogEntry('Match finished! Winner: ' + message.winner);
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

        function showDiceRoller() {
            document.getElementById('diceRoller').style.display = 'block';
        }

        function hideDiceRoller() {
            document.getElementById('diceRoller').style.display = 'none';
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
