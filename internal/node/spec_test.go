package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeInfoQualifiedIDPrefixesNamespaceForBareIDs(t *testing.T) {
	info := NodeInfo{
		ID:        "Runtime",
		Namespace: "tools",
	}

	if got := info.QualifiedID(); got != "tools.Runtime" {
		t.Fatalf("expected tools.Runtime, got %q", got)
	}
}

func TestNodeInfoQualifiedIDDoesNotDoublePrefixDottedIDs(t *testing.T) {
	info := NodeInfo{
		ID:        "tools.Runtime",
		Namespace: "tools",
	}

	if got := info.QualifiedID(); got != "tools.Runtime" {
		t.Fatalf("expected tools.Runtime, got %q", got)
	}
}

func TestNodeInfoQualifiedIDKeepsPathLikeSyncedIDCanonical(t *testing.T) {
	info := NodeInfo{
		ID:        "infra.config.env_lookup_os.osEnvLookup",
		Namespace: "config",
	}

	if got := info.QualifiedID(); got != "infra.config.env_lookup_os.osEnvLookup" {
		t.Fatalf("expected path-like ID to remain canonical, got %q", got)
	}
}

func TestLoadSchema11PreservesImplementationBehaviorContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Worker.yaml")
	content := `schema_version: "1.1"
node:
  id: Worker
  type: service
  file_path: worker.go
responsibility:
  summary: Execute one bounded unit of work.
  invariants:
    - Authoritative state changes only after validation.
interface:
  types:
    - name: Input
      signature: "type Input struct"
      description: Immutable worker request.
      fields:
        - name: ID
          type: string
          description: Stable request identity.
      values: [canonical]
  methods:
    - name: Execute
      signature: "Execute(input Input) (Output, error)"
      description: Validate and execute one input.
      parameters:
        - name: input
          type: Input
          description: Immutable request.
          constraint: Must be validated before use.
          examples: [valid-request]
      returns:
        type: Output
        description: Validated result.
        constraint: Never partially populated.
      throws:
        - type: InvalidInput
          condition: Validation fails.
      preconditions:
        - input is non-null
      postconditions:
        - output is canonical
      side_effects:
        - Commits state only after validation.
dependencies:
  - target: InputValidator
    type: interface
    injection: constructor
    usage: Validate the request.
    contract_hash: deadbeef
    requires: [Validate]
logic:
  rules:
    - name: atomic-commit
      condition: Validation succeeds.
      action: Commit the complete result.
implementation_contract:
  status: ready
  lifecycle:
    - One instance may execute multiple independent calls.
  constraints:
    - No implementation source is required by the packet.
  acceptance:
    - id: WORKER-VALID
      given: A valid immutable request.
      when: Execute is called.
      then:
        - One canonical result is returned.
metadata:
  status: specified
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write schema 1.1 fixture: %v", err)
	}

	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load schema 1.1 fixture: %v", err)
	}
	method := spec.Interface.Methods[0]
	if len(spec.Interface.Types) != 1 || spec.Interface.Types[0].Fields[0].Name != "ID" {
		t.Fatalf("public type contracts were not preserved: %+v", spec.Interface.Types)
	}
	if len(method.Preconditions) != 1 || len(method.Postconditions) != 1 || len(method.SideEffects) != 1 {
		t.Fatalf("behavior fields were not preserved: %+v", method)
	}
	if got := method.Parameters[0].Examples; len(got) != 1 || got[0] != "valid-request" {
		t.Fatalf("parameter examples were not preserved: %#v", got)
	}
	if method.Returns.Constraint != "Never partially populated." {
		t.Fatalf("return constraint was not preserved: %+v", method.Returns)
	}
	if len(spec.Dependencies[0].Requires) != 1 || spec.Dependencies[0].Requires[0] != "Validate" {
		t.Fatalf("dependency member requirements were not preserved: %+v", spec.Dependencies[0])
	}
	if len(spec.Logic.Rules) != 1 || spec.Logic.Rules[0].Name != "atomic-commit" {
		t.Fatalf("logic rules were not preserved: %+v", spec.Logic)
	}
	if spec.ImplementationContract == nil || spec.ImplementationContract.Status != "ready" || len(spec.ImplementationContract.Acceptance) != 1 {
		t.Fatalf("implementation contract was not preserved: %+v", spec.ImplementationContract)
	}

	roundTrip := filepath.Join(t.TempDir(), "Worker.yaml")
	if err := Save(roundTrip, spec); err != nil {
		t.Fatalf("save schema 1.1 fixture: %v", err)
	}
	reloaded, err := Load(roundTrip)
	if err != nil {
		t.Fatalf("reload schema 1.1 fixture: %v", err)
	}
	if reloaded.Interface.Methods[0].Postconditions[0] != "output is canonical" {
		t.Fatalf("schema 1.1 round trip lost behavior: %+v", reloaded.Interface.Methods[0])
	}
}

func TestLoadSchema11RejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Worker.yaml")
	content := `schema_version: "1.1"
