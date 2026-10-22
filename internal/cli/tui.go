package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

type stage int

const (
	stSetup     stage = iota // choose preset or components
	stLoading                // fetching definitions
	stParams                 // edit parameters, one form group per component
	stWorking                // generating
	stError                  // recoverable generation error
	stPreview                // scrollable result
	stOverwrite              // confirm replacing existing files
	stDone
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#6C50FF")).Padding(0, 1)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))
)

// Messages produced by background commands.
type (
	loadedMsg struct {
		answers []componentAnswers
		preset  string
		err     error
	}
	renderedMsg struct {
		r   *Rendered
		err error
	}
	writtenMsg struct {
		paths []string
		err   error
	}
)

type tuiModel struct {
	ctx context.Context
	s   Session

	stage         stage
	width, height int

	presetList []domain.PresetSummary
	compList   []domain.ComponentSummary

	// setup step
	setupForm *huh.Form
	mode      string
	presetID  string
	picked    []string

	// parameter step; answers keep their values when the user navigates back
	answers    []componentAnswers
	outDir     string
	paramsForm *huh.Form
	defs       map[string]*domain.ComponentDef

	// results
	rendered *Rendered
	vp       viewport.Model
	errMsg   string
	written  []string
	err      error
}

// RunTUI runs the full-screen interactive session and returns the written paths.
func RunTUI(ctx context.Context, s Session) ([]string, error) {
	m := &tuiModel{ctx: ctx, s: s, outDir: s.OutDir, defs: map[string]*domain.ComponentDef{}}
	var perr, cerr error
	m.presetList, perr = s.Presets.ListPresets(ctx)
	m.compList, cerr = s.Components.ListComponents(ctx)
	for _, e := range []error{perr, cerr} {
		if e != nil && s.ErrOut != nil {
			fmt.Fprintln(s.ErrOut, "warning:", e)
		}
	}
	if len(m.compList) == 0 {
		return nil, errs.InvalidRequestError{Reason: "no components available"}
	}
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
			return nil, ErrAborted
		}
		return nil, err
	}
	fm := final.(*tuiModel)
	return fm.written, fm.err
}

func (m *tuiModel) Init() tea.Cmd {
	m.setupForm = m.newSetupForm()
	return m.setupForm.Init()
}

func (m *tuiModel) newSetupForm() *huh.Form {
	m.mode = "components"
	if len(m.presetList) > 0 {
		m.mode = "preset" // a select starts on the option matching its value
	}
	presetOpts := make([]huh.Option[string], len(m.presetList))
	for i, p := range m.presetList {
		presetOpts[i] = huh.NewOption(fmt.Sprintf("%s — %s", p.ID, p.Description), p.ID)
	}
	compOpts := make([]huh.Option[string], len(m.compList))
	for i, c := range m.compList {
		compOpts[i] = huh.NewOption(fmt.Sprintf("%s — %s", c.ID, c.Description), c.ID)
	}
	return huh.NewForm(
		huh.NewGroup(huh.NewSelect[string]().Title("How do you want to start?").
			Options(huh.NewOption("From a preset", "preset"), huh.NewOption("Pick components myself", "components")).
			Value(&m.mode)).WithHideFunc(func() bool { return len(m.presetList) == 0 }),
		huh.NewGroup(huh.NewSelect[string]().Title("Preset").Options(presetOpts...).Value(&m.presetID)).
			WithHideFunc(func() bool { return m.mode != "preset" }),
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Components").Description("space or x toggles, enter confirms").
			Options(compOpts...).Value(&m.picked).
			Validate(func(s []string) error {
				if len(s) == 0 {
					return errors.New("select at least one component")
				}
				return nil
			})).WithHideFunc(func() bool { return m.mode != "components" }),
	).WithShowHelp(true)
}

// loadCmd fetches the chosen preset and component definitions.
func (m *tuiModel) loadCmd() tea.Cmd {
	presetID, picked := m.presetIDForRequest(), append([]string(nil), m.picked...)
	return func() tea.Msg {
		byComp := map[string]map[string]any{}
		ids := picked
		if presetID != "" {
			pr, err := m.s.Presets.GetPreset(m.ctx, presetID)
			if err != nil {
				return loadedMsg{err: err}
			}
			ids = nil
			for _, sv := range pr.Services {
				ids = append(ids, sv.Component)
				byComp[sv.Component] = sv.Params
			}
		}
		var answers []componentAnswers
		for _, id := range ids {
			def, err := m.s.Components.GetComponent(m.ctx, id)
			if err != nil {
				return loadedMsg{err: err}
			}
			ca := componentAnswers{ID: id}
			for _, p := range def.Params {
				pre := prefillFor(p, byComp[id])
				ca.Params = append(ca.Params, paramAnswer{Name: p.Name, Type: p.Type, Required: p.Required, Prefill: pre, Value: pre})
			}
			answers = append(answers, ca)
		}
		return loadedMsg{answers: answers, preset: presetID}
	}
}

// newParamsForm has one group per component so shift+tab walks back across
// components; bound pointers keep the user's input across rebuilds.
func (m *tuiModel) newParamsForm() *huh.Form {
	var groups []*huh.Group
	for i := range m.answers {
		ca := &m.answers[i]
		if len(ca.Params) == 0 {
			continue
		}
		fields := make([]huh.Field, len(ca.Params))
		for j := range ca.Params {
			pa := &ca.Params[j]
			title := pa.Name
			if pa.Required {
				title += " (required)"
			}
			in := huh.NewInput().Title(title).Value(&pa.Value)
			if d := m.paramDescription(ca.ID, pa.Name); d != "" {
				in = in.Description(d)
			}
			if pa.Type == "list" {
				in = in.Placeholder("comma-separated, optional")
			}
			if isSecret(pa.Name) {
				in = in.EchoMode(huh.EchoModePassword)
			}
			if pa.Required {
				in = in.Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("required")
					}
					return nil
				})
			}
			fields[j] = in
		}
		groups = append(groups, huh.NewGroup(fields...).Title(ca.ID))
	}
	groups = append(groups, huh.NewGroup(
		huh.NewInput().Title("Output directory").Value(&m.outDir),
	).Title("Output"))
	return huh.NewForm(groups...).WithShowHelp(true)
}

