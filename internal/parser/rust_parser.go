package parser

import (
	"os"
	"regexp"
	"strings"
)

var (
	rustTraitPattern          = regexp.MustCompile(`^\s*pub\s+trait\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustStructPattern         = regexp.MustCompile(`^\s*pub\s+struct\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustEnumPattern           = regexp.MustCompile(`^\s*pub\s+enum\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustTopLevelFunctionRegex = regexp.MustCompile(`^\s*pub\s+(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustImplPattern           = regexp.MustCompile(`^\s*impl(?:\s+([A-Za-z_][A-Za-z0-9_]*)\s+for\s+)?\s*([A-Za-z_][A-Za-z0-9_]*)`)
	rustDocCommentPattern     = regexp.MustCompile(`^\s*///\s?(.*)$`)
)

// RustParser parses Rust source files using lightweight heuristics.
type RustParser struct{}

// NewRustParser creates a new Rust parser.
func NewRustParser() *RustParser {
	return &RustParser{}
}

// Language returns "rust".
func (p *RustParser) Language() string {
	return "rust"
}

// ParseFile parses a Rust source file and returns the first extracted node.
func (p *RustParser) ParseFile(filePath string) (*ExtractedNode, error) {
	nodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return &ExtractedNode{
			FilePath: filePath,
			Language: "rust",
		}, nil
	}
	return nodes[0], nil
}

// ParseFileNodes parses a Rust source file and extracts all public nodes.
func (p *RustParser) ParseFileNodes(filePath string) ([]*ExtractedNode, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	nodes := make([]*ExtractedNode, 0)
	nodeByID := make(map[string]*ExtractedNode)
	pendingDoc := ""

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripRustLineComment(lines[i]))
		if line == "" {
			continue
		}

		if doc := parseRustDocLine(lines[i]); doc != "" {
			if pendingDoc != "" {
				pendingDoc += " "
			}
			pendingDoc += doc
			continue
		}

		if matches := rustTraitPattern.FindStringSubmatch(line); len(matches) > 1 {
			blockLines, end := captureRustBlock(lines, i)
			extracted := &ExtractedNode{
				ID:          matches[1],
				Type:        "interface",
				Language:    "rust",
				FilePath:    filePath,
				Description: pendingDoc,
			}
			p.parseRustTraitBlock(blockLines, extracted)
			nodes = append(nodes, extracted)
			nodeByID[extracted.ID] = extracted
			pendingDoc = ""
			i = end
			continue
		}

		if matches := rustStructPattern.FindStringSubmatch(line); len(matches) > 1 {
			extracted := &ExtractedNode{
				ID:          matches[1],
				Type:        "class",
				Language:    "rust",
				FilePath:    filePath,
				Description: pendingDoc,
			}
			if strings.Contains(line, "{") {
				blockLines, end := captureRustBlock(lines, i)
				p.parseRustStructBlock(blockLines, extracted)
				i = end
			}
			nodes = append(nodes, extracted)
			nodeByID[extracted.ID] = extracted
			pendingDoc = ""
			continue
		}

		if matches := rustEnumPattern.FindStringSubmatch(line); len(matches) > 1 {
			extracted := &ExtractedNode{
				ID:          matches[1],
				Type:        "class",
				Language:    "rust",
				FilePath:    filePath,
				Description: pendingDoc,
			}
			nodes = append(nodes, extracted)
			nodeByID[extracted.ID] = extracted
			if strings.Contains(line, "{") {
				_, end := captureRustBlock(lines, i)
				i = end
			}
			pendingDoc = ""
			continue
		}

		if matches := rustTopLevelFunctionRegex.FindStringSubmatch(line); len(matches) > 1 {
			signature, end := collectRustSignature(lines, i)
			method, ok := parseRustMethodSignature(signature, true)
			if !ok {
				pendingDoc = ""
				i = end
				continue
			}

			functionNode := &ExtractedNode{
				ID:          matches[1],
				Type:        "function",
				Language:    "rust",
				FilePath:    filePath,
				Description: pendingDoc,
				Methods: []ExtractedMethod{
					method,
				},
			}
			nodes = append(nodes, functionNode)
			nodeByID[functionNode.ID] = functionNode
			pendingDoc = ""
			i = end
			continue
		}

		if matches := rustImplPattern.FindStringSubmatch(line); len(matches) > 2 && strings.Contains(line, "{") {
			blockLines, end := captureRustBlock(lines, i)
			traitName := strings.TrimSpace(matches[1])
			owner := strings.TrimSpace(matches[2])
			node := nodeByID[owner]
			if node == nil {
				node = &ExtractedNode{
					ID:       owner,
					Type:     "class",
					Language: "rust",
					FilePath: filePath,
				}
				nodes = append(nodes, node)
				nodeByID[owner] = node
			}
			p.parseRustImplBlock(blockLines, traitName, node)
			pendingDoc = ""
			i = end
			continue
		}

		pendingDoc = ""
	}

	return nodes, nil
}

