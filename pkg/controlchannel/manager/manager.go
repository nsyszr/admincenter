package manager

type SessionConfig struct {
	Relam          string
	SessionTimeout int
	PingInterval   int
	PongTimeout    int
	EventsTopic    string
}

type SessionManager interface {
	GetSessionConfig(realm string) (*SessionConfig, error)
	CreateSession(realm string, cfg *SessionConfig) (int32, error)
	ExistsSession(sessionID int32) (bool, error)
	ExistsSessionForRealm(realm string) (bool, error)
	RemoveSession(sessionID int32) error
	RenewSession(sessionID int32) error
	CleanupSessions() error
}
