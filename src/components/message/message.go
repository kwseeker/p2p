package message

import "github.com/pion/webrtc/v4"

const (
    TypeRegisterRequest = iota + 1
    TypeRegisterResponse
    TypeHeartbeatRequest
    TypeHeartbeatResponse
    TypeSdpRequest
    TypeSdpResponse
    TypeCandidateRequest
    TypeCandidateResponse
)

type MMeta struct {
    Type int `json:"type"` // 消息类型
}

type RegisterBody struct {
    Cid      string `json:"cid"`      // Peer ID，比如远程控制场景每个可控终端都有一个唯一ID
    AuthCode string `json:"authCode"` // 认证码，比如远程控制场景密码认证
}

type RegisterRequest struct {
    MMeta
    RegisterBody
}

func NewRegisterRequest(cid, authCode string) RegisterRequest {
    return RegisterRequest{
        MMeta: MMeta{
            Type: TypeRegisterRequest,
        },
        RegisterBody: RegisterBody{
            Cid:      cid,
            AuthCode: authCode,
        },
    }
}

type RegisterResponse struct {
    MMeta
    RegisterBody
    Success bool `json:"success"`
}

func NewRegisterResponse(registerRequest RegisterRequest, success bool) RegisterResponse {
    return RegisterResponse{
        MMeta: MMeta{
            Type: TypeRegisterResponse,
        },
        RegisterBody: registerRequest.RegisterBody,
        Success:      success,
    }
}

//type HeartBeatMessage struct {
//    MMeta
//}
//
//func NewHeartBeatMessage() *HeartBeatMessage {
//    return &HeartBeatMessage{
//        MMeta: MMeta{
//            Type: TypeHeartbeatRequest,
//        },
//    }
//}

type SdpBody struct {
    Sd       webrtc.SessionDescription `json:"sd"`       // SDP
    From     string                    `json:"from"`     // 来源 Peer ID
    To       string                    `json:"to"`       // 目标 Peer ID
    AuthCode string                    `json:"authCode"` // 目标 Peer 的 AuthCode
}

type SdpRequest struct {
    MMeta
    SdpBody
}

func NewSdpRequest(sd webrtc.SessionDescription, from, to, authCode string) SdpRequest {
    return SdpRequest{
        MMeta: MMeta{
            Type: TypeSdpRequest,
        },
        SdpBody: SdpBody{
            Sd:       sd,
            From:     from,
            To:       to,
            AuthCode: authCode,
        },
    }
}

type SdpResponse struct {
    MMeta
    SdpBody
    Success bool `json:"success"`
}

func NewSdpResponse(sdpRequest SdpRequest, success bool) SdpResponse {
    return SdpResponse{
        MMeta: MMeta{
            Type: TypeSdpResponse,
        },
        SdpBody: sdpRequest.SdpBody,
    }
}

type CandidateBody struct {
    Candidate string `json:"candidate"`
    From      string `json:"from"` // 来源 Peer ID
    To        string `json:"to"`   // 目标 Peer ID
}

type CandidateRequest struct {
    MMeta
    CandidateBody
}

func NewCandidateRequest(candidate, from, to string) CandidateRequest {
    return CandidateRequest{
        MMeta: MMeta{
            Type: TypeCandidateRequest,
        },
        CandidateBody: CandidateBody{
            Candidate: candidate,
            From:      from,
            To:        to,
        },
    }
}

type CandidateResponse struct {
    MMeta
    CandidateBody
    Success bool `json:"success"`
}

func NewCandidateResponse(candidateRequest CandidateRequest, success bool) CandidateResponse {
    return CandidateResponse{
        MMeta: MMeta{
            Type: TypeCandidateResponse,
        },
        CandidateBody: candidateRequest.CandidateBody,
        Success:       success,
    }
}
