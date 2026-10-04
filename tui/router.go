package tui

import (
	"context"
	"sort"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/tui/screens"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// router.go is the single source of truth for navigation, help, and the command
// palette (design §2). No screen is reachable unless it is registered here, and
// no nav item exists unless it maps to a real service seam. The registry drives
// SideNav, the HelpOverlay, direct key jumps, and the CommandPalette.

// ScreenID identifies a screen in the registry.
type ScreenID string

// The 15 registered screens (design §2). Wallet and Entity are split; Timeline
// and Help are contextual (Nav=false).
const (
	ScreenHome        ScreenID = "home"
	ScreenSearch      ScreenID = "search"
	ScreenWallet      ScreenID = "wallet"
	ScreenTransaction ScreenID = "transaction"
	ScreenEntity      ScreenID = "entity"
	ScreenGraph       ScreenID = "graph"
	ScreenNetwork     ScreenID = "network"
	ScreenGeoMap      ScreenID = "geomap"
	ScreenDetection   ScreenID = "detection"
	ScreenAlerts      ScreenID = "alerts"
	ScreenMonitoring  ScreenID = "monitoring"
	ScreenData        ScreenID = "data"
	ScreenReports     ScreenID = "reports"
	ScreenSettings    ScreenID = "settings"
	ScreenHelp        ScreenID = "help"
	ScreenTimeline    ScreenID = "timeline"
	ScreenBlock       ScreenID = "block"
	ScreenExtensions  ScreenID = "extensions"
)

// RenderCap is a bitfield of the component capabilities a screen mounts
// (design §2). It documents, per screen, which visualizers the shell wires up.
type RenderCap uint8

const (
	CapTable RenderCap = 1 << iota
	CapGraph
	CapMap
	CapTimeline
	CapStream
	CapDetail
	CapForm
)

// Has reports whether the capability set includes cap.
func (c RenderCap) Has(cap RenderCap) bool { return c&cap != 0 }

// AppCtx is handed to each screen factory. It carries exactly what a screen
// needs to render: the shared app seam (for services via app/services.go), the
// resolved theme styles, the current layout breakpoint, the cancellable program
// context, and the globally selected subject. Screens obtain analysis services
// through App — never from Engine.* (nil) and never by running SQL/analysis
// themselves (AGENTS §12).
type AppCtx struct {
	App        *app.App
	Styles     theme.Styles
	Breakpoint Breakpoint
	// Program is the program context (tea.WithContext). Screens derive child
	// contexts for cancellable service commands. May be nil in tests.
	Program context.Context
	// Subject is the globally selected investigation subject at build time.
	Subject Subject
	// Summarizer is the OPTIONAL local-LLM narrator for the :explain pane
	// (design §E). It is transport-free from tui's perspective: tui imports only
	// package llm (the interface), never llm/ollama. A nil value is treated as
	// llm.NewDeterministic() downstream, so the pane always works. The concrete
	// (possibly network-capable) implementation is injected at the cli/commands
	// composition root via NewRoot's WithSummarizer option.
	Summarizer llm.Summarizer
	// Acquire is the OPTIONAL network-acquisition seam (app.AcquireService),
	// injected at the cli/commands composition root via WithAcquireService. It
	// is the only network-touching dependency a screen may hold, and only via
	// the interface so no provider/net import enters tui's closure. Nil = the
	// TUI is offline-only and must not offer network acquisition.
	Acquire app.AcquireService
}

// Screen is a mounted screen. Each builds from components and holds only
// ephemeral presentation state (design §2, §4). View receives the exact Frame
// the shell allocated and must not overflow it.
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View(Frame) string
	ShortHelp() []key.Binding
}

// ScreenDef is a registry entry. The factory (New) is the only way to construct
// the screen, which guarantees the registry is the single reachability source.
type ScreenDef struct {
	ID    ScreenID
	Title string
	Key   string
	Caps  RenderCap
	Help  string
	Nav   bool
	New   func(*AppCtx) Screen
}

