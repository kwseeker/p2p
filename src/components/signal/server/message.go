package server

import "github.com/pion/webrtc/v4"

const (
	TypeRegister = iota + 1
	TypeHeartbeat
	TypeSdp
	TypeCandidate
)

type Message struct {
	Type int `json:"type"` // 消息类型
}

type RegisterMessage struct {
	Message
	Cid      string `json:"cid"`      // Peer ID，比如远程控制场景每个可控终端都有一个唯一ID
	AuthCode string `json:"authCode"` // 认证码，比如远程控制场景密码认证
}

func NewRegisterMessage(cid, authCode string) *RegisterMessage {
	return &RegisterMessage{
		Message: Message{
			Type: TypeRegister,
		},
		Cid:      cid,
		AuthCode: authCode,
	}
}

type HeartBeatMessage struct {
	Message
}

func NewHeartBeatMessage() *HeartBeatMessage {
	return &HeartBeatMessage{
		Message: Message{
			Type: TypeHeartbeat,
		},
	}
}

type SdpMessage struct {
	Message
	Sd       webrtc.SessionDescription `json:"sd"`       // SDP
	From     string                    `json:"from"`     // 来源 Peer ID
	To       string                    `json:"to"`       // 目标 Peer ID
	AuthCode string                    `json:"authCode"` // 目标 Peer 的 AuthCode
}

type CandidateMessage struct {
	Message
	Candidate string `json:"candidate"`
	From      string `json:"from"` // 来源 Peer ID
	To        string `json:"to"`   // 目标 Peer ID
}

type RegisterResponse struct {
	RegisterMessage
	Success bool `json:"success"`
}

func NewRegisterResponse(registerMessage RegisterMessage, success bool) RegisterResponse {
	return RegisterResponse{
		RegisterMessage: registerMessage,
		Success:         success,
	}
}

type SdpResponse struct {
	SdpMessage
	Success bool `json:"success"`
}

func NewSdpResponse(sdpMessage SdpMessage, success bool) SdpResponse {
	return SdpResponse{
		SdpMessage: sdpMessage,
		Success:    success,
	}
}

type CandidateResponse struct {
	CandidateMessage
	Success bool `json:"success"`
}

func NewCandidateResponse(CandidateMessage CandidateMessage, success bool) CandidateResponse {
	return CandidateResponse{
		CandidateMessage: CandidateMessage,
		Success:          success,
	}
}
