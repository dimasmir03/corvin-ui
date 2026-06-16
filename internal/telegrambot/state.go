package telegrambot

type StateStore struct {
	// reserved for instruction/support flows
}

func NewStateStore() *StateStore {
	return &StateStore{}
}
