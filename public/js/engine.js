class BitGuitarEngine {
    constructor(canvasId, customKeys) {
        this.canvas = document.getElementById(canvasId);
        this.ctx = this.canvas.getContext('2d');
        this.width = this.canvas.width;
        this.height = this.canvas.height;
        
        this.audioCtx = null;
        this.audioSource = null;
        this.gainNode = null;
        this.startTime = 0;
        this.durationMs = 0;
        
        this.trackX = [50, 150, 250, 350];
        this.hitY = this.height - 50;
        this.speed = 0.5;
        
        this.notes = [];
        this.activeNotes = [];
        this.particles = [];
        
        this.score = 0;
        this.combo = 0;
        
        this.running = false;
        this.onGameOver = null;
        this.onScoreUpdate = null;
        this.onTimeUpdate = null;
        
        // Teclas personalizáveis
        this.keyLabels = customKeys || ['D', 'F', 'J', 'K'];
        this.keyMap = {};
        this.keyLabels.forEach((k, i) => {
            this.keyMap[k.toLowerCase()] = i;
            this.keyMap[k.toUpperCase()] = i;
        });
        
        this.bindEvents();
    }
    
    bindEvents() {
        window.addEventListener('keydown', (e) => {
            if (!this.running) return;
            const lane = this.keyMap[e.key];
            if (lane !== undefined) {
                this.handleHit(lane);
            }
        });
        
        const zones = document.querySelectorAll('.touch-zone');
        zones.forEach(zone => {
            zone.addEventListener('touchstart', (e) => {
                e.preventDefault();
                if (!this.running) return;
                const lane = parseInt(zone.getAttribute('data-lane'));
                this.handleHit(lane);
            });
        });
    }
    
    handleHit(lane) {
        if (!this.audioCtx) return;
        const currentTime = (this.audioCtx.currentTime - this.startTime) * 1000;
        
        let hitNoteIndex = -1;
        let minDiff = 200;
        
        for (let i = 0; i < this.activeNotes.length; i++) {
            const note = this.activeNotes[i];
            if (note.lane === lane && !note.hit) {
                const diff = Math.abs(note.time - currentTime);
                if (diff < minDiff) {
                    minDiff = diff;
                    hitNoteIndex = i;
                }
            }
        }
        
        if (hitNoteIndex !== -1) {
            this.activeNotes[hitNoteIndex].hit = true;
            let points = 50;
            if (minDiff < 50) points = 300;
            else if (minDiff < 100) points = 100;
            
            this.combo++;
            this.score += points * (1 + Math.floor(this.combo / 10));
            this.drawFeedback(lane, "HIT");
        } else {
            this.combo = 0;
            this.drawFeedback(lane, "MISS");
        }
        
        if (this.onScoreUpdate) this.onScoreUpdate(this.score, this.combo);
    }
    
    drawFeedback(lane, text) {
        this.particles.push({
            x: this.trackX[lane],
            y: this.hitY - 20,
            text: text,
            life: 1.0,
            vx: 0,
            vy: -1.5,
            type: 'text'
        });
        
        if (text !== "MISS") {
            for (let i = 0; i < 8; i++) {
                this.particles.push({
                    x: this.trackX[lane] + (Math.random() - 0.5) * 30,
                    y: this.hitY + (Math.random() - 0.5) * 20,
                    text: Math.random() > 0.5 ? '1' : '0',
                    life: 1.0,
                    vx: (Math.random() - 0.5) * 6,
                    vy: (Math.random() - 0.5) * 6 - 2,
                    type: 'bit'
                });
            }
        }
    }
    
    async loadSong(url, beatmap) {
        this.notes = [...beatmap];
        this.activeNotes = [...beatmap];
        
        this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        this.gainNode = this.audioCtx.createGain();
        this.gainNode.connect(this.audioCtx.destination);
        
        const response = await fetch(url);
        const arrayBuffer = await response.arrayBuffer();
        const audioBuffer = await this.audioCtx.decodeAudioData(arrayBuffer);
        
        this.audioSource = this.audioCtx.createBufferSource();
        this.audioSource.buffer = audioBuffer;
        this.audioSource.connect(this.gainNode);
        
        this.audioSource.onended = () => {
            this.running = false;
            if (this.onGameOver) this.onGameOver(this.score);
        };
    }
    
    start(offsetSec = 0) {
        this.score = 0;
        this.combo = 0;
        if (this.onScoreUpdate) this.onScoreUpdate(this.score, this.combo);
        
        this.running = true;
        this.startTime = this.audioCtx.currentTime - offsetSec;
        
        try {
            this.audioSource.start(0, offsetSec);
        } catch (e) {
            console.error("Audio already started or failed", e);
        }
        
        this.loop();
    }
    
    getCurrentTimeMs() {
        if (!this.audioCtx) return 0;
        return (this.audioCtx.currentTime - this.startTime) * 1000;
    }
    
    loop() {
        if (!this.running) return;
        
        this.update();
        this.draw();
        
        requestAnimationFrame(() => this.loop());
    }
    
    update() {
        const currentTime = (this.audioCtx.currentTime - this.startTime) * 1000;
        
        for (let i = 0; i < this.activeNotes.length; i++) {
            const note = this.activeNotes[i];
            if (!note.hit && (currentTime - note.time) > 200) {
                note.hit = true;
                this.combo = 0;
                if (this.onScoreUpdate) this.onScoreUpdate(this.score, this.combo);
            }
        }
        
        this.particles.forEach(p => {
            p.x += p.vx;
            p.y += p.vy;
            p.life -= 0.03;
        });
        this.particles = this.particles.filter(p => p.life > 0);
        
        // Fire time update callback
        if (this.onTimeUpdate) {
            this.onTimeUpdate(currentTime, this.durationMs);
        }
    }
    
    draw() {
        this.ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
        this.ctx.fillRect(0, 0, this.width, this.height);
        
        // Lanes
        this.ctx.strokeStyle = 'rgba(0, 255, 0, 0.2)';
        this.ctx.lineWidth = 1;
        for (let i = 1; i < 4; i++) {
            this.ctx.beginPath();
            this.ctx.moveTo(i * 100, 0);
            this.ctx.lineTo(i * 100, this.height);
            this.ctx.stroke();
        }
        
        // Target line
        this.ctx.strokeStyle = '#0f0';
        this.ctx.lineWidth = 2;
        this.ctx.beginPath();
        this.ctx.moveTo(0, this.hitY);
        this.ctx.lineTo(this.width, this.hitY);
        this.ctx.stroke();
        
        // Keys
        this.ctx.fillStyle = '#0f0';
        this.ctx.font = '20px "Share Tech Mono"';
        this.ctx.textAlign = 'center';
        for (let i = 0; i < 4; i++) {
            this.ctx.fillText(this.keyLabels[i].toUpperCase(), this.trackX[i], this.hitY + 30);
        }
        
        // Notes
        if (!this.audioCtx) return;
        const currentTime = (this.audioCtx.currentTime - this.startTime) * 1000;
        
        this.ctx.font = '24px "Share Tech Mono"';
        
        this.activeNotes.forEach(note => {
            const timeDiff = note.time - currentTime;
            const y = this.hitY - (timeDiff * this.speed);
            
            if (y > -50 && y < this.height + 50) {
                if (note.hit) {
                    this.ctx.fillStyle = 'rgba(0, 255, 0, 0.2)';
                } else {
                    this.ctx.fillStyle = note.type === 0 ? '#0a0' : '#0f0';
                }
                this.ctx.fillText(note.type === 0 ? '0' : '1', this.trackX[note.lane], y);
            }
        });
        
        // Particles
        this.particles.forEach(p => {
            this.ctx.globalAlpha = p.life;
            if (p.type === 'text') {
                this.ctx.font = 'bold 24px "Share Tech Mono"';
                this.ctx.fillStyle = p.text === 'MISS' ? '#f00' : (p.text === 'HIT' ? '#0f0' : '#ff0');
                this.ctx.fillText(p.text, p.x, p.y);
            } else {
                this.ctx.font = 'bold 16px "Share Tech Mono"';
                this.ctx.fillStyle = '#0f0';
                this.ctx.fillText(p.text, p.x, p.y);
            }
        });
        this.ctx.globalAlpha = 1.0;
    }
    
    setMute(isMuted) {
        if (this.gainNode) {
            this.gainNode.gain.value = isMuted ? 0 : 1;
        }
    }
}
