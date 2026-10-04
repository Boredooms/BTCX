// Package reporting builds and renders forensic investigation reports from
// local evidence only. It is NETWORK-FORBIDDEN: nothing in this package (or its
// sub-packages) may import net/http, acquisition adapters, provider SDKs, or
// the investigation orchestrator. Reports are assembled from an
// already-produced schema.InvestigationResult plus sdk.Repository reads.
package reporting

// ReportSchemaVersion and GeneratorVersion are opaque, equality-compared
// version tokens stamped onto every report and its snapshot.
//
// They are NOT semantic version numbers to be parsed or range-compared. Any
// change to the meaning, shape, or wording of a report — or to the way a
// snapshot is built or hashed — requires minting a NEW string here. The
// existing string must never be silently reused for a changed meaning, so that
// a consumer comparing version tokens can always trust that an identical token
// implies an identical report contract.
const (
	// ReportSchemaVersion identifies the report/snapshot data contract.
	ReportSchemaVersion = "report-schema-v1"
	// GeneratorVersion identifies the generator implementation that produced
	// the report (rendering + snapshot assembly rules).
	GeneratorVersion = "report-gen-v1"
)
