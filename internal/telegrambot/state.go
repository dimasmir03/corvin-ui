package telegrambot

import "sync"

type StateStore struct {
	mu          sync.RWMutex
	instruction map[int64]int
}

func NewStateStore() *StateStore {
	return &StateStore{
		instruction: make(map[int64]int),
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
