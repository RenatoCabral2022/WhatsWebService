// Whats - WebRTC client POC
// Server creates offer, browser creates answer.

const API_BASE = 'http://localhost:8080/v1';

let pc = null;        // RTCPeerConnection
let dc = null;        // RTCDataChannel
let sessionId = null;
let localStream = null;
let ttsMarks = null;  // Current word marks from tts.marks event
let ttsPlaybackStart = 0;  // Timestamp when TTS audio playback began
let markTimers = [];  // setTimeout IDs for word highlighting
let ingestPollTimer = null;
let bufferCapSec = 60;
let waveformBars = [];  // Pre-seeded random bar heights for waveform look

async function connect() {
    try {
        updateStatus('Creating session...');

        // 1. Create session via REST — receive server's SDP offer
        const res = await fetch(`${API_BASE}/sessions`, { method: 'POST' });
        if (!res.ok) {
            updateStatus('Failed to create session');
            return;
        }
        const session = await res.json();
        sessionId = session.sessionId;
        updateStatus(`Session ${sessionId.slice(0, 8)}... created`);

        // 2. Create PeerConnection with ICE servers from server
        pc = new RTCPeerConnection({
            iceServers: session.iceServers || [
                { urls: 'stun:stun.l.google.com:19302' }
            ]
        });

        // 3. Receive server-created data channel
        pc.ondatachannel = (event) => {
            dc = event.channel;
            dc.onopen = () => updateStatus('Data channel open');
            dc.onmessage = (e) => {
                if (typeof e.data === 'string') {
                    handleServerMessage(JSON.parse(e.data));
                }
                // ignore binary messages
            };
        };

        // 4. Handle remote audio track from gateway
        pc.ontrack = (event) => {
            const audio = document.getElementById('remote-audio');
            if (audio && event.streams && event.streams[0]) {
                audio.srcObject = event.streams[0];
            }
        };

        pc.oniceconnectionstatechange = () => {
            console.log('ICE connection state:', pc.iceConnectionState);
        };

        // 5. Set server's offer as remote description FIRST (answerer model)
        updateStatus('Setting remote description...');
        await pc.setRemoteDescription({
            type: 'offer',
            sdp: session.sdpOffer
        });

        // 6. Get microphone audio and add tracks
        updateStatus('Requesting mic...');
        localStream = await navigator.mediaDevices.getUserMedia({
            audio: {
                channelCount: 1,
                echoCancellation: true,
                noiseSuppression: true
            }
        });
        localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

        // 7. Create answer
        updateStatus('Creating answer...');
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);

        // 8. Wait for ICE gathering to complete (with timeout)
        updateStatus('ICE gathering...');
        await new Promise((resolve) => {
            if (pc.iceGatheringState === 'complete') {
                resolve();
                return;
            }
            const timeout = setTimeout(resolve, 5000);
            pc.onicegatheringstatechange = () => {
                if (pc.iceGatheringState === 'complete') {
                    clearTimeout(timeout);
                    resolve();
                }
            };
        });

        // 9. Send SDP answer to server
        updateStatus('Sending answer...');
        const answerRes = await fetch(
            `${API_BASE}/sessions/${sessionId}/webrtc/answer`,
            {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ sdpAnswer: pc.localDescription.sdp })
            }
        );

        if (answerRes.ok) {
            updateStatus('Connected');
        } else {
            const errText = await answerRes.text();
            updateStatus(`Answer failed: ${errText}`);
            return;
        }

        document.getElementById('btn-enunciate').disabled = false;
        document.getElementById('btn-disconnect').disabled = false;
        document.getElementById('btn-ingest-start').disabled = false;

    } catch (err) {
        console.error('Connect error:', err);
        updateStatus(`Error: ${err.message}`);
    }
}

function enunciate() {
    if (!dc || dc.readyState !== 'open') return;

    const lookbackSeconds = parseInt(document.getElementById('lookback-slider').value, 10);
    const targetLang = document.getElementById('target-language').value || null;
    const msg = {
        type: 'command.enunciate',
        sessionId: sessionId,
        actionId: crypto.randomUUID(),
        timestamp: Date.now(),
        payload: {
            lookbackSeconds: lookbackSeconds,
            targetLanguage: targetLang,
            ttsOptions: { voice: 'default', speed: 1.0 }
        }
    };
    dc.send(JSON.stringify(msg));
}

