package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gdc-tools/gdc/internal/node"
	"gopkg.in/yaml.v3"
)

const (
	syncOwnerCode     = "code"
	syncOwnerAuthored = "authored"
)

type syncSemanticChange struct {
	Path        string
	Ownership   string
	Disposition string
	Before      string
	After       string
}

type syncSemanticReport struct {
	Changes           []syncSemanticChange
	CodeOwnedChanges  int
	AuthoredChanges   int
	ReviewRequired    int
	AuthoredPreserved int
}

func analyzeCodeSyncChanges(existing, next *node.Spec) syncSemanticReport {
	before := flattenSpecLeaves(existing)
	after := flattenSpecLeaves(next)
	paths := make(map[string]bool, len(before)+len(after))
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}

	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	report := syncSemanticReport{}
	for _, path := range ordered {
		oldValue, oldExists := before[path]
		newValue, newExists := after[path]
		owner, disposition := syncOwnershipForPath(existing, path)
		if oldExists && newExists && oldValue == newValue {
			if owner == syncOwnerAuthored {
				report.AuthoredPreserved++
			}
			continue
		}
		change := syncSemanticChange{
			Path: path, Ownership: owner, Disposition: disposition,
			Before: semanticDisplayValue(oldValue, oldExists),
			After:  semanticDisplayValue(newValue, newExists),
		}
		report.Changes = append(report.Changes, change)
		switch disposition {
		case "review_required":
			report.ReviewRequired++
		case "mechanical":
			report.CodeOwnedChanges++
		default:
			report.AuthoredChanges++
		}
	}
	return report
}

func semanticDisplayValue(value string, exists bool) string {
	if !exists {
		return "<absent>"
	}
	return value
}

func flattenSpecLeaves(spec *node.Spec) map[string]string {
	result := make(map[string]string)
	if spec == nil {
		return result
	}
	encoded, err := yaml.Marshal(spec)
	if err != nil {
		return result
	}
	var value any
	if err := yaml.Unmarshal(encoded, &value); err != nil {
		return result
	}
	flattenSemanticValue(value, "", result)
	return result
}

func flattenSemanticValue(value any, path string, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			flattenSemanticValue(typed[key], joinSemanticPath(path, key), out)
		}
	case []any:
		for index, item := range typed {
			segment := strconv.Itoa(index)
			if mapping, ok := item.(map[string]any); ok {
				if identity := semanticIdentity(mapping); identity != "" {
					segment = identity
				}
			}
			flattenSemanticValue(item, path+"["+sanitizeSemanticIdentity(segment)+"]", out)
		}
	case nil:
		out[path] = "null"
	default:
		out[path] = fmt.Sprint(typed)
	}
}

func semanticIdentity(mapping map[string]any) string {
	for _, key := range []string{"name", "id", "target", "signature"} {
		if value, ok := mapping[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func sanitizeSemanticIdentity(value string) string {
	value = strings.ReplaceAll(value, "]", "\\]")
	return strings.ReplaceAll(value, "\n", " ")
}

func joinSemanticPath(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "." + segment
}

func syncOwnershipForPath(spec *node.Spec, path string) (string, string) {
	// External contract hashes are explicit human review receipts. A code-sync
	// policy cannot downgrade them to mechanically writable data.
	if strings.HasPrefix(path, "implementation_contract.external_contracts[") && strings.HasSuffix(path, ".contract_hash") {
		return syncOwnerAuthored, "review_required"
	}

	if spec != nil && spec.SyncPolicy != nil {
		if owner := explicitSyncOwner(spec.SyncPolicy.Ownership, path); owner != "" {
			if owner == syncOwnerCode {
				return owner, "mechanical"
			}
			return owner, "authored"
		}
	}
	if builtInCodeOwnedPath(path) {
		return syncOwnerCode, "mechanical"
	}
	if spec != nil && spec.SyncPolicy != nil && spec.SyncPolicy.Default != "" {
		if spec.SyncPolicy.Default == syncOwnerCode {
			return syncOwnerCode, "mechanical"
		}
		return syncOwnerAuthored, "authored"
	}
	if spec != nil && spec.Metadata.Origin == "code_extracted" && spec.ImplementationContract == nil {
		// Legacy extraction-only nodes remain mechanically refreshable until a
		// curator claims ownership or adds an implementation contract.
		return syncOwnerCode, "mechanical"
	}
	return syncOwnerAuthored, "authored"
}

func explicitSyncOwner(rules map[string]string, path string) string {
	if len(rules) == 0 {
		return ""
	}
	patterns := make([]string, 0, len(rules))
	for pattern := range rules {
		patterns = append(patterns, pattern)
	}
	sort.SliceStable(patterns, func(i, j int) bool { return len(patterns[i]) > len(patterns[j]) })
	for _, pattern := range patterns {
		if syncPathPatternMatches(pattern, path) {
			return rules[pattern]
		}
	}
	return ""
}

func syncPathPatternMatches(pattern, path string) bool {
	expression := regexp.QuoteMeta(strings.TrimSpace(pattern))
	expression = strings.ReplaceAll(expression, `\[\*\]`, `\[[^\]]+\]`)
	expression = strings.ReplaceAll(expression, `\*`, `[^.]+`)
	matched, err := regexp.MatchString("^"+expression+"$", path)
	return err == nil && matched
}

func builtInCodeOwnedPath(path string) bool {
	if path == "node.id" || path == "node.type" || path == "node.namespace" || path == "node.file_path" {
		return true
	}
	if strings.HasPrefix(path, "language_spec.") {
		return true
	}
	if path == "metadata.origin" || path == "metadata.extracted_at" {
		return true
	}
	patterns := []string{
		`interface.constructors[*].signature`,
		`interface.constructors[*].parameters[*].name`,
		`interface.constructors[*].parameters[*].type`,
		`interface.constructors[*].access`,
		`interface.constructors[*].attributes[*]`,
		`interface.methods[*].name`,
		`interface.methods[*].signature`,
		`interface.methods[*].parameters[*].name`,
		`interface.methods[*].parameters[*].type`,
		`interface.methods[*].returns.type`,
		`interface.methods[*].async`,
		`interface.methods[*].access`,
		`interface.methods[*].exported`,
		`interface.methods[*].static`,
		`interface.methods[*].virtual`,
		`interface.methods[*].abstract`,
		`interface.methods[*].attributes[*]`,
		`interface.properties[*].name`,
		`interface.properties[*].type`,
		`interface.properties[*].access`,
		`interface.properties[*].readonly`,
		`interface.properties[*].exported`,
		`interface.properties[*].static`,
		`interface.properties[*].attributes[*]`,
		`interface.events[*].name`,
		`interface.events[*].signature`,
		`dependencies[*].target`,
		`dependencies[*].type`,
		`dependencies[*].injection`,
	}
	for _, pattern := range patterns {
		if syncPathPatternMatches(pattern, path) {
			return true
		}
	}
	return false
}

func formatSemanticChange(change syncSemanticChange) string {
	return fmt.Sprintf("%s [%s/%s]: %s -> %s", change.Path, change.Ownership, change.Disposition, change.Before, change.After)
}
