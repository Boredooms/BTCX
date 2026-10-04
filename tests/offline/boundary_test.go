// Package offline contains architectural guard tests that enforce the BCTX
// offline boundary: only the acquisition layer may depend on the network.
//
// Three complementary checks run here:
//
//   - TestAnalysisPackagesHaveNoTransitiveNetworkDeps uses the Go toolchain's
//     own package graph (`go list -deps -json`) to assert that no analysis
//     package has ANY transitive dependency on a network-TRANSPORT package or
//     on an acquisition/provider adapter. This is the strong, authoritative
//     check: it sees through indirection the source grep cannot.
//   - TestAnalysisPackagesHaveNoDirectNetworkImports is the retained grep
//     backstop: it walks the analysis source and fails on a direct transport
//     import OR a net-dial call pattern. It runs even when `go` is absent, so a
//     new socket added to analysis code is still caught offline.
//   - TestNetworkStateOpensNoHTTP asserts the one deliberately-excluded
//     transitive node (network/state) itself pulls in no HTTP/WebSocket/TLS
//     transport, keeping the exclusion honest.
//
// SCOPING OF THE TRANSITIVE FORBIDDEN SET (deliberate, not a loosening)
// --------------------------------------------------------------------
// The transitive deps check forbids true network-FETCH/TRANSPORT surfaces that
// analysis code could use to pull data over the wire (net/http, crypto/tls,
// golang.org/x/net transport, and the BCTX acquisition/provider packages).
// Three transitive edges are DELIBERATELY EXCLUDED from the transitive set
// because they are provably benign; each is kept honest by a separate, narrower
// assertion so nothing is loosened by accident:
//
//  1. github.com/bctx/bctx/network/state — reachable from every analysis
//     package only as a TYPE edge: sdk -> sdk/engine.go holds a state.Prober
//     field and NetworkStatus() returns state.Status. Analysis code uses only
//     sdk.Repository + the persistence Row types; it never constructs the
//     Engine nor calls the Prober. Physically breaking this edge would require
//     refactoring package sdk (move Engine/prober out, or the Repository
//     interface) — fenced off by project constraints and of zero runtime
//     benefit. Honesty is preserved by TestNetworkStateOpensNoHTTP (network/
//     state opens no HTTP/WS/TLS) AND by the direct-import grep (no analysis
//     package may DIRECTLY import network/state or call net.Dial*).
//
//  2. net/url — rides in via the stdlib html/template (URL-attribute
//     sanitization, in-memory, opens no socket) used by the self-contained HTML
//     renderer, and via the stdlib net resolver pulled in by network/state.
//     Both are offline-safe stdlib. net/url is therefore not in the transitive
//     forbidden set, but a DIRECT import of net/url in analysis source is still
//     rejected by the grep backstop.
//
//  3. github.com/bctx/bctx/monitoring -> .../acquisition — the Phase-7 brief
//     allows this verbatim: "Monitoring ... may compose acquisition, but must
//     not import HTTP clients." So the composition edge is allowed, while
//     TestMonitoringAndReportingNoHTTPTransport asserts monitoring (and
//     reporting) pull in no net/http transport, and reporting imports no
//     acquisition at all.
//
// bare "net" is deliberately ALLOWED: blockchain/parser legitimately uses
// net.ParseIP, and the ingestion -> parser -> net edge is a real, offline-safe
// dependency (address/IP parsing, no sockets). The dial surface of net is
// caught by the call-pattern grep backstop instead.
package offline

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// modulePath is the Go module path of this repository.
const modulePath = "github.com/bctx/bctx"

// analysisDirs are packages (relative to the repo root) that MUST remain
// network-free. If any of them reaches net/http (or a raw net dialer), the
// offline guarantee is broken.
var analysisDirs = []string{
	"storage", "graph", "ml", "risk", "evidence", "investigation",
	"ingestion", "reporting", "pkg", "blockchain/normalizer",
	"blockchain/parser",
	// Phase 6: the monitoring orchestrator composes acquisition + persist +
	// analysis but must stay network-free itself; all network use lives in the
	// acquisition provider (blockchain/acquisition/explorer).
	"monitoring",
}