func (p *RustParser) parseRustTraitBlock(lines []string, node *ExtractedNode) {
	pendingDoc := ""
	for i := 1; i < len(lines)-1; i++ {
		if doc := parseRustDocLine(lines[i]); doc != "" {
			if pendingDoc != "" {
				pendingDoc += " "
			}
			pendingDoc += doc
			continue
		}

		line := strings.TrimSpace(stripRustLineComment(lines[i]))
		if line == "" {
			continue
		}
		if !strings.Contains(line, "fn ") {
			pendingDoc = ""
			continue
		}

		signature, end := collectRustSignature(lines, i)
		method, ok := parseRustMethodSignature(signature, true)
		if ok {
			method.Description = pendingDoc
			node.Methods = append(node.Methods, method)
		}
		pendingDoc = ""
		i = end
	}
}

func (p *RustParser) parseRustStructBlock(lines []string, node *ExtractedNode) {
	for _, rawLine := range lines[1 : len(lines)-1] {
		line := strings.TrimSpace(stripRustLineComment(rawLine))
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ",") {
			line = strings.TrimSuffix(line, ",")
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx <= 0 {
			continue
		}

		fieldName := strings.TrimSpace(strings.TrimPrefix(line[:colonIdx], "pub "))
		typeRef := strings.TrimSpace(line[colonIdx+1:])
		isPublic := strings.HasPrefix(strings.TrimSpace(line), "pub ")
		if isPublic {
			node.Properties = append(node.Properties, ExtractedProperty{
				Name:     fieldName,
				Type:     typeRef,
				Access:   "get; set",
				IsPublic: true,
			})
		}

		for _, depType := range extractRustDependencyTypes(typeRef) {
			node.Dependencies = mergeDependencies(node.Dependencies, []ExtractedDependency{{
				Target:    depType,
				Type:      "interface",
				FieldName: fieldName,
				Injection: "field",
			}})
		}
	}
}

func (p *RustParser) parseRustImplBlock(lines []string, traitName string, node *ExtractedNode) {
	if traitName != "" {
		node.Dependencies = mergeDependencies(node.Dependencies, []ExtractedDependency{{
			Target:    traitName,
			Type:      "interface",
			Injection: "implements",
		}})
	}

	pendingDoc := ""
	for i := 1; i < len(lines)-1; i++ {
		if doc := parseRustDocLine(lines[i]); doc != "" {
			if pendingDoc != "" {
				pendingDoc += " "
			}
			pendingDoc += doc
			continue
		}

		line := strings.TrimSpace(stripRustLineComment(lines[i]))
		if line == "" || !strings.Contains(line, "fn ") {
			continue
		}

		shouldExtract := strings.Contains(line, "pub fn ") || strings.Contains(line, "pub async fn ")
		if traitName != "" {
			shouldExtract = strings.Contains(line, "fn ")
		}
		if !shouldExtract {
			pendingDoc = ""
			continue
		}

		signature, end := collectRustSignature(lines, i)
		method, ok := parseRustMethodSignature(signature, true)
		if ok {
			method.Description = pendingDoc
			if isRustConstructor(method, node.ID) {
				node.Constructors = append(node.Constructors, ExtractedConstructor{
					Signature:   method.Signature,
					Parameters:  method.Parameters,
					Description: pendingDoc,
				})
				node.Dependencies = mergeDependencies(node.Dependencies, rustDependenciesFromParameters(method.Parameters, "constructor"))
			} else {
				node.Methods = append(node.Methods, method)
				node.Dependencies = mergeDependencies(node.Dependencies, rustDependenciesFromParameters(method.Parameters, "method"))
			}
		}

		pendingDoc = ""
		i = end
	}
}

func parseRustDocLine(line string) string {
	matches := rustDocCommentPattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func captureRustBlock(lines []string, start int) ([]string, int) {
	block := make([]string, 0)
	depth := 0
	foundOpening := false

	for i := start; i < len(lines); i++ {
		line := lines[i]
		block = append(block, line)
		for _, ch := range line {
			switch ch {
			case '{':
				depth++
				foundOpening = true
			case '}':
				depth--
				if foundOpening && depth == 0 {
					return block, i
				}
			}
		}
	}

	return block, len(lines) - 1
}

func collectRustSignature(lines []string, start int) (string, int) {
	var parts []string
	parens := 0
	angles := 0

	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(stripRustLineComment(lines[i]))
		if line == "" {
			continue
		}
		parts = append(parts, line)
		for _, ch := range line {
			switch ch {
			case '(':
				parens++
			case ')':
				if parens > 0 {
					parens--
				}
			case '<':
				angles++
			case '>':
				if angles > 0 {
					angles--
				}
			}
		}

		trimmed := strings.TrimSpace(line)
		if parens == 0 && angles == 0 && (strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, ";")) {
			signature := strings.Join(parts, " ")
			signature = strings.TrimSpace(strings.TrimSuffix(signature, "{"))
			signature = strings.TrimSpace(strings.TrimSuffix(signature, ";"))
			return collapseWhitespace(signature), i
		}
	}

	return collapseWhitespace(strings.Join(parts, " ")), len(lines) - 1
}