// registry is the ordered, canonical screen table (design §2). Order is the
// SideNav display order for Nav=true entries.
var registry = []ScreenDef{
	{ID: ScreenHome, Title: "Dashboard", Key: "1", Caps: CapTable | CapDetail, Nav: true,
		Help: "Case overview, counts, network/model state", New: realFactory(ScreenHome)},
	{ID: ScreenSearch, Title: "Analysis", Key: "2", Caps: CapTable, Nav: true,
		Help: "Enter a wallet/tx/IP to run the full investigation pipeline", New: realFactory(ScreenSearch)},
	{ID: ScreenWallet, Title: "Wallet / Entity", Key: "3", Caps: CapDetail | CapTable | CapGraph, Nav: true,
		Help: "Analyze a wallet: risk, signals, related, subgraph", New: realFactory(ScreenWallet)},
	{ID: ScreenTransaction, Title: "Transaction", Key: "4", Caps: CapDetail | CapTable, Nav: true,
		Help: "Inspect a transaction and its edges", New: realFactory(ScreenTransaction)},
	{ID: ScreenEntity, Title: "Entity Cluster", Key: "5", Caps: CapDetail | CapGraph, Nav: true,
		Help: "Clustered addresses for the subject", New: realFactory(ScreenEntity)},
	{ID: ScreenGraph, Title: "Graph", Key: "6", Caps: CapGraph, Nav: true,
		Help: "Explore the bounded transaction graph", New: realFactory(ScreenGraph)},
	{ID: ScreenNetwork, Title: "Network / IP", Key: "7", Caps: CapTable | CapDetail, Nav: true,
		Help: "Network observations by IP / ASN", New: realFactory(ScreenNetwork)},
	{ID: ScreenGeoMap, Title: "Geo Map", Key: "8", Caps: CapMap, Nav: true,
		Help: "Country-binned observation density (metadata)", New: realFactory(ScreenGeoMap)},
	{ID: ScreenDetection, Title: "Detection", Key: "9", Caps: CapTable | CapDetail, Nav: true,
		Help: "Pattern + anomaly/flow model outputs", New: realFactory(ScreenDetection)},
	{ID: ScreenAlerts, Title: "Alerts", Key: "a", Caps: CapTable | CapDetail, Nav: true,
		Help: "Case alerts and monitor alerts", New: realFactory(ScreenAlerts)},
	{ID: ScreenMonitoring, Title: "Live Monitoring", Key: "o", Caps: CapStream | CapTable, Nav: true,
		Help: "Live/historical monitor sessions", New: realFactory(ScreenMonitoring)},
	{ID: ScreenData, Title: "Data / Ingestion", Key: "d", Caps: CapTable | CapDetail, Nav: true,
		Help: "Datasets, checkpoints, graph build", New: realFactory(ScreenData)},
	{ID: ScreenReports, Title: "Reports", Key: "r", Caps: CapTable | CapDetail, Nav: true,
		Help: "Build, preview, export, verify reports", New: realFactory(ScreenReports)},
	{ID: ScreenExtensions, Title: "Extensions", Key: "x", Caps: CapTable | CapDetail, Nav: true,
		Help: "Ministry knowledge bases, model packs, demo examples (offline)", New: realFactory(ScreenExtensions)},
	{ID: ScreenSettings, Title: "Settings", Key: "s", Caps: CapForm | CapDetail, Nav: true,
		Help: "Config, cases, geoip DB status (read-mostly)", New: realFactory(ScreenSettings)},
	// Contextual screens (Nav=false): reachable by direct key / navigation only.
	{ID: ScreenHelp, Title: "Help", Key: "?", Caps: CapDetail, Nav: false,
		Help: "Keyboard reference", New: realFactory(ScreenHelp)},
	{ID: ScreenTimeline, Title: "Timeline", Key: "t", Caps: CapTimeline, Nav: false,
		Help: "Chronological events for the subject", New: realFactory(ScreenTimeline)},
	{ID: ScreenBlock, Title: "Block", Key: "B", Caps: CapDetail | CapTable, Nav: false,
		Help: "Block header + its transactions (offline after acquire)", New: realFactory(ScreenBlock)},
}

