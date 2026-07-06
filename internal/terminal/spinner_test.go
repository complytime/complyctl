// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpinnerModelView(t *testing.T) {
	m := spinnerModel{}
	m.spinner.Spinner.Frames = []string{"⠋"}
	m.message = "loading..."

	view := m.View()
	assert.True(t, strings.Contains(view, "loading..."),
		"expected spinner view to contain message, got: %q", view)
}

func TestSpinnerModelStopMsg(t *testing.T) {
	m := spinnerModel{}
	m.message = "test"

	updated, cmd := m.Update(stopMsg{})
	require.NotNil(t, updated)
	require.NotNil(t, cmd)

	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	assert.True(t, isQuit, "expected tea.Quit command on stopMsg")
}

func TestSpinnerModelTickAdvancesFrame(t *testing.T) {
	m := spinnerModel{}
	m.spinner.Spinner.Frames = []string{"A", "B", "C"}
	m.message = "working"

	view1 := m.View()
	updated, _ := m.Update(m.spinner.Tick())
	view2 := updated.(spinnerModel).View()

	assert.Contains(t, view1, "working")
	assert.Contains(t, view2, "working")
}

func TestNewSpinnerWriter_NilWriter(t *testing.T) {
	s := NewSpinnerWriter("test msg", nil)
	require.NotNil(t, s)
	assert.Nil(t, s.program, "nil writer should produce a silent spinner")
	assert.Nil(t, s.writer, "nil writer should not set fallback writer")

	// Start and Stop must be safe no-ops.
	s.Start()
	s.Stop()
}

func TestNewSpinnerWriter_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinnerWriter("test msg", &buf)
	require.NotNil(t, s)
	assert.Nil(t, s.program,
		"non-TTY writer should not create a bubbletea program")
	assert.NotNil(t, s.writer,
		"non-TTY writer should set fallback writer")
}

func TestSpinner_NonTTY_PlainOutput(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinnerWriter("Generating policy artifacts...", &buf)

	s.Start()
	output := buf.String()
	assert.Equal(t, "Generating policy artifacts...\n", output,
		"Start should print message followed by newline")

	s.Stop()
	output = buf.String()
	assert.Equal(t,
		"Generating policy artifacts...\ndone\n", output,
		"Stop should append done followed by newline")
}

func TestSpinner_NonTTY_NoEscapeSequences(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinnerWriter("Scanning targets...", &buf)

	s.Start()
	s.Stop()

	output := buf.String()
	assert.NotContains(t, output, "\x1b",
		"non-TTY output must not contain ANSI escape sequences")
	assert.NotContains(t, output, "\x1b[",
		"non-TTY output must not contain CSI escape sequences")
}

func TestIsWriterTTY_Buffer(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, isWriterTTY(&buf),
		"bytes.Buffer should not be detected as a TTY")
}

func TestIsWriterTTY_Nil(t *testing.T) {
	assert.False(t, isWriterTTY(nil),
		"nil writer should not be detected as a TTY")
}
