package cli

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func newTestModel(exists []string) *tuiModel {
	c := fakeCat{}
	m := &tuiModel{
		ctx: context.Background(),
		s: Session{
			Components: c, Presets: c,
			Exists: func(string) []string { return exists },
			Write: func(dir string, r *Rendered) ([]string, error) {
				return []string{dir + "/compose.yaml"}, nil
			},
		},
		defs: map[string]*domain.ComponentDef{},
	}
	m.answers = []componentAnswers{{ID: "db", Params: []paramAnswer{{Name: "DB_PASSWORD", Type: "scalar", Required: true, Value: "typed-secret"}}}}
	m.outDir = "out"
	m.width, m.height = 100, 30
	return m
}

func TestTUIErrorKeepsAnswersAndReturnsToParams(t *testing.T) {
	m := newTestModel(nil)
	m.stage = stWorking
	m.Update(renderedMsg{err: errs.PortConflictError{Port: "80/tcp"}})
	if m.stage != stError || m.errMsg == "" {
		t.Fatalf("stage=%v errMsg=%q", m.stage, m.errMsg)
	}
	m.Update(key("enter"))
	if m.stage != stParams || m.paramsForm == nil {
		t.Fatalf("stage=%v", m.stage)
	}
	if m.answers[0].Params[0].Value != "typed-secret" {
		t.Fatal("answers were lost on the way back")
	}
}

func TestTUIUnexpectedErrorQuits(t *testing.T) {
	m := newTestModel(nil)
	boom := errors.New("boom")
	_, cmd := m.Update(renderedMsg{err: boom})
	if cmd == nil || m.stage != stDone || !errors.Is(m.err, boom) {
		t.Fatalf("stage=%v err=%v", m.stage, m.err)
	}
}

func TestTUIPreviewWriteAndOverwriteConfirmation(t *testing.T) {
	r := &Rendered{Compose: []byte("services: {}\n"), Env: []byte("A=1\n"), Warnings: []string{"w"}}

	// Nothing exists: 'w' writes immediately.
	m := newTestModel(nil)
	m.Update(renderedMsg{r: r})
	if m.stage != stPreview {
		t.Fatalf("stage=%v", m.stage)
	}
	_, cmd := m.Update(key("w"))
	if cmd == nil {
		t.Fatal("expected a write command")
	}
	m.Update(cmd())
	if len(m.written) != 1 || m.err != nil || m.stage != stDone {
		t.Fatalf("written=%v err=%v", m.written, m.err)
	}

	// Existing files: ask first; 'n' returns to the preview, 'y' writes.
	m = newTestModel([]string{"out/.env"})
	m.Update(renderedMsg{r: r})
	if _, cmd := m.Update(key("w")); cmd != nil || m.stage != stOverwrite {
		t.Fatalf("stage=%v", m.stage)
	}
	m.Update(key("n"))
	if m.stage != stPreview {
		t.Fatalf("stage=%v", m.stage)
	}
	m.Update(key("w"))
	if _, cmd := m.Update(key("y")); cmd == nil {
		t.Fatal("y should write")
	}
}

func TestTUIQuitAndAbort(t *testing.T) {
	m := newTestModel(nil)
	m.Update(renderedMsg{r: &Rendered{Compose: []byte("x")}})
	m.Update(key("q"))
	if !errors.Is(m.err, ErrAborted) {
		t.Fatalf("err=%v", m.err)
	}
	m = newTestModel(nil)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !errors.Is(m.err, ErrAborted) {
		t.Fatalf("ctrl+c err=%v", m.err)
	}
}

func TestTUILoadCmdFromComponents(t *testing.T) {
	m := newTestModel(nil)
	m.mode, m.picked = "components", []string{"db"}
	msg := m.loadCmd()().(loadedMsg)
	if msg.err != nil || len(msg.answers) != 1 || msg.answers[0].ID != "db" || len(msg.answers[0].Params) != 2 {
		t.Fatalf("%+v", msg)
	}
	m.picked = []string{"nope"}
	if msg := m.loadCmd()().(loadedMsg); !errors.Is(msg.err, errs.ErrNotFound) {
		t.Fatalf("want not found, got %v", msg.err)
	}
}
