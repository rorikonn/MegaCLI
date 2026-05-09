// Package app wires together services, coordinates agents, and manages
// application lifecycle.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/megacli/megacli/internal/agent"
	"github.com/megacli/megacli/internal/agent/notify"
	"github.com/megacli/megacli/internal/agent/tools/mcp"
	"github.com/megacli/megacli/internal/askuser"
	"github.com/megacli/megacli/internal/config"
	"github.com/megacli/megacli/internal/db"
	"github.com/megacli/megacli/internal/event"
	"github.com/megacli/megacli/internal/filetracker"
	"github.com/megacli/megacli/internal/format"
	"github.com/megacli/megacli/internal/history"
	"github.com/megacli/megacli/internal/ipc"
	"github.com/megacli/megacli/internal/log"
	"github.com/megacli/megacli/internal/lsp"
	"github.com/megacli/megacli/internal/megatool"
	"github.com/megacli/megacli/internal/message"
	"github.com/megacli/megacli/internal/orchestrator"
	"github.com/megacli/megacli/internal/permission"
	"github.com/megacli/megacli/internal/pubsub"
	"github.com/megacli/megacli/internal/session"
	"github.com/megacli/megacli/internal/shell"
	"github.com/megacli/megacli/internal/skills"
	"github.com/megacli/megacli/internal/ui/anim"
	"github.com/megacli/megacli/internal/ui/styles"
	"github.com/megacli/megacli/internal/update"
	"github.com/megacli/megacli/internal/version"
)

// UpdateAvailableMsg is sent when a new version is available.
type UpdateAvailableMsg struct {
	CurrentVersion string
	LatestVersion  string
	IsDevelopment  bool
}

// UpdateDownloadingMsg is sent when an update download has started.
type UpdateDownloadingMsg struct {
	CurrentVersion string
	LatestVersion  string
}

// UpdateProgressMsg is sent periodically during an update download to
// report progress.
type UpdateProgressMsg struct {
	Downloaded     int64
	Total          int64
	CurrentVersion string
	LatestVersion  string
}

// UpdateAppliedMsg is sent when an update has been applied and a restart
// is needed to use the new version.
type UpdateAppliedMsg struct {
	Version string
}

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service
	FileTracker filetracker.Service
	AskUser     askuser.Service

	AgentCoordinator agent.Coordinator

	// MegaCli extensions
	Orchestrator     *orchestrator.Orchestrator
	IPCManager       *ipc.Manager
	MegaToolRegistry *megatool.Registry

	LSPManager *lsp.Manager

	config *config.ConfigStore

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          *pubsub.Broker[tea.Msg]
	tuiWG           *sync.WaitGroup

	// global context and cleanup functions
	globalCtx          context.Context
	cleanupFuncs       []func(context.Context) error
	agentNotifications *pubsub.Broker[notify.Notification]
}

