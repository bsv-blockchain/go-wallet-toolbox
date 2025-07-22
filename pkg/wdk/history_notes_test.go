package wdk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestMarshalAndUnmarshalHistoryNote(t *testing.T) {
	tests := map[string]struct {
		note         HistoryNote
		expectedJSON string
	}{
		"empty": {
			note:         HistoryNote{},
			expectedJSON: `{"when":"0001-01-01T00:00:00Z","what":""}`,
		},
		"'what' field only": {
			note: HistoryNote{
				What: "test event",
			},
			expectedJSON: `{"when":"0001-01-01T00:00:00Z","what":"test event"}`,
		},
		"with 'when' and 'what' fields": {
			note: HistoryNote{
				When: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				What: "test event with time",
			},
			expectedJSON: `{"when":"2023-10-01T12:00:00Z","what":"test event with time"}`,
		},
		"with 'when', 'what' and 'user_id' fields": {
			note: HistoryNote{
				When:   time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				What:   "test event with user",
				UserID: to.Ptr(1),
			},
			expectedJSON: `{"when":"2023-10-01T12:00:00Z","what":"test event with user","user_id":1}`,
		},
		"with 'when', 'what', 'user_id' and additional attributes": {
			note: HistoryNote{
				When:   time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				What:   "test event with attributes",
				UserID: to.Ptr(1),
				Attributes: map[string]any{
					"attribute1": "value1",
					"attribute2": 42,
					"attribute3": true,
				},
			},
			expectedJSON: `{"when":"2023-10-01T12:00:00Z","what":"test event with attributes","user_id":1,"attribute1":"value1","attribute2":42,"attribute3":true}`,
		},
		"with attribute that overrides 'what' field": {
			note: HistoryNote{
				When:   time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				What:   "test event with overridden when",
				UserID: to.Ptr(1),
				Attributes: map[string]any{
					"what":       "should not override",
					"attribute1": "value1",
				},
			},
			expectedJSON: `{"when":"2023-10-01T12:00:00Z","what":"test event with overridden when","user_id":1,"attribute1":"value1"}`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			jsonData, err := json.Marshal(test.note)

			// then:
			require.NoError(t, err)
			require.JSONEq(t, test.expectedJSON, string(jsonData))

			// when:
			var unmarshalledNote HistoryNote
			err = json.Unmarshal(jsonData, &unmarshalledNote)

			// then:
			require.NoError(t, err)
			require.Equal(t, test.note.When, unmarshalledNote.When)
		})
	}
}
