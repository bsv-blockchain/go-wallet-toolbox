package tsgenerated

import (
	_ "embed"
)

//go:embed create_action_result.json
var createActionResultJSON string

func CreateActionResultJSON() string {
	return createActionResultJSON
}