func parseRustMethodSignature(signature string, isPublic bool) (ExtractedMethod, bool) {
	signature = collapseWhitespace(signature)
	fnIdx := strings.Index(signature, "fn ")
	if fnIdx < 0 {
		return ExtractedMethod{}, false
	}

	prefix := strings.TrimSpace(signature[:fnIdx])
	remainder := signature[fnIdx+3:]
	openParen := strings.Index(remainder, "(")
	if openParen <= 0 {
		return ExtractedMethod{}, false
	}

	name := strings.TrimSpace(remainder[:openParen])
	paramsStart := openParen
	paramsEnd := findMatchingParen(remainder, paramsStart)
	if paramsEnd < 0 {
		return ExtractedMethod{}, false
	}

	paramsRaw := remainder[paramsStart+1 : paramsEnd]
	afterParams := strings.TrimSpace(remainder[paramsEnd+1:])
	returns := ""
	if strings.HasPrefix(afterParams, "->") {
		returns = strings.TrimSpace(strings.TrimPrefix(afterParams, "->"))
	}

	parameters := parseRustParameters(paramsRaw)
	method := ExtractedMethod{
		Name:       name,
		Signature:  buildRustSignature(name, paramsRaw, returns),
		Parameters: parameters,
		Returns:    returns,
		IsPublic:   isPublic,
		Async:      strings.Contains(prefix, "async"),
		Access:     "public",
	}

	return method, true
}

func buildRustSignature(name, params, returns string) string {
	signature := name + "(" + collapseWhitespace(strings.TrimSpace(params)) + ")"
	if returns != "" {
		signature += " -> " + collapseWhitespace(returns)
	}
	return signature
}

func parseRustParameters(params string) []ExtractedParameter {
	parts := splitRustCommaList(params)
	result := make([]ExtractedParameter, 0, len(parts))

	for _, part := range parts {
		part = collapseWhitespace(strings.TrimSpace(part))
		if part == "" || strings.Contains(part, "self") {
			continue
		}

		colonIdx := strings.Index(part, ":")
		if colonIdx <= 0 {
			continue
		}

		name := strings.TrimSpace(strings.TrimPrefix(part[:colonIdx], "mut "))
		typeRef := strings.TrimSpace(part[colonIdx+1:])
		result = append(result, ExtractedParameter{
			Name: name,
			Type: typeRef,
		})
	}

	return result
}

func rustDependenciesFromParameters(params []ExtractedParameter, injection string) []ExtractedDependency {
	deps := make([]ExtractedDependency, 0)
	for _, param := range params {
		for _, depType := range extractRustDependencyTypes(param.Type) {
			deps = append(deps, ExtractedDependency{
				Target:    depType,
				Type:      "interface",
				FieldName: param.Name,
				Injection: injection,
			})
		}
	}
	return deps
}

func extractRustDependencyTypes(typeRef string) []string {
	identifiers := splitRustTypeIdentifiers(typeRef)
	deps := make([]string, 0, len(identifiers))
	seen := make(map[string]bool, len(identifiers))

	for _, identifier := range identifiers {
		if shouldSkipRustDependency(identifier) || seen[identifier] {
			continue
		}
		seen[identifier] = true
		deps = append(deps, identifier)
	}

	return deps
}

func splitRustTypeIdentifiers(typeRef string) []string {
	replacer := strings.NewReplacer(
		"&mut", " ",
		"&", " ",
		"dyn", " ",
		"::", " ",
		"<", " ",
		">", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		",", " ",
		";", " ",
	)
	cleaned := replacer.Replace(typeRef)
	parts := strings.Fields(cleaned)
	identifiers := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimPrefix(part, "'")
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		identifiers = append(identifiers, NormalizeTypeReference(part))
	}

	return identifiers
}

func shouldSkipRustDependency(identifier string) bool {
	if identifier == "" || len(identifier) == 1 {
		return true
	}
	if first := identifier[0]; first < 'A' || first > 'Z' {
		return true
	}

	switch identifier {
	case "Arc", "BTreeMap", "BTreeSet", "Box", "Cow", "HashMap", "HashSet", "Mutex",
		"Option", "Pin", "Rc", "Result", "RwLock", "Self", "String", "Vec":
		return true
	default:
		return false
	}
}

func isRustConstructor(method ExtractedMethod, owner string) bool {
	if method.Name != "new" {
		return false
	}
	return method.Returns == "Self" || method.Returns == owner
}

func findMatchingParen(value string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitRustCommaList(value string) []string {
	var parts []string
	var current strings.Builder
	angles := 0
	parens := 0
	brackets := 0

	for _, ch := range value {
		switch ch {
		case '<':
			angles++
			current.WriteRune(ch)
		case '>':
			if angles > 0 {
				angles--
			}
			current.WriteRune(ch)
		case '(':
			parens++
			current.WriteRune(ch)
		case ')':
			if parens > 0 {
				parens--
			}
			current.WriteRune(ch)
		case '[':
			brackets++
			current.WriteRune(ch)
		case ']':
			if brackets > 0 {
				brackets--
			}
			current.WriteRune(ch)
		case ',':
			if angles == 0 && parens == 0 && brackets == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func stripRustLineComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

var _ Parser = (*RustParser)(nil)
var _ MultiNodeParser = (*RustParser)(nil)
