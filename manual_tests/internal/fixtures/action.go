package fixtures

type ActionType string

const (
	ActionInternalize ActionType = "internalize"
	ActionBalance     ActionType = "balance"
	ActionSendData    ActionType = "send_data"
)
