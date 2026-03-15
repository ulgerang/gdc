package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <node>",
	Short: "Compare a YAML spec against the current code",
	Long: `Compare the stored YAML spec for a node with the implementation found at file_path.

This command reports signature drift between the spec and current code without
rewriting the YAML.

Examples:
  $ gdc diff Agent
  $ gdc diff Agent --config .gdc/config.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runDiff,
}

func runDiff(cmd *cobra.Command, args []string) error {
	nodeName := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	nodesDir := cfg.NodesDir()
	nodePath := filepath.Join(nodesDir, nodeName+".yaml")
	spec, err := node.Load(nodePath)
	if err != nil {
		return fmt.Errorf("node %s not found: %w", nodeName, err)
	}
	if strings.TrimSpace(spec.Node.FilePath) == "" {
		return fmt.Errorf("node %s has no file_path to diff against", nodeName)
	}

	p, err := parser.GetParser(cfg.Project.Language)
	if err != nil {
		return fmt.Errorf("failed to load parser for %s: %w", cfg.Project.Language, err)
	}

	resolvedPath := cfg.ResolvePath(spec.Node.FilePath)
	extracted, err := findExtractedNodeInFile(p, resolvedPath, spec.Node.ID)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", spec.Node.FilePath, err)
	}
	if extracted == nil {
		return fmt.Errorf("symbol %s not found in %s", spec.Node.ID, spec.Node.FilePath)
	}

	report := buildDriftReport(spec, extracted)
	if report.isEmpty() {
		printSuccess("No drift detected for %s", spec.Node.ID)
		return nil
	}

	printDriftReport(spec, report)
	return nil
}

type driftReport struct {
	MissingMethods      []string
	ExtraMethods        []string
	MethodMismatches    []signatureDrift
	MissingProperties   []string
	ExtraProperties     []string
	PropertyMismatches  []typeDrift
	MissingEvents       []string
	ExtraEvents         []string
	EventMismatches     []signatureDrift
	MissingConstructors []string
	ExtraConstructors   []string
	MissingDeps         []string
	ExtraDeps           []string
}

type signatureDrift struct {
	Name          string
	SpecSignature string
	CodeSignature string
}

type typeDrift struct {
	Name     string
	SpecType string
	CodeType string
}

func (r driftReport) isEmpty() bool {
	return len(r.MissingMethods) == 0 &&
		len(r.ExtraMethods) == 0 &&
		len(r.MethodMismatches) == 0 &&
		len(r.MissingProperties) == 0 &&
		len(r.ExtraProperties) == 0 &&
		len(r.PropertyMismatches) == 0 &&
		len(r.MissingEvents) == 0 &&
		len(r.ExtraEvents) == 0 &&
		len(r.EventMismatches) == 0 &&
		len(r.MissingConstructors) == 0 &&
		len(r.ExtraConstructors) == 0 &&
		len(r.MissingDeps) == 0 &&
		len(r.ExtraDeps) == 0
}

func buildDriftReport(spec *node.Spec, extracted *parser.ExtractedNode) driftReport {
	report := driftReport{}

	specMethods := make(map[string]node.Method, len(spec.Interface.Methods))
	codeMethods := make(map[string]parser.ExtractedMethod, len(extracted.Methods))
	for _, method := range spec.Interface.Methods {
		specMethods[method.Name] = method
	}
	for _, method := range extracted.Methods {
		codeMethods[method.Name] = method
	}

	for name, specMethod := range specMethods {
		codeMethod, ok := codeMethods[name]
		if !ok {
			report.MissingMethods = append(report.MissingMethods, name)
			continue
		}
		if !signaturesCompatible(specMethod.Signature, codeMethod.Signature) {
			report.MethodMismatches = append(report.MethodMismatches, signatureDrift{
				Name:          name,
				SpecSignature: specMethod.Signature,
				CodeSignature: codeMethod.Signature,
			})
		}
	}
	for name := range codeMethods {
		if _, ok := specMethods[name]; !ok {
			report.ExtraMethods = append(report.ExtraMethods, name)
		}
	}

	specProps := make(map[string]node.Property, len(spec.Interface.Properties))
	codeProps := make(map[string]parser.ExtractedProperty, len(extracted.Properties))
	for _, prop := range spec.Interface.Properties {
		specProps[prop.Name] = prop
	}
	for _, prop := range extracted.Properties {
		codeProps[prop.Name] = prop
	}
	for name, specProp := range specProps {
		codeProp, ok := codeProps[name]
		if !ok {
			report.MissingProperties = append(report.MissingProperties, name)
			continue
		}
		if !typesCompatible(specProp.Type, codeProp.Type) {
			report.PropertyMismatches = append(report.PropertyMismatches, typeDrift{
				Name:     name,
				SpecType: specProp.Type,
				CodeType: codeProp.Type,
			})
		}
	}
	for name := range codeProps {
		if _, ok := specProps[name]; !ok {
			report.ExtraProperties = append(report.ExtraProperties, name)
		}
	}

	specEvents := make(map[string]node.Event, len(spec.Interface.Events))
	codeEvents := make(map[string]parser.ExtractedEvent, len(extracted.Events))
	for _, event := range spec.Interface.Events {
		specEvents[event.Name] = event
	}
	for _, event := range extracted.Events {
		codeEvents[event.Name] = event
	}
	for name, specEvent := range specEvents {
		codeEvent, ok := codeEvents[name]
		if !ok {
			report.MissingEvents = append(report.MissingEvents, name)
			continue
		}
		if !signaturesCompatible(specEvent.Signature, codeEvent.Signature) {
			report.EventMismatches = append(report.EventMismatches, signatureDrift{
				Name:          name,
				SpecSignature: specEvent.Signature,
				CodeSignature: codeEvent.Signature,
			})
		}
	}
	for name := range codeEvents {
		if _, ok := specEvents[name]; !ok {
			report.ExtraEvents = append(report.ExtraEvents, name)
		}
	}

	specCtorSigs := make(map[string]bool, len(spec.Interface.Constructors))
	codeCtorSigs := make(map[string]bool, len(extracted.Constructors))
	for _, ctor := range spec.Interface.Constructors {
		specCtorSigs[normalizeSignature(ctor.Signature)] = true
	}
	for _, ctor := range extracted.Constructors {
		codeCtorSigs[normalizeSignature(ctor.Signature)] = true
	}
	for _, ctor := range spec.Interface.Constructors {
		sig := normalizeSignature(ctor.Signature)
		if !codeCtorSigs[sig] {
			report.MissingConstructors = append(report.MissingConstructors, ctor.Signature)
		}
	}
	for _, ctor := range extracted.Constructors {
		sig := normalizeSignature(ctor.Signature)
		if !specCtorSigs[sig] {
			report.ExtraConstructors = append(report.ExtraConstructors, ctor.Signature)
		}
	}

	specDeps := make(map[string]bool, len(spec.Dependencies))
	codeDeps := make(map[string]bool, len(extracted.Dependencies))
	for _, dep := range spec.Dependencies {
		specDeps[dep.Target] = true
	}
	for _, dep := range extracted.Dependencies {
		codeDeps[dep.Target] = true
	}
	for dep := range specDeps {
		if !codeDeps[dep] {
			report.MissingDeps = append(report.MissingDeps, dep)
		}
	}
	for dep := range codeDeps {
		if !specDeps[dep] {
			report.ExtraDeps = append(report.ExtraDeps, dep)
		}
	}

	sort.Strings(report.MissingMethods)
	sort.Strings(report.ExtraMethods)
	sort.Strings(report.MissingProperties)
	sort.Strings(report.ExtraProperties)
	sort.Strings(report.MissingEvents)
	sort.Strings(report.ExtraEvents)
	sort.Strings(report.MissingConstructors)
	sort.Strings(report.ExtraConstructors)
	sort.Strings(report.MissingDeps)
	sort.Strings(report.ExtraDeps)
	sort.Slice(report.MethodMismatches, func(i, j int) bool { return report.MethodMismatches[i].Name < report.MethodMismatches[j].Name })
	sort.Slice(report.PropertyMismatches, func(i, j int) bool { return report.PropertyMismatches[i].Name < report.PropertyMismatches[j].Name })
	sort.Slice(report.EventMismatches, func(i, j int) bool { return report.EventMismatches[i].Name < report.EventMismatches[j].Name })

	return report
}

func printDriftReport(spec *node.Spec, report driftReport) {
	fmt.Printf("Drift detected for %s\n\n", spec.Node.ID)

	for _, mismatch := range report.MethodMismatches {
		fmt.Printf("Method %q\n", mismatch.Name)
		fmt.Printf("  YAML spec:  %s\n", mismatch.SpecSignature)
		fmt.Printf("  Code actual: %s\n\n", mismatch.CodeSignature)
	}
	for _, mismatch := range report.PropertyMismatches {
		fmt.Printf("Property %q\n", mismatch.Name)
		fmt.Printf("  YAML spec:  %s\n", mismatch.SpecType)
		fmt.Printf("  Code actual: %s\n\n", mismatch.CodeType)
	}
	for _, mismatch := range report.EventMismatches {
		fmt.Printf("Event %q\n", mismatch.Name)
		fmt.Printf("  YAML spec:  %s\n", mismatch.SpecSignature)
		fmt.Printf("  Code actual: %s\n\n", mismatch.CodeSignature)
	}

	printDriftList("Missing methods in code", report.MissingMethods)
	printDriftList("Extra methods in code", report.ExtraMethods)
	printDriftList("Missing properties in code", report.MissingProperties)
	printDriftList("Extra properties in code", report.ExtraProperties)
	printDriftList("Missing events in code", report.MissingEvents)
	printDriftList("Extra events in code", report.ExtraEvents)
	printDriftList("Missing constructors in code", report.MissingConstructors)
	printDriftList("Extra constructors in code", report.ExtraConstructors)
	printDriftList("Missing dependencies in code", report.MissingDeps)
	printDriftList("Extra dependencies in code", report.ExtraDeps)
}

func printDriftList(title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Println(title)
	for _, value := range values {
		fmt.Printf("  - %s\n", value)
	}
	fmt.Println()
}