// New initializes a new application instance.
func New(ctx context.Context, conn *sql.DB, store *config.ConfigStore) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	files := history.NewService(q, conn)
	cfg := store.Config()
	skipPermissionsRequests := store.Overrides().SkipPermissionRequests
	var allowedTools []string
	if cfg.Permissions != nil && cfg.Permissions.AllowedTools != nil {
		allowedTools = cfg.Permissions.AllowedTools
	}

	// Initialize the orchestrator with an event callback that publishes
	// agent status changes to the TUI event broker.
	eventsBroker := pubsub.NewBroker[tea.Msg]()
	orch := orchestrator.New(func(evt orchestrator.AgentEvent) {
		eventsBroker.Publish(pubsub.UpdatedEvent, AgentStatusMsg(evt))
	})

	// Initialize the MegaTool registry with a display handler that
	// publishes display events to the TUI.
	megaReg := megatool.NewRegistry(func(evt megatool.DisplayEvent) {
		eventsBroker.Publish(pubsub.UpdatedEvent, ToolDisplayMsg(evt))
	})
	megaReg.Register(megatool.NewShowFileTool(store.WorkingDir()))

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(store.WorkingDir(), skipPermissionsRequests, allowedTools),
		FileTracker: filetracker.NewService(q),
		AskUser:     askuser.NewService(),
		LSPManager:  lsp.NewManager(store),

		Orchestrator:     orch,
		MegaToolRegistry: megaReg,

		globalCtx: ctx,

		config: store,

		events:             eventsBroker,
		serviceEventsWG:    &sync.WaitGroup{},
		tuiWG:              &sync.WaitGroup{},
		agentNotifications: pubsub.NewBroker[notify.Notification](),
	}

	app.setupEvents()

	// Check for updates in the background.
	go app.checkForUpdates(ctx)

	go mcp.Initialize(ctx, app.Permissions, store)

	// cleanup database upon app shutdown
	app.cleanupFuncs = append(
		app.cleanupFuncs,
		func(context.Context) error { return conn.Close() },
		func(ctx context.Context) error { return mcp.Close(ctx) },
	)

	// TODO: remove the concept of agent config, most likely.
	if !cfg.IsConfigured() {
		slog.Warn("No agent configuration found")
		return app, nil
	}
	if err := app.InitCoderAgent(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize coder agent: %w", err)
	}

	// Register the coder agent with the orchestrator.
	orch.RegisterAgent(&orchestrator.ManagedAgent{
		Name:        config.AgentCoder,
		Role:        orchestrator.RolePrimary,
		Coordinator: app.AgentCoordinator,
	})

	// Start IPC server for cross-process communication.
	app.IPCManager = ipc.NewManager(orch)
	agentNames := []string{config.AgentCoder}
	instanceName := ""
	if cfg.Options != nil && cfg.Options.Name != "" {
		instanceName = cfg.Options.Name
	}
	if err := app.IPCManager.Start(instanceName, store.WorkingDir(), agentNames); err != nil {
		slog.Warn("Failed to start IPC server", "error", err)
	} else {
		app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
			app.IPCManager.Stop()
			return nil
		})
	}

	// Inject MegaTool wrappers + delegate tools into the coder agent's tool chain.
	extraTools := megaReg.AsAgentTools()
	extraTools = append(extraTools, orchestrator.NewDelegateTool(orch, config.AgentCoder))
	if app.IPCManager != nil {
		extraTools = append(extraTools, ipc.NewRemoteDelegateTool(app.IPCManager, config.AgentCoder))
	}
	app.AgentCoordinator.SetExtraTools(extraTools)

	// Set up callback for LSP state updates.
	app.LSPManager.SetCallback(func(name string, client *lsp.Client) {
		if client == nil {
			updateLSPState(name, lsp.StateUnstarted, nil, nil, 0)
			return
		}
		client.SetDiagnosticsCallback(updateLSPDiagnostics)
		updateLSPState(name, client.GetServerState(), nil, client, 0)
	})
	go app.LSPManager.TrackConfigured()

	return app, nil
}

// Config returns the pure-data configuration.
func (app *App) Config() *config.Config {
	return app.config.Config()
}

// Store returns the config store.
func (app *App) Store() *config.ConfigStore {
	return app.config
}

// Events returns a per-caller subscription channel for application events.
// Each caller receives its own channel; all callers receive every event.
func (app *App) Events(ctx context.Context) <-chan pubsub.Event[tea.Msg] {
	return app.events.Subscribe(ctx)
}

// SendEvent publishes a message to all event subscribers.
func (app *App) SendEvent(msg tea.Msg) {
	app.events.Publish(pubsub.UpdatedEvent, msg)
}

// AgentNotifications returns the broker for agent notification events.
func (app *App) AgentNotifications() *pubsub.Broker[notify.Notification] {
	return app.agentNotifications
}

