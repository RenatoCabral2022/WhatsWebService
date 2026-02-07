// Whats - WebRTC client POC
// Minimal reference implementation for browser-based audio DVR.

const API_BASE = 'http://localhost:8080/v1';

let pc = null;        // RTCPeerConnection
let dc = null;        // RTCDataChannel
let sessionId = null;
let localStream = null;

async function connect() {
    // 1. Create session via REST
    const res = await fetch(`${API_BASE}/sessions`, { method: 'POST' });
    const session = await res.json();
    sessionId = session.sessionId;

    // 2. Create PeerConnection
    pc = new RTCPeerConnection({
        iceServers: session.iceServers || [
            { urls: 'stun:stun.l.google.com:19302' }
        ]
    });

    // 3. Create data channel for commands/events
    dc = pc.createDataChannel('commands', { ordered: true });
    dc.onopen = () => updateStatus('Data channel open');
    dc.onmessage = (e) => handleServerMessage(JSON.parse(e.data));

    // 4. Get microphone audio
    localStream = await navigator.mediaDevices.getUserMedia({
        audio: {
            sampleRate: 16000,
            channelCount: 1,
            echoCancellation: true,
            noiseSuppression: true
        }
    });

    localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

    // 5. Create offer
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    // 6. TODO: exchange SDP with server via POST /v1/sessions/{id}/webrtc/answer

    updateStatus('Connected');
    document.getElementById('btn-enunciate').disabled = false;
    document.getElementById('btn-disconnect').disabled = false;
}

function enunciate(lookbackSeconds = 5) {
    if (!dc || dc.readyState !== 'open') return;

    const msg = {
        type: 'command.enunciate',
        sessionId: sessionId,
        actionId: crypto.randomUUID(),
        timestamp: Date.now(),
        payload: {
            lookbackSeconds: lookbackSeconds,
            targetLanguage: null,
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
                `[final] ${msg.payload.text}`;
            break;
        case 'tts.started':
            updateStatus('TTS streaming...');
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

function disconnect() {
    if (dc) dc.close();
    if (pc) pc.close();
    if (localStream) localStream.getTracks().forEach(t => t.stop());
    if (sessionId) {
        fetch(`${API_BASE}/sessions/${sessionId}`, { method: 'DELETE' });
    }
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