// forbiddenTransitivePaths are exact import paths that no analysis package may
// pull in transitively. These are the real network-TRANSPORT stdlib packages
// (fetch surfaces). net/url is intentionally absent (see the package comment:
// it rides in via offline-safe html/template + the net resolver).
var forbiddenTransitivePaths = []string{
	"net/http",
	"net/http/httptest",
	"crypto/tls",
}

// forbiddenTransitivePrefixes are import-path prefixes forbidden anywhere in an
// analysis package's transitive closure: the x/net transport tree and the BCTX
// network-allowed layer (acquisition + explorer/provider adapters). Note
// network/state is intentionally absent here (see the package comment: it is a
// benign type edge via sdk, kept honest by TestNetworkStateOpensNoHTTP).
//
// github.com/bctx/bctx/acquisition appears here, but the monitoring root is
// granted a narrow, documented exemption for it (see acquisitionComposers and
// the package comment): the acquisition package is the provider INTERFACE +
// scripted/offline provider and itself opens no HTTP/TLS socket (the real
// explorer HTTP client lives under blockchain/acquisition, which stays
// forbidden everywhere, monitoring included).
var forbiddenTransitivePrefixes = []string{
	"golang.org/x/net/",
	modulePath + "/acquisition",
	modulePath + "/blockchain/acquisition",
}

// acquisitionComposerRoots are the analysis roots the Phase-7 brief explicitly
// permits to COMPOSE the acquisition interface ("Monitoring ... may compose
// acquisition, but must not import HTTP clients"). For these roots the
// github.com/bctx/bctx/acquisition edge is allowed, but blockchain/acquisition
// (the HTTP explorer provider) and all HTTP/TLS transport remain forbidden —
// enforced by exemptEdge below and by TestMonitoringAndReportingNoHTTPTransport.
var acquisitionComposerRoots = []string{
	modulePath + "/monitoring",
}

