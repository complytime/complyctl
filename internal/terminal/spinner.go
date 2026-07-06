// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

type stopMsg struct{}

type spinnerModel struct {
	spinner spinner.Model
	message string
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(stopMsg); ok {
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() string {
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// Spinner renders an animated braille spinner using charmbracelet/bubbles.
// When the output writer is not a terminal, the spinner falls back to
// plain-text progress lines instead of escape sequences.
// Start() launches the animation; Stop() halts it and cleans up.
type Spinner struct {
	program *tea.Program // non-nil only when writing to a TTY
	writer  io.Writer    // non-nil only when writing to a non-TTY
	message string       // progress message for plain-text fallback
}

// NewSpinner creates a spinner that writes to stderr.
func NewSpinner(message string) *Spinner {
	return NewSpinnerWriter(message, os.Stderr)
}

// isWriterTTY reports whether w is backed by a terminal file descriptor.
func isWriterTTY(w io.Writer) bool {
	type fder interface{ Fd() uintptr }
	if f, ok := w.(fder); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

// NewSpinnerWriter creates a spinner that writes to w instead of stderr.
// When w is a terminal, an animated braille spinner is used. Otherwise,
// Start prints a plain-text progress line and Stop prints "done",
// consistent with the progress output of complyctl get.
// When w is nil, the spinner is fully silent (no output).
func NewSpinnerWriter(message string, w io.Writer) *Spinner {
	if w == nil {
		return &Spinner{}
	}
	if !isWriterTTY(w) {
		return &Spinner{writer: w, message: message}
	}
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	m := spinnerModel{spinner: s, message: message}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
	)
	return &Spinner{program: p}
}

// Start begins the spinner. On a TTY it launches the animated braille
// spinner, on a non-TTY it prints the progress message, and with a nil
// writer it is a silent no-op.
func (s *Spinner) Start() {
	if s.program != nil {
		go func() {
			_, _ = s.program.Run()
		}()
		return
	}
	if s.writer != nil {
		fmt.Fprintf(s.writer, "%s\n", s.message)
	}
}

// Stop halts the spinner. On a TTY it stops the animated program, on a
// non-TTY it prints "done", and with a nil writer it is a silent no-op.
func (s *Spinner) Stop() {
	if s.program != nil {
		s.program.Send(stopMsg{})
		s.program.Wait()
		return
	}
	if s.writer != nil {
		fmt.Fprintln(s.writer, "done")
	}
}
