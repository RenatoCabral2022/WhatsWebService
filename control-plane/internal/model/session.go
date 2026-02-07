package model

type CreateSessionResponse struct {
	SessionID  string      `json:"sessionId"`
	SdpOffer   string      `json:"sdpOffer"`
	IceServers []IceServer `json:"iceServers"`
}

type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type WebRTCAnswerRequest struct {
	SdpAnswer string `json:"sdpAnswer"`
}
