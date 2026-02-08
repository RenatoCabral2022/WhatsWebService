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
        case 'error':
            console.error('Server error:', msg.payload);
            updateStatus(`Error: ${msg.payload.message}`);
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

function disconnect() {
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
}

function updateStatus(text) {
    document.getElementById('status').innerText = text;
}