// resolveSession resolves which session to use for a non-interactive run
// If continueSessionID is set, it looks up that session by ID
// If useLast is set, it returns the most recently updated top-level session
// Otherwise, it creates a new session
func (app *App) resolveSession(ctx context.Context, continueSessionID string, useLast bool) (session.Session, error) {
	switch {
	case continueSessionID != "":
		if app.Sessions.IsAgentToolSession(continueSessionID) {
			return session.Session{}, fmt.Errorf("cannot continue an agent tool session: %s", continueSessionID)
		}
		sess, err := app.Sessions.Get(ctx, continueSessionID)
		if err != nil {
			return session.Session{}, fmt.Errorf("session not found: %s", continueSessionID)
		}
		if sess.ParentSessionID != "" {
			return session.Session{}, fmt.Errorf("cannot continue a child session: %s", continueSessionID)
		}
		return sess, nil

	case useLast:
		sess, err := app.Sessions.GetLast(ctx)
		if err != nil {
			return session.Session{}, fmt.Errorf("no sessions found to continue")
		}
		return sess, nil

	default:
		return app.Sessions.Create(ctx, agent.DefaultSessionName)
	}
}

// RunNonInteractive runs the application in non-interactive mode with the
// given prompt, printing to stdout.
func (app *App) RunNonInteractive(ctx context.Context, output io.Writer, prompt, largeModel, smallModel string, hideSpinner bool, continueSessionID string, useLast bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if largeModel != "" || smallModel != "" {
		if err := app.overrideModelsForNonInteractive(ctx, largeModel, smallModel); err != nil {
			return fmt.Errorf("failed to override models: %w", err)
		}
	}

	var (
		spinner   *format.Spinner
		stdoutTTY bool
		stderrTTY bool
		stdinTTY  bool
		progress  bool
	)

	if f, ok := output.(*os.File); ok {
		stdoutTTY = term.IsTerminal(f.Fd())
	}
	stderrTTY = term.IsTerminal(os.Stderr.Fd())
	stdinTTY = term.IsTerminal(os.Stdin.Fd())
	progress = app.config.Config().Options.Progress == nil || *app.config.Config().Options.Progress

	if !hideSpinner && stderrTTY {
		t := styles.ThemeForProvider(app.config.Config().Models[config.SelectedModelTypeLarge].Provider)

		// Detect background color to set the appropriate color for the
		// spinner's 'Generating...' text. Without this, that text would be
		// unreadable in light terminals.
		hasDarkBG := true
		if f, ok := output.(*os.File); ok && stdinTTY && stdoutTTY {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, f)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(charmtone.Pepper, t.WorkingLabelColor)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			LabelColor:  defaultFG,
			GradColorA:  t.WorkingGradFromColor,
			GradColorB:  t.WorkingGradToColor,
			CycleColors: true,
		})
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !hideSpinner && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}

	// Wait for MCP initialization to complete before reading MCP tools.
	if err := mcp.WaitForInit(ctx); err != nil {
		return fmt.Errorf("failed to wait for MCP initialization: %w", err)
	}

	// force update of agent models before running so mcp tools are loaded
	app.AgentCoordinator.UpdateModels(ctx)

	defer stopSpinner()

	sess, err := app.resolveSession(ctx, continueSessionID, useLast)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}

	if continueSessionID != "" || useLast {
		slog.Info("Continuing session for non-interactive run", "session_id", sess.ID)
	} else {
		slog.Info("Created session for non-interactive run", "session_id", sess.ID)
	}

	// Automatically approve all permission requests for this non-interactive
	// session.
	app.Permissions.AutoApproveSession(sess.ID)

	if saved := strings.TrimSpace(sess.ActiveAgent); saved != "" {
		if _, err := app.AgentCoordinator.SwitchAgent(ctx, saved); err != nil {
			slog.Warn("Could not restore agent from session, using default",
				"saved_agent", saved, "error", err)
		}
	}

	type response struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan response, 1)

	go func(ctx context.Context, sessionID, prompt string) {
		result, err := app.AgentCoordinator.Run(ctx, sess.ID, prompt)
		if err != nil {
			done <- response{
				err: fmt.Errorf("failed to start agent processing stream: %w", err),
			}
			return
		}
		done <- response{
			result: result,
		}
	}(ctx, sess.ID, prompt)

	messageEvents := app.Messages.Subscribe(ctx)
	messageReadBytes := make(map[string]int)
	var printed bool

	defer func() {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}

		// Always print a newline at the end. If output is a TTY this will
		// prevent the prompt from overwriting the last line of output.
		_, _ = fmt.Fprintln(output)
	}()

	for {
		if progress && stderrTTY {
			// HACK: Reinitialize the terminal progress bar on every iteration
			// so it doesn't get hidden by the terminal due to inactivity.
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case result := <-done:
			stopSpinner()
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, agent.ErrRequestCancelled) {
					slog.Debug("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.err)
			}
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				// Trim leading whitespace. Sometimes the LLM includes leading
				// formatting and intentation, which we don't want here.
				if readBytes == 0 {
					part = strings.TrimLeft(part, " \t")
				}
				// Ignore initial whitespace-only messages.
				if printed || strings.TrimSpace(part) != "" {
					printed = true
					fmt.Fprint(output, part)
				}
				messageReadBytes[msg.ID] = len(content)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

func (app *App) UpdateAgentModel(ctx context.Context) error {
	if app.AgentCoordinator == nil {
		return fmt.Errorf("agent configuration is missing")
	}
	return app.AgentCoordinator.UpdateModels(ctx)
}

// overrideModelsForNonInteractive parses the model strings and temporarily
// overrides the model configurations, then rebuilds the agent.
// Format: "model-name" (searches all providers) or "provider/model-name".
// Model matching is case-insensitive.
// If largeModel is provided but smallModel is not, the small model defaults to
// the provider's default small model.
func (app *App) overrideModelsForNonInteractive(ctx context.Context, largeModel, smallModel string) error {
	providers := app.config.Config().Providers.Copy()

	largeMatches, smallMatches, err := findModels(providers, largeModel, smallModel)
	if err != nil {
		return err
	}

	var largeProviderID string

	// Override large model.
	if largeModel != "" {
		found, err := validateMatches(largeMatches, largeModel, "large")
		if err != nil {
			return err
		}
		largeProviderID = found.provider
		slog.Info("Overriding large model for non-interactive run", "provider", found.provider, "model", found.modelID)
		app.config.Config().Models[config.SelectedModelTypeLarge] = config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}
	}

	// Override small model.
	switch {
	case smallModel != "":
		found, err := validateMatches(smallMatches, smallModel, "small")
		if err != nil {
			return err
		}
		slog.Info("Overriding small model for non-interactive run", "provider", found.provider, "model", found.modelID)
		app.config.Config().Models[config.SelectedModelTypeSmall] = config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}

	case largeModel != "":
		// No small model specified, but large model was - use provider's default.
		smallCfg := app.GetDefaultSmallModel(largeProviderID)
		app.config.Config().Models[config.SelectedModelTypeSmall] = smallCfg
	}

	return app.AgentCoordinator.UpdateModels(ctx)
}

