package telegrambot

import "sync"

type UserMode string

const (
	ModeNone         UserMode = ""
	ModeSupport      UserMode = "support"
	ModeSupportReply UserMode = "support_reply"
)

type UserState struct {
	Mode        UserMode
	ComplaintID uint
}

type StateStore struct {
	mu          sync.RWMutex
	instruction map[int64]int
	states      map[int64]UserState
}

func NewStateStore() *StateStore {
	return &StateStore{
		instruction: make(map[int64]int),
		states:      make(map[int64]UserState),
	}
}

func (s *StateStore) SetInstructionStep(tgID int64, step int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.instruction[tgID] = step
	s.mu.Unlock()
}

func (s *StateStore) GetInstructionStep(tgID int64) int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	step := s.instruction[tgID]
	s.mu.RUnlock()
	return step
}

func (s *StateStore) ClearInstruction(tgID int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.instruction, tgID)
	s.mu.Unlock()
}

func (s *StateStore) SetMode(tgID int64, mode UserMode) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if mode == ModeNone {
		delete(s.states, tgID)
	} else {
		s.states[tgID] = UserState{Mode: mode}
	}
	s.mu.Unlock()
}

func (s *StateStore) GetMode(tgID int64) UserMode {
	if s == nil {
		return ModeNone
	}
	s.mu.RLock()
	mode := s.states[tgID].Mode
	s.mu.RUnlock()
	return mode
}

func (s *StateStore) SetSupportReply(tgID int64, complaintID uint) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.states[tgID] = UserState{
		Mode:        ModeSupportReply,
		ComplaintID: complaintID,
	}
	s.mu.Unlock()
}

func (s *StateStore) GetState(tgID int64) UserState {
	if s == nil {
		return UserState{}
	}
	s.mu.RLock()
	state := s.states[tgID]
	s.mu.RUnlock()
	return state
}

func (s *StateStore) ClearMode(tgID int64) {
	s.SetMode(tgID, ModeNone)
}
