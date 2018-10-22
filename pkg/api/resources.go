package api

import (
	"time"
)

const (
	resourceTypeSession     = "SessionV100"
	resourceTypeSessionList = "SessionListV100"
)

type SessionDetails struct {
	Realm          string    `json:"realm"`
	ConnectedSince time.Time `json:"connectedSince"`
	LastMessage    time.Time `json:"lastMessage"`
	SessionTimeout int       `json:"sessionTimeout"`
	PingInterval   int       `json:"pingInterval"`
	PongTimeout    int       `json:"pongTimeout"`
}

type ActiveSessions struct {
	Active []*SessionDetails `json:"active"`
	Total  int               `json:"total"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SessionRunRequest struct {
	Realms  []string `json:"realms"`
	Command string   `json:"command"`
}

type SessionRunResponse struct {
	Realms map[string]SessionRunRealmResponse `json:"realms"`
}

type SessionRunRealmResponse struct {
	Success bool           `json:"success"`
	Error   *ErrorResponse `json:"error,omitempty"`
	Results interface{}    `json:"results,omitempty"`
}

type RPCM3CLIResult struct {
	Output []string `json:"output"`
}