// SetModelByString resolves a model string (e.g. "anthropic/claude-sonnet-4-5"
// or "gpt-4o") and sets it as the active large model. The agent is rebuilt
// to use the new model immediately.
func (app *App) SetModelByString(ctx context.Context, modelStr string) error {
	providers := app.config.Config().Providers.Copy()

	matches, _, err := findModels(providers, modelStr, "")
	if err != nil {
		return err
	}
	found, err := validateMatches(matches, modelStr, "command")
	if err != nil {
		return err
	}

	app.config.Config().Models[config.SelectedModelTypeLarge] = config.SelectedModel{
		Provider: found.provider,
		Model:    found.modelID,
	}

	smallCfg := app.GetDefaultSmallModel(found.provider)
	app.config.Config().Models[config.SelectedModelTypeSmall] = smallCfg

	return app.AgentCoordinator.UpdateModels(ctx)
}

// GetDefaultSmallModel returns the default small model for the given
// provider. Falls back to the large model if no default is found.
func (app *App) GetDefaultSmallModel(providerID string) config.SelectedModel {
	cfg := app.config.Config()
	largeModelCfg := cfg.Models[config.SelectedModelTypeLarge]

	// Find the provider in the known providers list to get its default small model.
	knownProviders, _ := config.Providers(cfg)
	var knownProvider *catwalk.Provider
	for _, p := range knownProviders {
		if string(p.ID) == providerID {
			knownProvider = &p
			break
		}
	}

	// For unknown/local providers, use the large model as small.
	if knownProvider == nil {
		slog.Warn("Using large model as small model for unknown provider", "provider", providerID, "model", largeModelCfg.Model)
		return largeModelCfg
	}

	defaultSmallModelID := knownProvider.DefaultSmallModelID
	model := cfg.GetModel(providerID, defaultSmallModelID)
	if model == nil {
		slog.Warn("Default small model not found, using large model", "provider", providerID, "model", largeModelCfg.Model)
		return largeModelCfg
	}

	slog.Info("Using provider default small model", "provider", providerID, "model", defaultSmallModelID)
	return config.SelectedModel{
		Provider:  providerID,
		Model:     defaultSmallModelID,
		MaxTokens: model.DefaultMaxTokens,
	}
}