function handleServerMessage(msg) {
    switch (msg.type) {
        case 'asr.partial':
            document.getElementById('transcript').innerText =
                `[partial] ${msg.payload.text}`;
            break;
        case 'asr.final':
            document.getElementById('transcript').innerText =
                `[${msg.payload.language}] ${msg.payload.text}`;
            if (msg.payload.translatedText) {
                // Show translated text as plain text until marks arrive
                document.getElementById('translation').innerText =
                    `[${msg.payload.targetLanguage}] ${msg.payload.translatedText}`;
            } else {
                document.getElementById('translation').innerText = '';
            }
            break;
        case 'tts.marks':
            ttsMarks = msg.payload;
            // Render words as individual spans for highlighting
            renderWordSpans(msg.payload);
            break;
        case 'tts.started':
            updateStatus('TTS playing...');
            // Ensure audio element is playing (browser may pause autoplay after silence)
            const audioEl = document.getElementById('remote-audio');
            if (audioEl && audioEl.paused) {
                audioEl.play().catch(() => {});
            }
            // Start word highlighting — marks are already available (sent before tts.started)
            startWordHighlighting();
            break;
        case 'tts.done':
            updateStatus('Connected');
            // Mark all words as spoken
            finalizeHighlighting();
            break;
        case 'metrics.latency':
            document.getElementById('metrics').innerText =
                `Latency: ${JSON.stringify(msg.payload)}`;
            break;
        case 'ingest.started':
            document.getElementById('ingest-status').innerText =
                `Ingesting: ${msg.payload.url}`;
            document.getElementById('btn-ingest-start').disabled = true;
            document.getElementById('btn-ingest-stop').disabled = false;
            startBufferPolling();
            break;
        case 'ingest.stopped':
            document.getElementById('ingest-status').innerText =
                `Ingest stopped (${msg.payload.reason})`;
            document.getElementById('btn-ingest-start').disabled = false;
            document.getElementById('btn-ingest-stop').disabled = true;
            stopBufferPolling();
            break;
        case 'error':
            console.error('Server error:', msg.payload);
            updateStatus(`Error: ${msg.payload.message}`);
            if (msg.payload.code === 'INGEST_FAILED') {
                document.getElementById('ingest-status').innerText =
                    `Ingest error: ${msg.payload.message}`;
                document.getElementById('btn-ingest-start').disabled = false;
                document.getElementById('btn-ingest-stop').disabled = true;
                stopBufferPolling();
            }
            break;
    }
}

function renderWordSpans(marks) {
    const el = document.getElementById('translation');
    // Build HTML with a span per word
    const spans = marks.words.map((w, i) =>
        `<span class="word" data-idx="${i}">${escapeHtml(w.word)}</span>`
    ).join(' ');
    el.innerHTML = spans;
}

function startWordHighlighting() {
    // Clear any previous timers
    markTimers.forEach(t => clearTimeout(t));
    markTimers = [];

    if (!ttsMarks || !ttsMarks.words) return;

    ttsPlaybackStart = performance.now();
    const words = ttsMarks.words;
    const el = document.getElementById('translation');
    const spans = el.querySelectorAll('.word');

    words.forEach((w, i) => {
        // Schedule highlighting at word start time
        const startTimer = setTimeout(() => {
            // Remove active from previous
            spans.forEach(s => s.classList.remove('active'));
            // Add active + spoken to current
            if (spans[i]) {
                spans[i].classList.add('active', 'spoken');
            }
        }, w.startMs);
        markTimers.push(startTimer);
    });

    // Remove active class after last word ends
    if (words.length > 0) {
        const lastEnd = words[words.length - 1].endMs;
        const endTimer = setTimeout(() => {
            spans.forEach(s => s.classList.remove('active'));
        }, lastEnd);
        markTimers.push(endTimer);
    }
}

function finalizeHighlighting() {
    markTimers.forEach(t => clearTimeout(t));
    markTimers = [];
    const el = document.getElementById('translation');
    el.querySelectorAll('.word').forEach(s => {
        s.classList.remove('active');
        s.classList.add('spoken');
    });
    ttsMarks = null;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

async function ingestStart() {
    if (!sessionId) return;
    const url = document.getElementById('ingest-url').value.trim();
    if (!url) {
        document.getElementById('ingest-status').innerText = 'Enter a URL first';
        return;
    }
    try {
        document.getElementById('btn-ingest-start').disabled = true;
        document.getElementById('ingest-status').innerText = 'Starting ingest...';
        const res = await fetch(
            `${API_BASE}/sessions/${sessionId}/ingest/start`,
            {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url: url })
            }
        );
        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: res.statusText }));
            document.getElementById('ingest-status').innerText =
                `Ingest failed: ${err.error || res.statusText}`;
            document.getElementById('btn-ingest-start').disabled = false;
        }
    } catch (err) {
        document.getElementById('ingest-status').innerText = `Error: ${err.message}`;
        document.getElementById('btn-ingest-start').disabled = false;
    }
}

async function ingestStop() {
    if (!sessionId) return;
    try {
        document.getElementById('btn-ingest-stop').disabled = true;
        await fetch(
            `${API_BASE}/sessions/${sessionId}/ingest/stop`,
            { method: 'POST' }
        );
    } catch (err) {
        console.error('Ingest stop error:', err);
    }
}

// ── Buffer visualization ──────────────────────────────────────

