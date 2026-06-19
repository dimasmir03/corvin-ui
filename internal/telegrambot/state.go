package telegrambot

import "sync"

type UserMode string

const (
	ModeNone    UserMode = ""
	ModeSupport UserMode = "support"
)

type StateStore struct {
	mu          sync.RWMutex
	instruction map[int64]int
	modes       map[int64]UserMode
}

func NewStateStore() *StateStore {
	return &StateStore{
		instruction: make(map[int64]int),
		modes:       make(map[int64]UserMode),
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
		delete(s.modes, tgID)
	} else {
		s.modes[tgID] = mode
	}
	s.mu.Unlock()
}

func (s *StateStore) GetMode(tgID int64) UserMode {
	if s == nil {
		return ModeNone
	}
	s.mu.RLock()
	mode := s.modes[tgID]
	s.mu.RUnlock()
	return mode
}

func (s *StateStore) ClearMode(tgID int64) {
	s.SetMode(tgID, ModeNone)
}