func (app *App) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	setupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "agent-notifications", app.agentNotifications.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "mcp", mcp.SubscribeEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "skills", skills.SubscribeEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "askuser", app.AskUser.Subscribe, app.events)
	cleanupFunc := func(context.Context) error {
		cancel()
		app.serviceEventsWG.Wait()
		app.events.Shutdown()
		return nil
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	broker *pubsub.Broker[tea.Msg],
) {
	wg.Go(func() {
		subCh := subscriber(ctx)
		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("Subscription channel closed", "name", name)
					return
				}
				broker.Publish(pubsub.UpdatedEvent, tea.Msg(event))
			case <-ctx.Done():
				slog.Debug("Subscription cancelled", "name", name)
				return
			}
		}
	})
}

func (app *App) InitCoderAgent(ctx context.Context) error {
	coderAgentCfg := app.config.Config().Agents[config.AgentCoder]
	if coderAgentCfg.ID == "" {
		return fmt.Errorf("coder agent configuration is missing")
	}
	var err error
	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
		app.Permissions,
		app.History,
		app.FileTracker,
		app.AskUser,
		app.LSPManager,
		app.agentNotifications,
	)
	if err != nil {
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}
	return nil
}

// Subscribe sends events to the TUI as tea.Msgs.
func (app *App) Subscribe(program *tea.Program) {
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("TUI subscription panic: attempting graceful shutdown")
		program.Quit()
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
		slog.Debug("Cancelling TUI message handler")
		tuiCancel()
		app.tuiWG.Wait()
		return nil
	})
	defer app.tuiWG.Done()

	events := app.events.Subscribe(tuiCtx)
	msgs := unboundedEventRelay(tuiCtx, events)
	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("TUI message handler shutting down")
			return
		case msg, ok := <-msgs:
			if !ok {
				slog.Debug("TUI message channel closed")
				return
			}
			program.Send(msg)
		}
	}
}

