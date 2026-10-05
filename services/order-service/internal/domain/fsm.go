package domain

type Transition struct {
	From string
	To   string
}

type StateMachine struct {
	transitions map[string]map[string]bool
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		transitions: make(map[string]map[string]bool),
	}
}

func (sm *StateMachine) Allow(from, to string) *StateMachine {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[string]bool)
	}
	sm.transitions[from][to] = true
	return sm
}

func (sm *StateMachine) AllowMultiple(from string, to ...string) *StateMachine {
	for _, t := range to {
		sm.Allow(from, t)
	}
	return sm
}

func (sm *StateMachine) CanTransition(from, to string) bool {
	if sm.transitions[from] == nil {
		return false
	}
	return sm.transitions[from][to]
}

func (sm *StateMachine) GetAvailableTransitions(from string) []string {
	if sm.transitions[from] == nil {
		return []string{}
	}

	available := make([]string, 0, len(sm.transitions[from]))
	for to := range sm.transitions[from] {
		available = append(available, to)
	}
	return available
}

func (sm *StateMachine) IsFinal(status string) bool {
	transitions := sm.transitions[status]
	return len(transitions) == 0
}
