package wdk

import "io"

type HistoryNotes interface {
	List() []map[string]any
	PrettyPrint(writer io.Writer) error
}
