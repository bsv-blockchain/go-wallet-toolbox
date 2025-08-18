package fixtures

type ActionType string

const (
	ActionInternalize           ActionType = "internalize"
	ActionBalance               ActionType = "balance"
	ActionSendData              ActionType = "send_data"
	ActionSendDataPeriodically  ActionType = "send_data_periodically"
	ActionSendP2PKH             ActionType = "send_p2pkh"
	ActionSendP2PKHPeriodically ActionType = "send_p2pkh_periodically"
	ActionListOutputs           ActionType = "list_outputs"
)
