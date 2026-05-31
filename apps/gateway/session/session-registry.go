package session

import "sync"

// GameSessionRegistry keeps the active owner for each gateway account and
// character. A duplicate login closes the previous sockets so the older session
// can unwind through the existing logout path.
type GameSessionRegistry interface {
	RegisterAccount(*GameSession)
	UnregisterAccount(*GameSession)
	RegisterCharacter(*GameSession, uint64)
	UnregisterCharacter(*GameSession, uint64)
	IsAccountSession(*GameSession) bool
	IsCharacterSession(*GameSession, uint64) bool
}

type SessionRegistry struct {
	mu                sync.Mutex
	accountSessions   map[uint32]*GameSession
	characterSessions map[uint64]*GameSession
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		accountSessions:   make(map[uint32]*GameSession),
		characterSessions: make(map[uint64]*GameSession),
	}
}

func (r *SessionRegistry) RegisterAccount(session *GameSession) {
	if r == nil || session == nil || session.accountID == 0 {
		return
	}

	var previous *GameSession
	r.mu.Lock()
	previous = r.accountSessions[session.accountID]
	r.accountSessions[session.accountID] = session
	r.mu.Unlock()

	if previous != nil && previous != session {
		previous.closeDuplicateSession("duplicate account session")
	}
}

func (r *SessionRegistry) UnregisterAccount(session *GameSession) {
	if r == nil || session == nil || session.accountID == 0 {
		return
	}

	r.mu.Lock()
	if r.accountSessions[session.accountID] == session {
		delete(r.accountSessions, session.accountID)
	}
	r.mu.Unlock()
}

func (r *SessionRegistry) RegisterCharacter(session *GameSession, characterGUID uint64) {
	if r == nil || session == nil || characterGUID == 0 {
		return
	}

	var previous *GameSession
	r.mu.Lock()
	previous = r.characterSessions[characterGUID]
	r.characterSessions[characterGUID] = session
	r.mu.Unlock()

	if previous != nil && previous != session {
		previous.closeDuplicateSession("duplicate character session")
	}
}

func (r *SessionRegistry) UnregisterCharacter(session *GameSession, characterGUID uint64) {
	if r == nil || session == nil || characterGUID == 0 {
		return
	}

	r.mu.Lock()
	if r.characterSessions[characterGUID] == session {
		delete(r.characterSessions, characterGUID)
	}
	r.mu.Unlock()
}

func (r *SessionRegistry) IsAccountSession(session *GameSession) bool {
	if r == nil || session == nil || session.accountID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accountSessions[session.accountID] == session
}

func (r *SessionRegistry) IsCharacterSession(session *GameSession, characterGUID uint64) bool {
	if r == nil || session == nil || characterGUID == 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.characterSessions[characterGUID] == session
}

func (s *GameSession) unregisterFromSessionRegistry() {
	if s.sessionRegistry == nil {
		return
	}

	if s.character != nil {
		s.sessionRegistry.UnregisterCharacter(s, s.character.GUID)
	}
	s.sessionRegistry.UnregisterAccount(s)
}

func (s *GameSession) closeDuplicateSession(reason string) {
	if s == nil {
		return
	}

	if s.logger != nil {
		s.logger.Warn().
			Uint32("account", s.accountID).
			Str("reason", reason).
			Msg("Closing superseded gateway session")
	}

	s.cancelSession()

	if s.worldSocket != nil {
		s.worldSocket.Close()
	}
	if s.gameSocket != nil {
		s.gameSocket.Close()
	}
}
