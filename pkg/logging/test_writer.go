package logging

import (
	"strings"
	"sync"

	"github.com/go-softwarelab/common/pkg/seq"
)

// TestWriter is a simple io.Writer implementation that writes to a string builder.
// It is useful for testing purposes - to check what was written by the logger.
type TestWriter struct {
	mu      sync.Mutex
	builder strings.Builder
}

// Write satisfies the io.Writer interface.
func (w *TestWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.builder.Write(p) //nolint:wrapcheck // no need to wrap the error for testing
}

// String returns the content written to the writer.
func (w *TestWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.builder.String()
}

// Lines returns the written content split into lines.
func (w *TestWriter) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return seq.Collect(strings.Lines(w.builder.String()))
}

// Clear resets the writer's buffer.
func (w *TestWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.builder.Reset()
}