// exemptEdge reports whether a flagged (analysisPkg -> dep) edge is one of the
// deliberately-allowed exemptions: a brief-sanctioned composer reaching the
// acquisition interface package (but never the HTTP explorer provider).
func exemptEdge(analysisPkg, dep string) bool {
	// Only the bare acquisition interface package (and its sub-packages) is
	// ever exempt; blockchain/acquisition is never exempt.
	if strings.HasPrefix(dep, modulePath+"/blockchain/acquisition") {
		return false
	}
	isAcquisition := dep == modulePath+"/acquisition" ||
		strings.HasPrefix(dep, modulePath+"/acquisition/")
	if !isAcquisition {
		return false
	}
	for _, root := range acquisitionComposerRoots {
		if analysisPkg == root || strings.HasPrefix(analysisPkg, root+"/") {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// tests/offline -> repo root is two levels up.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// forbiddenTransitive reports whether importPath is a forbidden transitive
// network/transport or acquisition package (exact match or forbidden prefix).
func forbiddenTransitive(importPath string) bool {
	for _, p := range forbiddenTransitivePaths {
		if importPath == p {
			return true
		}
	}
	for _, pre := range forbiddenTransitivePrefixes {
		if importPath == strings.TrimSuffix(pre, "/") || strings.HasPrefix(importPath, pre) {
			return true
		}
	}
	return false
}

// goListPackage is the subset of `go list -json` output we decode.
type goListPackage struct {
	ImportPath string   `json:"ImportPath"`
	Imports    []string `json:"Imports"`
	Deps       []string `json:"Deps"`
}

// runGoListDeps runs `go list -deps -json` over the given roots from the repo
// root and returns the decoded package objects. It skips the calling test when
// `go` is not on PATH (never a silent green).
func runGoListDeps(t *testing.T, roots []string) []goListPackage {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH; skipping deps-closure boundary check " +
			"(run with the Go toolchain available to enforce it)")
	}
	root := repoRoot(t)
	args := append([]string{"list", "-deps", "-json"}, roots...)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list -deps failed: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("go list -deps failed: %v", err)
	}
	var pkgs []goListPackage
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var pkg goListPackage
		if derr := dec.Decode(&pkg); derr != nil {
			t.Fatalf("decode go list json: %v", derr)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// TestAnalysisPackagesHaveNoTransitiveNetworkDeps is the authoritative offline
// guard. For each forbidden-root package it asks the Go toolchain for the FULL
// transitive dependency closure and asserts none of it is a network-transport
// or acquisition package. This sees through any level of indirection a source
// grep would miss. It skips (never silently passes) when `go` is absent.
func TestAnalysisPackagesHaveNoTransitiveNetworkDeps(t *testing.T) {
	roots := make([]string, 0, len(analysisDirs))
	for _, dir := range analysisDirs {
		roots = append(roots, modulePath+"/"+dir+"/...")
	}
	pkgs := runGoListDeps(t, roots)

	var offenders []string
	seenAnalysis := map[string]bool{}
	for _, pkg := range pkgs {
		// Only enforce the rule on packages that belong to an analysis root;
		// the forbidden packages must never be reachable FROM an analysis
		// package, which we assert via each analysis package's own Deps.
		if !isAnalysisPackage(pkg.ImportPath) {
			continue
		}
		seenAnalysis[pkg.ImportPath] = true
		for _, dep := range pkg.Deps {
			if forbiddenTransitive(dep) && !exemptEdge(pkg.ImportPath, dep) {
				offenders = append(offenders, pkg.ImportPath+" -> "+dep)
			}
		}
	}

	if len(seenAnalysis) == 0 {
		t.Fatal("no analysis packages were resolved by `go list`; the deps " +
			"boundary check did not actually run")
	}
	// reporting must be among the audited roots (it is a Phase-7 addition and a
	// primary subject of this guard).
	if !seenAnalysis[modulePath+"/reporting"] {
		t.Fatal("reporting package was not audited by the deps check; " +
			"resolved analysis packages did not include reporting")
	}

	if len(offenders) > 0 {
		t.Fatalf("forbidden transitive network/acquisition dependency found "+
			"in analysis packages (only acquisition may touch the network):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// isAnalysisPackage reports whether importPath is one of our analysis packages
// (lives under an analysisDirs root within this module).
func isAnalysisPackage(importPath string) bool {
	if !strings.HasPrefix(importPath, modulePath+"/") {
		return false
	}
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	for _, dir := range analysisDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// directForbidden are source substrings for forbidden transport imports. Each
// is matched as a quoted import path so a bare `net` import is never flagged.
// Unlike the transitive set this INCLUDES net/url and network/state: a DIRECT
// import of either in analysis source is a real red flag (the transitive
// exclusions above are only for indirect, provably-benign edges).
//
// The acquisition-INTERFACE import (github.com/bctx/bctx/acquisition) is listed
// here too, but it is exempted for the brief-sanctioned composer roots
// (monitoring) via directForbiddenException. blockchain/acquisition (the HTTP
// explorer provider) is NEVER exempted.
var directForbidden = []string{
	`"net/http"`,
	`"net/http/httptest"`,
	`"net/url"`,
	`"crypto/tls"`,
	`"golang.org/x/net/`,
	`"` + modulePath + `/blockchain/acquisition`,
	`"` + modulePath + `/acquisition`,
	`"` + modulePath + `/network/state"`,
}

// acquisitionInterfaceImport is the quoted import prefix of the acquisition
// INTERFACE package (not the HTTP explorer provider).
const acquisitionInterfaceImport = `"` + modulePath + `/acquisition`

// directForbiddenException reports whether a flagged direct import in a given
// source file is a sanctioned exemption. Only the acquisition-interface import
// inside a composer root (monitoring) is exempt; blockchain/acquisition and all
// transport imports are never exempt.
func directForbiddenException(path, forbidden string) bool {
	if forbidden != acquisitionInterfaceImport {
		return false
	}
	norm := filepath.ToSlash(path)
	for _, root := range acquisitionComposerRoots {
		rel := strings.TrimPrefix(root, modulePath+"/")
		if strings.Contains(norm, "/"+rel+"/") {
			return true
		}
	}
	return false
}

// dialCallPatterns are net-dial / listen call expressions that would open a
// socket. They are forbidden in analysis source even though the bare "net"
// import itself is allowed for address/IP parsing (net.ParseIP, net.ParseCIDR,
// net.IP). We match the call forms, not the parsing helpers.
var dialCallPatterns = []string{
	"net.Dial(",
	"net.DialTimeout(",
	"net.DialContext(",
	"net.Dialer{",
	"net.Listen(",
	"net.ListenPacket(",
	"net.ListenTCP(",
	"net.ListenUDP(",
}

// forEachAnalysisSource walks every non-test .go file under the analysis dirs
// and calls fn with its path and contents.
func forEachAnalysisSource(t *testing.T, fn func(path, src string)) {
	t.Helper()
	root := repoRoot(t)
	for _, dir := range analysisDirs {
		base := filepath.Join(root, filepath.FromSlash(dir))
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // dir may not exist; skip
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil // tests may use net for fixtures
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			fn(path, string(data))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
}

// TestAnalysisPackagesHaveNoDirectNetworkImports is the grep backstop. It walks
// the analysis source and fails on a direct transport import OR a net-dial call
// pattern. The bare "net" import and net.ParseIP/net.IP/net.ParseCIDR remain
// allowed. This runs even when the Go toolchain is unavailable, so a brand-new
// socket added to analysis code is caught regardless.
func TestAnalysisPackagesHaveNoDirectNetworkImports(t *testing.T) {
	forEachAnalysisSource(t, func(path, src string) {
		for _, f := range directForbidden {
			if strings.Contains(src, f) && !directForbiddenException(path, f) {
				t.Errorf("forbidden network import %s found in analysis file %s "+
					"(only the acquisition layer may use the network)", f, path)
			}
		}
		for _, call := range dialCallPatterns {
			if strings.Contains(src, call) {
				t.Errorf("forbidden net-dial call %q found in analysis file %s "+
					"(bare net is allowed only for ParseIP/ParseCIDR, never dialing)",
					call, path)
			}
		}
	})
}

// TestMonitoringAndReportingNoHTTPTransport is the direct-import assertion for
// the two packages whose transitive exclusions are the most sensitive:
//
//   - monitoring MAY compose acquisition (brief-allowed) but must not pull an
//     HTTP client — it must not transitively reach net/http / crypto/tls /
//     x/net transport.
//   - reporting must not import acquisition AT ALL, and must not reach
//     net/http transport.
func TestMonitoringAndReportingNoHTTPTransport(t *testing.T) {
	pkgs := runGoListDeps(t, []string{
		modulePath + "/monitoring/...",
		modulePath + "/reporting/...",
	})

	for _, pkg := range pkgs {
		isMon := pkg.ImportPath == modulePath+"/monitoring" ||
			strings.HasPrefix(pkg.ImportPath, modulePath+"/monitoring/")
		isRep := pkg.ImportPath == modulePath+"/reporting" ||
			strings.HasPrefix(pkg.ImportPath, modulePath+"/reporting/")
		if !isMon && !isRep {
			continue
		}
		for _, dep := range pkg.Deps {
			// Neither may reach an HTTP/TLS/x-net transport.
			if dep == "net/http" || dep == "net/http/httptest" ||
				dep == "crypto/tls" || strings.HasPrefix(dep, "golang.org/x/net/") {
				t.Errorf("%s must not reach network transport %q", pkg.ImportPath, dep)
			}
			// reporting must not reach acquisition at all.
			if isRep && (dep == modulePath+"/acquisition" ||
				strings.HasPrefix(dep, modulePath+"/acquisition/") ||
				strings.HasPrefix(dep, modulePath+"/blockchain/acquisition")) {
				t.Errorf("reporting package %s must not import acquisition (%q)",
					pkg.ImportPath, dep)
			}
		}
	}
}

// TestNetworkStateOpensNoHTTP keeps the one deliberately-excluded transitive
// node honest: network/state (reachable from analysis only as a type edge via
// sdk) must itself pull in no HTTP/WebSocket/TLS transport. Its only socket use
// is a bounded TCP dial for a connectivity probe (net.Dialer), which analysis
// code never invokes.
func TestNetworkStateOpensNoHTTP(t *testing.T) {
	pkgs := runGoListDeps(t, []string{modulePath + "/network/state"})
	for _, pkg := range pkgs {
		if pkg.ImportPath != modulePath+"/network/state" {
			continue
		}
		for _, dep := range pkg.Deps {
			if dep == "net/http" || dep == "net/http/httptest" ||
				dep == "crypto/tls" || strings.HasPrefix(dep, "golang.org/x/net/http") {
				t.Errorf("network/state must open no HTTP/TLS transport, found dep %q", dep)
			}
		}
		return
	}
	t.Fatal("network/state was not resolved by `go list`")
}

// tuiLeafOfflineRoots are the STRICTLY-OFFLINE tui leaf packages (design §15,
// §16): they read installed local assets (GeoIP .mmdb / world geometry) and
// must never reach acquisition, monitoring, HTTP/TLS transport, or a socket.
// Unlike the general tui/... roots (whose Monitoring/Data screens legitimately
// compose acquisition/monitoring), these two are held to the stricter rule.
var tuiLeafOfflineRoots = []string{
	modulePath + "/tui/geoip",
	modulePath + "/tui/mapdata",
}

// TestTUIPackagesHaveNoHTTPTransport is the dedicated offline-boundary check for
// the Phase 8 TUI packages (design §17.1, acceptance §19.2). It is modeled on
// TestMonitoringAndReportingNoHTTPTransport, NOT added to analysisDirs: the
// general tui/... roots are allowed to compose acquisition/monitoring (the
// Monitoring and Data screens require it) and those edges are already proven
// transport-free, so this check forbids ONLY the transport set for tui/...:
//
//	net/http, net/http/httptest, crypto/tls, golang.org/x/net/ (transport)
//
// For the two STRICTLY-OFFLINE leaf packages tui/geoip and tui/mapdata it
// ADDITIONALLY forbids reaching acquisition/monitoring and any bare net-dial
// call pattern — they must never reach the network layer or open a socket.
func TestTUIPackagesHaveNoHTTPTransport(t *testing.T) {
	pkgs := runGoListDeps(t, []string{modulePath + "/tui/..."})

	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if !strings.HasPrefix(pkg.ImportPath, modulePath+"/tui") {
			continue
		}
		seen[pkg.ImportPath] = true
		isLeaf := isTUILeafOffline(pkg.ImportPath)
		for _, dep := range pkg.Deps {
			// All tui packages: forbid HTTP/TLS/x-net transport.
			if dep == "net/http" || dep == "net/http/httptest" ||
				dep == "crypto/tls" || strings.HasPrefix(dep, "golang.org/x/net/") {
				t.Errorf("%s must not reach network transport %q", pkg.ImportPath, dep)
			}
			// Strictly-offline leaves: forbid acquisition + monitoring too.
			if isLeaf {
				if dep == modulePath+"/acquisition" ||
					strings.HasPrefix(dep, modulePath+"/acquisition/") ||
					strings.HasPrefix(dep, modulePath+"/blockchain/acquisition") ||
					dep == modulePath+"/monitoring" ||
					strings.HasPrefix(dep, modulePath+"/monitoring/") {
					t.Errorf("strictly-offline leaf %s must not reach %q", pkg.ImportPath, dep)
				}
			}
		}
	}

	// The two strictly-offline leaves must have been audited.
	for _, leaf := range tuiLeafOfflineRoots {
		if !seen[leaf] {
			t.Fatalf("offline-leaf package %s was not resolved by `go list`; "+
				"the tui boundary check did not actually run against it", leaf)
		}
	}

	// Source backstop: the strictly-offline leaves must open no socket and
	// import no net/* transport directly (runs even without the toolchain path
	// above). A bare "net" import is allowed only for IP value parsing inside
	// the MMDB reader, which lives in the dependency, not in our leaf source.
	assertTUILeavesNoDialSource(t)
}

func isTUILeafOffline(importPath string) bool {
	for _, leaf := range tuiLeafOfflineRoots {
		if importPath == leaf || strings.HasPrefix(importPath, leaf+"/") {
			return true
		}
	}
	return false
}

// assertTUILeavesNoDialSource greps the strictly-offline tui leaf source for any
// transport import or net-dial/listen call pattern.
func assertTUILeavesNoDialSource(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	leafTransport := []string{
		`"net/http"`, `"net/http/httptest"`, `"net/url"`, `"crypto/tls"`,
		`"golang.org/x/net/`,
		`"` + modulePath + `/acquisition`,
		`"` + modulePath + `/blockchain/acquisition`,
		`"` + modulePath + `/monitoring"`,
	}
	for _, leaf := range []string{"tui/geoip", "tui/mapdata"} {
		base := filepath.Join(root, filepath.FromSlash(leaf))
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			src := string(data)
			for _, f := range leafTransport {
				if strings.Contains(src, f) {
					t.Errorf("strictly-offline leaf file %s must not import %s", path, f)
				}
			}
			for _, call := range dialCallPatterns {
				if strings.Contains(src, call) {
					t.Errorf("strictly-offline leaf file %s must not dial (%q)", path, call)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
}

// llmOllamaPath is the import path of the ONLY new network-capable package
// (design §E). It is reachable ONLY from the real top-level command roots and
// must never enter the closure of any tui/..., reporting/..., or analysisDirs
// package.
const llmOllamaPath = modulePath + "/llm/ollama"

// confinedRoots are the roots over which llm/ollama (and net/http) must NEVER
// appear transitively: all tui packages, reporting, and every analysis root.
func confinedRoots() []string {
	roots := []string{modulePath + "/tui/...", modulePath + "/reporting/..."}
	for _, dir := range analysisDirs {
		roots = append(roots, modulePath+"/"+dir+"/...")
	}
	return roots
}

// TestLLMTransportConfinedToLLMOllama asserts the transport is fully confined:
// no tui/... (incl. tui/screens), reporting/..., or analysisDirs package has
// github.com/bctx/bctx/llm/ollama — nor net/http — in its transitive closure
// (design §E.6). This is the authoritative deps-closure guard for the new
// package; the direct-import grep below backs it up without the toolchain.
func TestLLMTransportConfinedToLLMOllama(t *testing.T) {
	pkgs := runGoListDeps(t, confinedRoots())

	inScope := func(p string) bool {
		if strings.HasPrefix(p, modulePath+"/tui") ||
			p == modulePath+"/reporting" || strings.HasPrefix(p, modulePath+"/reporting/") {
			return true
		}
		return isAnalysisPackage(p)
	}

	var offenders []string
	seen := 0
	for _, pkg := range pkgs {
		if !inScope(pkg.ImportPath) {
			continue
		}
		seen++
		for _, dep := range pkg.Deps {
			if dep == llmOllamaPath || dep == "net/http" || dep == "net/http/httptest" {
				offenders = append(offenders, pkg.ImportPath+" -> "+dep)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no confined packages were resolved by `go list`; the llm/ollama " +
			"confinement check did not actually run")
	}
	if len(offenders) > 0 {
		t.Fatalf("llm/ollama (or net/http) leaked into a confined package "+
			"(only cli/commands may import the transport):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestLLMOllamaIsOnlyNewTransport asserts package llm (the interface +
// deterministic summarizer) itself carries NO net/http in its closure, so the
// tui / tui/screens -> llm import stays transport-free (design §E.3/§E.6).
func TestLLMOllamaIsOnlyNewTransport(t *testing.T) {
	pkgs := runGoListDeps(t, []string{modulePath + "/llm"})
	for _, pkg := range pkgs {
		if pkg.ImportPath != modulePath+"/llm" {
			continue
		}
		for _, dep := range pkg.Deps {
			if dep == "net/http" || dep == "net/http/httptest" ||
				dep == "crypto/tls" || strings.HasPrefix(dep, "golang.org/x/net/") {
				t.Errorf("package llm must carry no transport, found dep %q "+
					"(the transport belongs only in llm/ollama)", dep)
			}
			if dep == llmOllamaPath {
				t.Errorf("package llm must not import its own transport sub-package %q", dep)
			}
		}
		return
	}
	t.Fatal("package llm was not resolved by `go list`")
}

// TestLLMOllamaReachableOnlyFromCommandRoots is the positive assertion: the
// transport IS reachable, but ONLY from the actual top-level command roots. The
// roots are ENUMERATED from `go list ./...` rather than assumed (today that is
// cli/...; no cmd/... is assumed to exist), so the seam cannot silently drift
// back into package app or tui (design §E.6).
func TestLLMOllamaReachableOnlyFromCommandRoots(t *testing.T) {
	all := runGoListDeps(t, []string{"./..."})

	// Enumerate the real command roots: packages under cli/, cmd/, or apps/ (the
	// binary main lives in apps/bctx and legitimately imports cli/commands). We
	// enumerate rather than assume a single tree so the seam cannot silently
	// drift back into package app or tui.
	commandRoots := map[string]bool{}
	for _, pkg := range all {
		rel := strings.TrimPrefix(pkg.ImportPath, modulePath+"/")
		if strings.HasPrefix(rel, "cli/") || strings.HasPrefix(rel, "cmd/") ||
			strings.HasPrefix(rel, "apps/") {
			commandRoots[pkg.ImportPath] = true
		}
	}
	if len(commandRoots) == 0 {
		t.Fatal("no cli/, cmd/, or apps/ command roots were resolved; cannot verify reachability")
	}

	// Every importer of llm/ollama must be a command root (or llm/ollama itself).
	var badImporters []string
	reachableFromCommand := false
	for _, pkg := range all {
		if pkg.ImportPath == llmOllamaPath {
			continue
		}
		for _, dep := range pkg.Deps {
			if dep != llmOllamaPath {
				continue
			}
			if commandRoots[pkg.ImportPath] {
				reachableFromCommand = true
			} else {
				badImporters = append(badImporters, pkg.ImportPath)
			}
		}
	}
	if len(badImporters) > 0 {
		t.Fatalf("llm/ollama is reachable from non-command packages (must be cli/.. only):\n  %s",
			strings.Join(badImporters, "\n  "))
	}
	if !reachableFromCommand {
		t.Fatal("llm/ollama is not reachable from any command root; the composition " +
			"root wiring in cli/commands/home.go appears to be missing")
	}
}

// TestLLMTransportNotDirectlyImportedUnderConfinedRoots is the toolchain-free
// grep backstop for §E.6: no non-test .go file under tui/ (incl. tui/screens),
// reporting/, or any analysisDirs root may DIRECTLY import "net/http" or
// "github.com/bctx/bctx/llm/ollama".
func TestLLMTransportNotDirectlyImportedUnderConfinedRoots(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{`"net/http"`, `"net/http/httptest"`, `"` + llmOllamaPath + `"`}

	dirs := append([]string{"tui", "reporting"}, analysisDirs...)
	for _, dir := range dirs {
		base := filepath.Join(root, filepath.FromSlash(dir))
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			src := string(data)
			for _, f := range forbidden {
				if strings.Contains(src, f) {
					t.Errorf("confined file %s must not import %s "+
						"(transport belongs only in llm/ollama, imported only from cli/commands)", path, f)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
}

// TestAcquisitionBoundaryExists confirms the acquisition package exists and is
// the designated network boundary (documented contract).
func TestAcquisitionBoundaryExists(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "acquisition", "types.go")); err != nil {
		t.Fatalf("acquisition boundary package missing: %v", err)
	}
}
