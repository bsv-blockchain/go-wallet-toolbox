package fixtures

type ActionType string

const (
	ActionInternalize    ActionType = "internalize"
	ActionBalance        ActionType = "balance"
	ActionSend           ActionType = "send"
	ActionListOutputs    ActionType = "list_outputs"
	ActionNoSendSendWith ActionType = "nosend_sendwith"
)