// unboundedEventRelay starts a goroutine that relays messages from the
// pubsub subscriber channel to an output channel using an internal
// unbounded buffer. This prevents program.Send() backpressure from filling
// the subscriber channel, which would cause the broker to silently drop
// critical events (e.g. ask_user prompts).
func unboundedEventRelay(ctx context.Context, in <-chan pubsub.Event[tea.Msg]) <-chan tea.Msg {
	out := make(chan tea.Msg)
	go func() {
		defer close(out)
		var queue []tea.Msg
		for {
			if len(queue) == 0 {
				select {
				case <-ctx.Done():
					return
				case ev, ok := <-in:
					if !ok {
						return
					}
					queue = append(queue, ev.Payload)
				}
			}
			select {
			case <-ctx.Done():
				return
			case out <- queue[0]:
				queue[0] = nil
				queue = queue[1:]
				if len(queue) == 0 {
					queue = nil
				}
			case ev, ok := <-in:
				if !ok {
					for _, msg := range queue {
						select {
						case out <- msg:
						case <-ctx.Done():
							return
						}
					}
					return
				}
				queue = append(queue, ev.Payload)
			}
		}
	}()
	return out
}

// Shutdown performs a graceful shutdown of the application.
func (app *App) Shutdown() {
	start := time.Now()
	defer func() { slog.Debug("Shutdown took " + time.Since(start).String()) }()

	// Cancel all orchestrated agents first.
	if app.Orchestrator != nil {
		app.Orchestrator.Cancel()
	}

	// Then cancel the legacy coordinator.
	if app.AgentCoordinator != nil {
		app.AgentCoordinator.CancelAll()
	}

	// Now run remaining cleanup tasks in parallel.
	var wg sync.WaitGroup

	// Shared shutdown context for all timeout-bounded cleanup.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send exit event
	wg.Go(func() {
		event.AppExited()
	})

	// Kill all background shells.
	wg.Go(func() {
		shell.GetBackgroundShellManager().KillAll(shutdownCtx)
	})

	// Shutdown all LSP clients.
	wg.Go(func() {
		app.LSPManager.KillAll(shutdownCtx)
	})

	// Call all cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			wg.Go(func() {
				if err := cleanup(shutdownCtx); err != nil {
					slog.Error("Failed to cleanup app properly on shutdown", "error", err)
				}
			})
		}
	}
	wg.Wait()
}

// checkForUpdates checks for available updates and automatically applies them.
func (app *App) checkForUpdates(ctx context.Context) {
	if !version.IsRelease() {
		slog.Info("Skipping update check for local build", "version", version.Version)
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info, err := update.Check(checkCtx, version.Version)
	if err != nil || !info.Available() {
		return
	}

	// Don't auto-update dev builds — the user built this locally.
	if info.IsDevelopment() {
		slog.Info("Skipping auto-update for development build",
			"current", info.Current,
			"latest", info.Latest,
		)
		return
	}

	// Try auto-update
	slog.Info("Update available, applying auto-update",
		"current", info.Current,
		"latest", info.Latest,
	)

	app.events.Publish(pubsub.UpdatedEvent, UpdateDownloadingMsg{
		CurrentVersion: info.Current,
		LatestVersion:  info.Latest,
	})

	updCtx, updCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer updCancel()

	newVersion, err := update.ApplyWithProgress(updCtx, info.Latest, func(downloaded, total int64) {
		app.events.Publish(pubsub.UpdatedEvent, UpdateProgressMsg{
			Downloaded:     downloaded,
			Total:          total,
			CurrentVersion: info.Current,
			LatestVersion:  info.Latest,
		})
	})
	if err != nil {
		slog.Warn("Auto-update failed, notifying user", "error", err)
		app.events.Publish(pubsub.UpdatedEvent, UpdateAvailableMsg{
			CurrentVersion: info.Current,
			LatestVersion:  info.Latest,
			IsDevelopment:  info.IsDevelopment(),
		})
		return
	}

	slog.Info("Updated to new version, restart to apply", "version", newVersion)
	app.events.Publish(pubsub.UpdatedEvent, UpdateAppliedMsg{
		Version: newVersion,
	})
}

// AgentStatusMsg wraps an orchestrator.AgentEvent as a tea.Msg for the TUI.
type AgentStatusMsg orchestrator.AgentEvent

// ToolDisplayMsg wraps a megatool.DisplayEvent as a tea.Msg for the TUI.
type ToolDisplayMsg megatool.DisplayEvent