// realFactory returns a registry factory that builds the real screen for an id
// and wraps it in the screens->tui adapter. It replaces stubFactory from
// FEAT-002 WITHOUT changing any registry wiring (ids/keys/caps/help/Nav): only
// ScreenDef.New now points here (FEAT-002 contract). A screen that is somehow
// unmapped falls back to the stub so the registry can never produce a nil
// Screen (tests assert Build never returns nil for a registered id).
func realFactory(id ScreenID) func(*AppCtx) Screen {
	return func(ctx *AppCtx) Screen {
		sc := toScreenCtx(ctx)
		if m := buildScreenModel(id, sc); m != nil {
			return adapt(m)
		}
		return stubFactory(id)(ctx)
	}
}

// buildScreenModel constructs the concrete screens.Model for an id. This is the
// single dispatch table from ScreenID to its implementation; keeping it here
// (not in the registry literal) lets the registry stay a pure data table.
func buildScreenModel(id ScreenID, sc *screens.ScreenCtx) screens.Model {
	switch id {
	case ScreenHome:
		return screens.NewHome(sc)
	case ScreenSearch:
		return screens.NewSearch(sc)
	case ScreenWallet:
		return screens.NewWallet(sc)
	case ScreenTransaction:
		return screens.NewTransaction(sc)
	case ScreenEntity:
		return screens.NewEntity(sc)
	case ScreenGraph:
		return screens.NewGraph(sc)
	case ScreenNetwork:
		return screens.NewNetwork(sc)
	case ScreenGeoMap:
		return screens.NewGeoMap(sc)
	case ScreenDetection:
		return screens.NewDetection(sc)
	case ScreenAlerts:
		return screens.NewAlerts(sc)
	case ScreenMonitoring:
		return screens.NewMonitoring(sc)
	case ScreenData:
		return screens.NewData(sc)
	case ScreenReports:
		return screens.NewReports(sc)
	case ScreenSettings:
		return screens.NewSettings(sc)
	case ScreenHelp:
		return screens.NewHelp(sc)
	case ScreenTimeline:
		return screens.NewTimeline(sc)
	case ScreenBlock:
		return screens.NewBlock(sc)
	case ScreenExtensions:
		return screens.NewExtensions(sc)
	default:
		return nil
	}
}

// index maps ScreenID and nav key to the registry entry for O(1) lookup. Built
// once at init; panics on a duplicate ID or key so a registration mistake fails
// loudly at startup rather than silently shadowing a screen.
var (
	byID  = map[ScreenID]*ScreenDef{}
	byKey = map[string]*ScreenDef{}
)

func init() {
	for i := range registry {
		def := &registry[i]
		if _, dup := byID[def.ID]; dup {
			panic("tui: duplicate screen id " + string(def.ID))
		}
		byID[def.ID] = def
		if def.Key != "" {
			if _, dup := byKey[def.Key]; dup {
				panic("tui: duplicate nav key " + def.Key + " for " + string(def.ID))
			}
			byKey[def.Key] = def
		}
	}
}

// Lookup returns the registry entry for an id, or nil if unregistered.
func Lookup(id ScreenID) *ScreenDef { return byID[id] }

// LookupKey returns the registry entry bound to a single-key nav shortcut.
func LookupKey(k string) *ScreenDef { return byKey[k] }

// Screens returns a copy of the full registry in display order.
func Screens() []ScreenDef {
	out := make([]ScreenDef, len(registry))
	copy(out, registry)
	return out
}

// NavScreens returns the registry entries that appear in the SideNav
// (Nav=true), in display order.
func NavScreens() []ScreenDef {
	var out []ScreenDef
	for _, d := range registry {
		if d.Nav {
			out = append(out, d)
		}
	}
	return out
}

// NavJumpKeys returns every registered single-key nav shortcut, sorted, so the
// global Jump binding and help text stay in sync with the registry.
func NavJumpKeys() []string {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Build constructs a screen from its registry factory, or nil if unregistered.
func Build(id ScreenID, ctx *AppCtx) Screen {
	def := byID[id]
	if def == nil || def.New == nil {
		return nil
	}
	return def.New(ctx)
}