node: {id: Worker, type: service}
responsibility: {summary: Execute work.}
interface:
  methods:
    - name: Execute
      signature: "Execute() error"
      description: Execute work.
      postconditons: [work completed]
metadata: {status: specified}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write strict fixture: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "postconditons") {
		t.Fatalf("expected field-specific strict parse error, got %v", err)
	}
}

func TestLoadSchema12ParsesSyncOwnershipAndAcceptanceCoverage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contract.yaml")
	content := `schema_version: "1.2"
node:
  id: Auth
  type: interface
responsibility:
  summary: Authenticate.
interface:
  methods:
    - name: LoginGuest
      signature: LoginGuest()
implementation_contract:
  status: sealed
  closed_world: true
  acceptance:
    - id: AUTH-001
      given: A player.
      when: LoginGuest runs.
      then: [Login succeeds.]
      covers:
        - symbol: Auth.LoginGuest
          aspects: [continuity]
sync_policy:
  default: authored
  ownership:
    interface.methods[*].signature: code
metadata:
  status: specified
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if spec.SyncPolicy == nil || spec.SyncPolicy.Ownership["interface.methods[*].signature"] != "code" {
		t.Fatalf("sync ownership was not parsed: %+v", spec.SyncPolicy)
	}
	covers := spec.ImplementationContract.Acceptance[0].Covers
	if len(covers) != 1 || covers[0].Symbol != "Auth.LoginGuest" || len(covers[0].Aspects) != 1 {
		t.Fatalf("acceptance coverage was not parsed: %+v", covers)
	}
}

func TestLoadSchema12PreservesProfiledReadinessContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Verifier.yaml")
	content := `schema_version: "1.2"
node: {id: Verifier, type: service}
responsibility: {summary: Verify one selected surface.}
interface:
  methods:
    - name: Verify
      signature: "Verify(profile string) error"
      description: Verify the selected surface.
      parameters:
        - {name: profile, type: string, description: Closed profile identifier.}
      returns: {type: error, description: Nil only for a green verdict.}
      postconditions: [Unknown profiles fail closed.]
implementation_contract:
  status: sealed
  closed_world: true
  constraints: [No undeclared surface may be selected.]
  acceptance:
    - id: HEADLESS-GREEN
      given: Valid vendored sources.
      when: Headless verification runs.
      then: [The verdict is green.]
  profiles:
    - id: headless
      description: Direct project-reference verification.
      requires: [vendored source identity]
      forbids: [publish plugin bytes]
      acceptance: [HEADLESS-GREEN]
  external_contracts:
    - id: modes
      path: contracts/modes.json
      contract_hash: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      description: Closed surface contract.
      profiles: [headless]
  gates:
    - id: design-approval
      kind: approval
      phase: implementation
      status: satisfied
      description: Independent design approval.
      profiles: [headless]
      contract: modes
metadata: {status: specified}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write schema 1.2 fixture: %v", err)
	}

	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load schema 1.2 fixture: %v", err)
	}
	contract := spec.ImplementationContract
	if contract == nil || !contract.ClosedWorld || contract.Status != "sealed" {
		t.Fatalf("schema 1.2 readiness metadata was not preserved: %+v", contract)
	}
	if len(contract.Profiles) != 1 || contract.Profiles[0].ID != "headless" || len(contract.Profiles[0].Forbids) != 1 {
		t.Fatalf("profile contract was not preserved: %+v", contract.Profiles)
	}
	if len(contract.ExternalContracts) != 1 || len(contract.Gates) != 1 || contract.Gates[0].Contract != "modes" {
		t.Fatalf("external contracts or gates were not preserved: %+v", contract)
	}

	roundTrip := filepath.Join(t.TempDir(), "Verifier.yaml")
	if err := Save(roundTrip, spec); err != nil {
		t.Fatalf("save schema 1.2 fixture: %v", err)
	}
	if _, err := Load(roundTrip); err != nil {
		t.Fatalf("reload schema 1.2 fixture: %v", err)
	}
}

func TestLoadSchema12RejectsUnknownProfileFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Verifier.yaml")
	content := `schema_version: "1.2"
node: {id: Verifier, type: service}
responsibility: {summary: Verify work.}
implementation_contract:
  status: ready
  profiles:
    - id: headless
      requirez: [source]
metadata: {status: specified}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write strict fixture: %v", err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "requirez") {
		t.Fatalf("expected schema 1.2 profile field rejection, got %v", err)
	}
}

func TestLoadSchema10RetainsPermissiveCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Legacy.yaml")
	content := `schema_version: "1.0"
node: {id: Legacy, type: class}
responsibility:
  summary: Preserve legacy extensions.
  legacy_extension: ignored
metadata: {status: draft}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write legacy fixture: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("schema 1.0 compatibility regressed: %v", err)
	}
}