function seedWaveformBars(count) {
    waveformBars = [];
    for (let i = 0; i < count; i++) {
        // Pseudo-random heights that look like an audio waveform
        waveformBars.push(0.25 + Math.random() * 0.75);
    }
}

function startBufferPolling() {
    const viz = document.getElementById('buffer-viz');
    viz.style.display = 'block';
    seedWaveformBars(120); // 120 bars = 2 per second for 60s
    // Set canvas internal resolution to match display size
    const canvas = document.getElementById('buffer-canvas');
    canvas.width = canvas.offsetWidth * (window.devicePixelRatio || 1);
    canvas.height = 64 * (window.devicePixelRatio || 1);
    drawBuffer(0);
    pollIngestStatus(); // poll immediately, don't wait 800ms
    ingestPollTimer = setInterval(pollIngestStatus, 800);
}

function stopBufferPolling() {
    if (ingestPollTimer) {
        clearInterval(ingestPollTimer);
        ingestPollTimer = null;
    }
}

async function pollIngestStatus() {
    if (!sessionId) return;
    try {
        const res = await fetch(`${API_BASE}/sessions/${sessionId}/ingest/status`);
        if (!res.ok) {
            console.warn('ingest status poll failed:', res.status);
            return;
        }
        const data = await res.json();
        drawBuffer(data.secondsBuffered || 0);
    } catch (err) {
        console.warn('ingest status poll error:', err);
    }
}

function drawBuffer(bufferedSec) {
    const canvas = document.getElementById('buffer-canvas');
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    const W = canvas.width;
    const H = canvas.height;
    const totalBars = waveformBars.length;
    const barW = W / totalBars;
    const gap = dpr;
    const filledBars = Math.round((bufferedSec / bufferCapSec) * totalBars);
    const lookbackSec = parseInt(document.getElementById('lookback-slider').value, 10);
    const lookbackBars = Math.round((lookbackSec / bufferCapSec) * totalBars);

    // Lookback window: the last N seconds of what's buffered
    const lookbackStart = Math.max(0, filledBars - lookbackBars);

    // Background
    ctx.fillStyle = '#1a1a1a';
    ctx.fillRect(0, 0, W, H);

    // Draw bars
    for (let i = 0; i < totalBars; i++) {
        const barH = waveformBars[i] * (H - 8 * dpr);
        const x = i * barW;
        const y = (H - barH) / 2;

        if (i < filledBars) {
            if (i >= lookbackStart) {
                // Lookback window — bright cyan
                ctx.fillStyle = '#22d3ee';
            } else {
                // Filled but outside lookback — dimmer teal
                ctx.fillStyle = '#1a6b5a';
            }
        } else {
            // Unfilled — subtle
            ctx.fillStyle = '#2a2a2a';
        }
        ctx.fillRect(x + gap / 2, y, Math.max(barW - gap, 1), barH);
    }

    // Draw lookback bracket line at the boundary
    if (filledBars > 0 && lookbackStart > 0) {
        const bx = lookbackStart * barW;
        ctx.strokeStyle = 'rgba(34,211,238,0.6)';
        ctx.lineWidth = dpr;
        ctx.setLineDash([3 * dpr, 3 * dpr]);
        ctx.beginPath();
        ctx.moveTo(bx, 2 * dpr);
        ctx.lineTo(bx, H - 2 * dpr);
        ctx.stroke();
        ctx.setLineDash([]);
    }

    // Draw playhead at fill position
    if (filledBars > 0 && filledBars < totalBars) {
        const px = filledBars * barW;
        ctx.fillStyle = '#fff';
        ctx.fillRect(px - dpr / 2, 0, dpr, H);
    }

    // Update label
    const label = document.getElementById('buffer-label');
    const bSec = bufferedSec.toFixed(1);
    label.textContent = `${bSec}s buffered / ${bufferCapSec}s  ·  lookback ${lookbackSec}s`;
}

// ── Disconnect ────────────────────────────────────────────────

function disconnect() {
    stopBufferPolling();
    document.getElementById('buffer-viz').style.display = 'none';
    if (dc) dc.close();
    if (pc) pc.close();
    if (localStream) localStream.getTracks().forEach(t => t.stop());
    if (sessionId) {
        fetch(`${API_BASE}/sessions/${sessionId}`, { method: 'DELETE' });
    }
    const audio = document.getElementById('remote-audio');
    if (audio) audio.srcObject = null;
    sessionId = null;
    pc = null;
    dc = null;
    updateStatus('Disconnected');
    document.getElementById('btn-enunciate').disabled = true;
    document.getElementById('btn-disconnect').disabled = true;
    document.getElementById('btn-ingest-start').disabled = true;
    document.getElementById('btn-ingest-stop').disabled = true;
    document.getElementById('ingest-status').innerText = '';
}

function updateStatus(text) {
    document.getElementById('status').innerText = text;
}
