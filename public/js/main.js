document.addEventListener('DOMContentLoaded', () => {
    // Atualiza a instrução de terminal de acordo com o host
    const terminalCmd = document.getElementById('terminal-cmd');
    if (terminalCmd) {
        terminalCmd.innerText = `curl -sL ${window.location.protocol}//${window.location.host}/play.sh | bash`;
    }

    const lobbyScreen = document.getElementById('lobby-screen');
    const gameUI = document.getElementById('game-ui');
    
    const playerNameInput = document.getElementById('player-name');
    const btnJoin = document.getElementById('btn-join');
    const joinContainer = document.getElementById('join-container');
    
    const bgAudio = document.getElementById('spectator-audio');
    const btnMute = document.getElementById('btn-mute');
    let isGlobalMuted = false;
    
    const radioStatus = document.getElementById('radio-status');
    const radioCountdown = document.getElementById('radio-countdown');
    const musicTimer = document.getElementById('music-timer');
    const nextSongInfo = document.getElementById('next-song-info');
    const lobbyLeaderboardList = document.getElementById('leaderboard-list');
    const gameLeaderboardList = document.getElementById('game-leaderboard-list');
    const gameNextSong = document.getElementById('game-next-song');
    const gameTimerHud = document.getElementById('game-timer-hud');
    const instructionsText = document.getElementById('instructions-text');
    
    const scoreEl = document.getElementById('score');
    const comboEl = document.getElementById('combo');
    const trackNameEl = document.getElementById('track-name');
    
    let engine = null;
    let ws = null;
    let myName = "Spectator";
    
    let currentServerState = "WAITING";
    let isPlayingLocal = false;
    let wantsToPlay = false;
    let countdownInterval = null;
    let musicTimerInterval = null;
    
    let serverStartTimeMs = 0;
    let serverDurationMs = 0;
    let serverNextSongTitle = "";

    function toggleMute() {
        isGlobalMuted = !isGlobalMuted;
        if (bgAudio) bgAudio.muted = isGlobalMuted;
        if (engine) engine.setMute(isGlobalMuted);
        if (btnMute) btnMute.innerText = isGlobalMuted ? "🔇 UNMUTE (M)" : "🔈 MUTE (M)";
    }

    if (btnMute) {
        btnMute.addEventListener('click', toggleMute);
    }

    // --- Configuração de Teclas ---
    let gameKeys = loadKeys();
    applyKeysToUI();
    
    function loadKeys() {
        const saved = localStorage.getItem('bitguitar_keys');
        if (saved) {
            try { return JSON.parse(saved); } catch(e) {}
        }
        return ['D', 'F', 'J', 'K'];
    }
    
    function saveKeys(keys) {
        localStorage.setItem('bitguitar_keys', JSON.stringify(keys));
    }
    
    function applyKeysToUI() {
        for (let i = 0; i < 4; i++) {
            const el = document.getElementById(`key-${i}`);
            if (el) el.value = gameKeys[i];
            const touchEl = document.getElementById(`touch-key-${i}`);
            if (touchEl) touchEl.textContent = gameKeys[i].toUpperCase();
        }
        instructionsText.innerHTML = `TECLAS: [${gameKeys.map(k=>k.toUpperCase()).join('] [')}]<br>Aperte a tecla quando o bit chegar na linha inferior.`;
    }
    
    document.getElementById('btn-save-keys').addEventListener('click', () => {
        const newKeys = [];
        for (let i = 0; i < 4; i++) {
            const val = document.getElementById(`key-${i}`).value.trim();
            if (!val) { alert(`Tecla da coluna ${i+1} está vazia!`); return; }
            newKeys.push(val);
        }
        gameKeys = newKeys;
        saveKeys(gameKeys);
        applyKeysToUI();
        const msg = document.getElementById('keys-saved-msg');
        msg.style.display = 'inline';
        setTimeout(() => { msg.style.display = 'none'; }, 2000);
    });

    // --- Conexão WebSocket ---
    connectWS();

    setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN && isPlayingLocal && engine && wantsToPlay) {
            ws.send(JSON.stringify({
                type: "SCORE",
                score: engine.score,
                combo: engine.combo
            }));
        }
    }, 500);

    btnJoin.addEventListener('click', () => {
        const val = playerNameInput.value.trim();
        if (!val) { alert("Digite sua assinatura primeiro!"); return; }
        myName = val;
        wantsToPlay = true;
        
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: "HELLO", name: myName }));
        }
        
        joinContainer.innerHTML = `<p style="color:#0f0;">[ STATUS: NA FILA COMO <b>${myName}</b> ]<br>Aguarde a próxima trilha para jogar.</p>`;
        
        const AudioContext = window.AudioContext || window.webkitAudioContext;
        const tempCtx = new AudioContext();
        tempCtx.resume();
    });

    function connectWS() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
        
        ws.onopen = () => {
            ws.send(JSON.stringify({ type: "HELLO", name: myName }));
        };
        
        ws.onmessage = (evt) => {
            const msg = JSON.parse(evt.data);
            if (msg.type === "STATE") {
                handleServerState(msg);
            } else if (msg.type === "LEADERBOARD") {
                updateLeaderboard(msg.leaderboard);
            }
        };
        
        ws.onclose = () => {
            radioStatus.innerText = "CONEXÃO PERDIDA COM A REDE CENTRAL";
            radioStatus.style.color = "red";
            // Reconectar após 3s
            setTimeout(connectWS, 3000);
        };
    }

    function formatTime(ms) {
        if (ms < 0) ms = 0;
        const s = Math.floor(ms / 1000);
        const m = Math.floor(s / 60);
        const sec = s % 60;
        return `${String(m).padStart(2,'0')}:${String(sec).padStart(2,'0')}`;
    }

    async function handleServerState(msg) {
        currentServerState = msg.state;
        serverNextSongTitle = msg.nextSong ? msg.nextSong.title : "";
        
        if (currentServerState === "WAITING") {
            if (bgAudio) {
                bgAudio.pause();
                bgAudio.currentTime = 0;
            }
            if (isPlayingLocal && engine) {
                engine.running = false;
                isPlayingLocal = false;
            }
            gameUI.classList.add('hidden');
            lobbyScreen.classList.remove('hidden');
            
            clearInterval(countdownInterval);
            clearInterval(musicTimerInterval);
            let timeMs = msg.countdownMs || 15000;
            radioStatus.innerText = "SISTEMA EM REPOUSO. PREPARANDO PRÓXIMO ALVO...";
            musicTimer.innerText = "";
            
            if (serverNextSongTitle) {
                nextSongInfo.innerText = `Próxima Trilha: ${serverNextSongTitle}`;
            } else {
                nextSongInfo.innerText = "";
            }
            
            radioCountdown.innerText = `Próxima invasão em: ${Math.ceil(timeMs/1000)}s`;
            countdownInterval = setInterval(() => {
                timeMs -= 1000;
                if (timeMs <= 0) timeMs = 0;
                radioCountdown.innerText = `Próxima invasão em: ${Math.ceil(timeMs/1000)}s`;
            }, 1000);

        } else if (currentServerState === "PLAYING") {
            clearInterval(countdownInterval);
            radioCountdown.innerText = "";
            
            serverStartTimeMs = msg.startTimeMs;
            serverDurationMs = msg.durationMs || 60000;
            
            // Timer no lobby (modo espectador)
            clearInterval(musicTimerInterval);
            musicTimerInterval = setInterval(() => {
                const elapsed = Date.now() - serverStartTimeMs;
                musicTimer.innerText = `${formatTime(elapsed)} / ${formatTime(serverDurationMs)}`;
                if (gameTimerHud) gameTimerHud.innerText = `${formatTime(elapsed)} / ${formatTime(serverDurationMs)}`;
            }, 500);
            
            if (serverNextSongTitle) {
                nextSongInfo.innerText = `Próxima Trilha: ${serverNextSongTitle}`;
                if (gameNextSong) gameNextSong.innerText = `Próxima: ${serverNextSongTitle}`;
            }
            
            if (wantsToPlay && !isPlayingLocal) {
                if (bgAudio) bgAudio.pause();
                isPlayingLocal = true;
                lobbyScreen.classList.add('hidden');
                gameUI.classList.remove('hidden');
                
                trackNameEl.innerText = msg.song.title;
                
                engine = new BitGuitarEngine('game-canvas', gameKeys);
                engine.setMute(isGlobalMuted);
                engine.durationMs = serverDurationMs;
                engine.onScoreUpdate = (score, combo) => {
                    scoreEl.innerText = String(score).padStart(6, '0');
                    comboEl.innerText = combo;
                };
                
                await engine.loadSong(msg.song.url, msg.song.beatmap);
                
                const now = Date.now();
                const diffMs = msg.startTimeMs - now;
                
                if (diffMs > 0) {
                    engine.ctx.fillStyle = '#000';
                    engine.ctx.fillRect(0,0,400,600);
                    engine.ctx.fillStyle = '#0f0';
                    engine.ctx.font = '20px "Share Tech Mono"';
                    engine.ctx.textAlign = 'center';
                    engine.ctx.fillText(`AGUARDANDO SYNC... (${Math.ceil(diffMs/1000)}s)`, 200, 300);
                    
                    setTimeout(() => { engine.start(0); }, diffMs);
                } else {
                    const offsetSec = Math.abs(diffMs) / 1000;
                    engine.start(offsetSec);
                }
            } else if (!wantsToPlay) {
                radioStatus.innerText = `EM OPERAÇÃO — ${msg.song.title}`;
                if (bgAudio && bgAudio.src !== window.location.origin + msg.song.url) {
                    bgAudio.src = msg.song.url;
                }
                const diffMs = msg.startTimeMs - Date.now();
                if (diffMs > 0) {
                    setTimeout(() => {
                        if (currentServerState === "PLAYING" && !wantsToPlay) {
                            bgAudio.currentTime = 0;
                            bgAudio.play().catch(e=>console.log("Audio play prevented:", e));
                        }
                    }, diffMs);
                } else {
                    bgAudio.currentTime = Math.abs(diffMs) / 1000;
                    bgAudio.play().catch(e=>console.log("Audio play prevented:", e));
                }
            }
        }
    }

    function updateLeaderboard(board) {
        if (!board) return;
        board.sort((a, b) => b.score - a.score);
        
        let html = '';
        let rank = 0;
        board.forEach((p) => {
            if (p.name === "Spectator" && p.score === 0) return;
            if (p.name === "Anon" && p.score === 0) return;
            rank++;
            const color = p.name === myName ? '#fff' : '#0f0';
            html += `<li style="color: ${color}"><span>${rank}. ${p.name}</span> <span>${String(p.score).padStart(6, '0')} [${p.combo}x]</span></li>`;
        });
        
        if (!html) html = '<li>Aguardando jogadores...</li>';
        lobbyLeaderboardList.innerHTML = html;
        gameLeaderboardList.innerHTML = html;
    }

    // --- ESC Modal ---
    const escModal = document.getElementById('esc-modal');
    const escYes = document.getElementById('esc-yes');
    const escNo = document.getElementById('esc-no');
    let escOpen = false;

    window.addEventListener('keydown', (e) => {
        if ((e.key === 'm' || e.key === 'M') && document.activeElement.tagName !== 'INPUT') {
            toggleMute();
            return;
        }
        if (e.key === 'Escape') {
            if (escOpen) {
                closeEscModal();
                return;
            }
            if (isPlayingLocal) {
                escOpen = true;
                escModal.classList.remove('hidden');
                escModal.style.display = 'flex';
                if (engine) engine.running = false;
            }
        }
    });

    escYes.addEventListener('click', () => {
        closeEscModal();
        // Voltar ao lobby
        if (engine) {
            engine.running = false;
            try { engine.audioSource.stop(); } catch(e) {}
        }
        isPlayingLocal = false;
        wantsToPlay = false;
        gameUI.classList.add('hidden');
        lobbyScreen.classList.remove('hidden');
        // Resetar join container
        joinContainer.innerHTML = `<p>Deseja invadir a próxima trilha?</p>
            <input type="text" id="player-name" maxlength="15" autocomplete="off" placeholder="Sua Assinatura" value="${myName !== 'Spectator' ? myName : ''}">
            <button id="btn-join" onclick="document.getElementById('btn-join')?.click()">ENTRAR NA FILA</button>`;
        // Re-bind btn
        const newBtn = document.getElementById('btn-join');
        if (newBtn) {
            newBtn.addEventListener('click', () => {
                const inp = document.getElementById('player-name');
                const val = inp ? inp.value.trim() : '';
                if (!val) { alert("Digite sua assinatura primeiro!"); return; }
                myName = val;
                wantsToPlay = true;
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ type: "HELLO", name: myName }));
                }
                joinContainer.innerHTML = `<p style="color:#0f0;">[ STATUS: NA FILA COMO <b>${myName}</b> ]<br>Aguarde a próxima trilha para jogar.</p>`;
            });
        }
    });

    escNo.addEventListener('click', () => {
        closeEscModal();
        if (engine) engine.running = true;
        if (engine) engine.loop();
    });

    function closeEscModal() {
        escOpen = false;
        escModal.classList.add('hidden');
        escModal.style.display = 'none';
    }
});
