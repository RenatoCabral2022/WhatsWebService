// Whats - WebRTC client POC
// Server creates offer, browser creates answer.

const API_BASE = 'http://localhost:8080/v1';

let pc = null;        // RTCPeerConnection
let dc = null;        // RTCDataChannel
let sessionId = null;
let localStream = null;

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
                document.getElementById('translation').innerText =
                    `[${msg.payload.targetLanguage}] ${msg.payload.translatedText}`;
            } else {
                document.getElementById('translation').innerText = '';
            }
            break;
        case 'tts.started':
            updateStatus('TTS streaming...');
            break;
        case 'tts.done':
            updateStatus('Connected');
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
