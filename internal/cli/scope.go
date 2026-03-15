package cli

import (
	"path/filepath"
	"strings"

	"github.com/gdc-tools/gdc/internal/config"
	"github.com/gdc-tools/gdc/internal/db"
	"github.com/gdc-tools/gdc/internal/node"
	"github.com/gdc-tools/gdc/internal/parser"
)

type syncScope struct {
	projectRoot string
	nodesDir    string
	files       map[string]bool
	dirs        []string
	symbols     []string
}

func newSyncScope(cfg *config.Config, files, dirs, symbols []string) *syncScope {
	scope := &syncScope{
		projectRoot: cfg.ProjectRoot,
		nodesDir:    cfg.NodesDir(),
		files:       make(map[string]bool),
	}

	for _, file := range files {
		if normalized := normalizeProjectPath(cfg.ProjectRoot, file); normalized != "" {
			scope.files[normalized] = true
		}
	}

	for _, dir := range dirs {
		if normalized := normalizeProjectPath(cfg.ProjectRoot, dir); normalized != "" {
			scope.dirs = append(scope.dirs, normalized)
		}
	}

	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		scope.symbols = append(scope.symbols, strings.ToLower(symbol))
	}

	return scope
}

func (s *syncScope) active() bool {
	return len(s.files) > 0 || len(s.dirs) > 0 || len(s.symbols) > 0
}

func (s *syncScope) hasFileScope() bool {
	return len(s.files) > 0 || len(s.dirs) > 0
}

func (s *syncScope) hasSymbolScope() bool {
	return len(s.symbols) > 0
}

func (s *syncScope) matchesSourceFile(path string) bool {
	if !s.hasFileScope() {
		return true
	}

	normalized := normalizeProjectPath(s.projectRoot, path)
	if normalized == "" {
		return false
	}
	if s.files[normalized] {
		return true
	}
	for _, dir := range s.dirs {
		if pathWithinScope(normalized, dir) {
			return true
		}
	}
	return false
}

func (s *syncScope) matchesNode(spec *node.Spec) bool {
	if spec == nil {
		return false
	}
	return s.matchesLookupTargets(buildNodeLookupTargets(spec, s.projectRoot, s.nodesDir))
}

func (s *syncScope) matchesStoredNode(record *db.NodeRecord) bool {
	if record == nil {
		return false
	}

	targets := []string{record.ID}
	if record.Namespace != "" {
		targets = append(targets, record.Namespace+"."+record.ID)
	}
	if record.ImplPath != "" {
		targets = append(targets, projectPathVariants(record.ImplPath, s.projectRoot)...)
	}
	if record.SpecPath != "" {
		targets = append(targets, projectPathVariants(record.SpecPath, s.projectRoot)...)
	} else if s.nodesDir != "" {
		targets = append(targets, projectPathVariants(filepath.Join(s.nodesDir, record.ID+".yaml"), s.projectRoot)...)
	}

	return s.matchesLookupTargets(targets)
}

func (s *syncScope) matchesExtractedNode(extracted *parser.ExtractedNode) bool {
	if extracted == nil {
		return false
	}

	targets := []string{extracted.ID}
	if extracted.Namespace != "" {
		targets = append(targets, extracted.Namespace+"."+extracted.ID)
	}
	if extracted.FilePath != "" {
		targets = append(targets, projectPathVariants(extracted.FilePath, s.projectRoot)...)
	}

	return s.matchesLookupTargets(targets)
}

func (s *syncScope) matchesLookupTargets(targets []string) bool {
	if !s.active() {
		return true
	}

	fileMatched := !s.hasFileScope()
	symbolMatched := !s.hasSymbolScope()

	for _, target := range dedupeStrings(targets) {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}

		normalizedTarget := strings.ToLower(target)
		if !fileMatched && s.matchesSourceFile(target) {
			fileMatched = true
		}

		if !symbolMatched {
			for _, symbol := range s.symbols {
				if normalizedTarget == symbol || strings.Contains(normalizedTarget, symbol) {
					symbolMatched = true
					break
				}
			}
		}

		if fileMatched && symbolMatched {
			return true
		}
	}

	return fileMatched && symbolMatched
}

func normalizeProjectPath(projectRoot, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) && projectRoot != "" {
		path = filepath.Join(projectRoot, path)
	}
	return normalizeComparablePath(path)
}

func normalizeComparablePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func pathWithinScope(path, dir string) bool {
	path = normalizeComparablePath(path)
	dir = normalizeComparablePath(dir)
	if path == "" || dir == "" {
		return false
	}
	if path == dir {
		return true
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func projectPathVariants(path, projectRoot string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	var variants []string
	normalized := normalizeComparablePath(path)
	if normalized != "" {
		variants = append(variants, normalized)
		variants = append(variants, normalizeComparablePath(filepath.ToSlash(path)))
	}

	if projectRoot != "" {
		absolute := path
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(projectRoot, path)
		}
		if normalizedAbsolute := normalizeComparablePath(absolute); normalizedAbsolute != "" {
			variants = append(variants, normalizedAbsolute)
		}

		relativeToRoot, err := filepath.Rel(projectRoot, absolute)
		if err == nil {
			variants = append(variants, normalizeComparablePath(relativeToRoot))
			variants = append(variants, normalizeComparablePath(filepath.ToSlash(relativeToRoot)))
		}
	}

	base := filepath.Base(path)
	if base != "" && base != "." {
		variants = append(variants, normalizeComparablePath(base))
	}

	return dedupeStrings(variants)
}

func buildNodeLookupTargets(spec *node.Spec, projectRoot, nodesDir string) []string {
	if spec == nil {
		return nil
	}

	targets := []string{spec.Node.ID}
	if spec.Node.Namespace != "" {
		targets = append(targets, spec.Node.Namespace+"."+spec.Node.ID)
	}
	if spec.Node.FilePath != "" {
		targets = append(targets, projectPathVariants(spec.Node.FilePath, projectRoot)...)
	}
	if nodesDir != "" {
		targets = append(targets, projectPathVariants(filepath.Join(nodesDir, spec.Node.ID+".yaml"), projectRoot)...)
	}

	return dedupeStrings(targets)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