func (m *tuiModel) paramDescription(comp, name string) string {
	if def := m.defs[comp]; def != nil {
		return paramDescription(def, name)
	}
	return ""
}

func (m *tuiModel) request() domain.GenerateRequest {
	return buildWizardRequest(m.presetIDForRequest(), m.answers)
}

func (m *tuiModel) presetIDForRequest() string {
	if m.mode == "components" {
		return ""
	}
	return m.presetID
}

func (m *tuiModel) renderCmd() tea.Cmd {
	req := m.request()
	return func() tea.Msg {
		r, err := m.s.Render(m.ctx, req)
		return renderedMsg{r: r, err: err}
	}
}

func (m *tuiModel) writeCmd() tea.Cmd {
	dir, r := m.outDir, m.rendered
	return func() tea.Msg {
		paths, err := m.s.Write(dir, r)
		return writtenMsg{paths: paths, err: err}
	}
}

func (m *tuiModel) quit(err error) (tea.Model, tea.Cmd) {
	m.err = err
	m.stage = stDone
	return m, tea.Quit
}

func (m *tuiModel) goParams() tea.Cmd {
	m.stage = stParams
	m.paramsForm = m.newParamsForm()
	cmds := []tea.Cmd{m.paramsForm.Init()}
	if m.width > 0 { // a fresh form has not seen the window size yet
		_, c := m.paramsForm.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width, m.vp.Height = msg.Width, max(msg.Height-6, 3)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.quit(ErrAborted)
		}
	case loadedMsg:
		if msg.err != nil {
			return m.quit(msg.err)
		}
		m.answers = msg.answers
		for _, ca := range m.answers {
			if def, err := m.s.Components.GetComponent(m.ctx, ca.ID); err == nil {
				m.defs[ca.ID] = def
			}
		}
		return m, m.goParams()
	case renderedMsg:
		switch {
		case msg.err == nil:
			m.rendered = msg.r
			m.stage = stPreview
			m.vp = viewport.New(m.width, max(m.height-6, 3))
			m.vp.SetContent(string(msg.r.Compose))
		case errors.Is(msg.err, errs.ErrInvalidInput) || errors.Is(msg.err, errs.ErrConflict):
			m.errMsg = msg.err.Error()
			m.stage = stError
		default:
			return m.quit(msg.err)
		}
		return m, nil
	case writtenMsg:
		m.written = msg.paths
		return m.quit(msg.err)
	}

	switch m.stage {
	case stSetup:
		return m.updateForm(msg, m.setupForm, func() tea.Cmd {
			m.stage = stLoading
			return m.loadCmd()
		})
	case stParams:
		return m.updateForm(msg, m.paramsForm, func() tea.Cmd {
			m.stage = stWorking
			return m.renderCmd()
		})
	case stError:
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "enter", "b":
				return m, m.goParams()
			case "q", "esc":
				return m.quit(ErrAborted)
			}
		}
	case stPreview:
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "w", "enter":
				if len(m.s.Exists(m.outDir)) > 0 {
					m.stage = stOverwrite
					return m, nil
				}
				return m, m.writeCmd()
			case "b":
				return m, m.goParams()
			case "e":
				m.vp.SetContent(string(m.rendered.Env))
				return m, nil
			case "c":
				m.vp.SetContent(string(m.rendered.Compose))
				return m, nil
			case "q", "esc":
				return m.quit(ErrAborted)
			}
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	case stOverwrite:
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "y":
				return m, m.writeCmd()
			case "n", "esc":
				m.stage = stPreview
			}
		}
	}
	return m, nil
}

// updateForm drives an embedded huh form and calls next when it completes.
func (m *tuiModel) updateForm(msg tea.Msg, f *huh.Form, next func() tea.Cmd) (tea.Model, tea.Cmd) {
	model, cmd := f.Update(msg)
	if nf, ok := model.(*huh.Form); ok {
		f = nf
	}
	switch f.State {
	case huh.StateCompleted:
		return m, next()
	case huh.StateAborted:
		return m.quit(ErrAborted)
	}
	return m, cmd
}

func (m *tuiModel) View() string {
	header := titleStyle.Render("Compose Assembler")
	var body, help string
	switch m.stage {
	case stSetup:
		body = m.setupForm.View()
	case stLoading, stWorking:
		body = dimStyle.Render("Working…")
	case stParams:
		body = m.paramsForm.View()
	case stError:
		body = errStyle.Render("Cannot generate") + "\n\n" + m.errMsg
		help = "enter/b: back to parameters (your values are kept) • q: quit"
	case stPreview:
		body = m.vp.View()
		for _, w := range m.rendered.Warnings {
			body += "\n" + warnStyle.Render("warning: "+w)
		}
		help = fmt.Sprintf("↑/↓ scroll • c compose • e .env • w write to %s • b back • q quit", m.outDir)
	case stOverwrite:
		body = warnStyle.Render(strings.Join(m.s.Exists(m.outDir), "\n") + "\n\nalready exist. Overwrite?")
		help = "y: overwrite • n: back"
	}
	out := header + "\n\n" + body
	if help != "" {
		out += "\n\n" + dimStyle.Render(help)
	}
	return out + "\n"
}
