package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	pythonClassPattern      = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s*\((.*)\))?\s*:`)
	pythonFunctionPattern   = regexp.MustCompile(`^(async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pythonIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*`)
)

// PythonParser parses Python source files using indentation and signature
// heuristics without importing or executing project code.
type PythonParser struct{}

// NewPythonParser creates a Python parser.
func NewPythonParser() *PythonParser {
	return &PythonParser{}
}

// Language returns "python".
func (p *PythonParser) Language() string {
	return "python"
}

// ParseFile returns the first public node for backward compatibility.
func (p *PythonParser) ParseFile(filePath string) (*ExtractedNode, error) {
	nodes, err := p.ParseFileNodes(filePath)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return &ExtractedNode{
			FilePath: filePath,
			Language: "python",
			Module:   pythonModuleName(filePath),
		}, nil
	}
	return nodes[0], nil
}

// ParseFileNodes extracts public top-level classes and functions from a Python
// module. Nested declarations and private names are intentionally ignored.
func (p *PythonParser) ParseFileNodes(filePath string) ([]*ExtractedNode, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	module := pythonModuleName(filePath)
	nodes := make([]*ExtractedNode, 0)
	pendingDoc := ""

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if pythonIndent(raw) != 0 {
			continue
		}
		if doc := pythonCommentText(trimmed); doc != "" {
			pendingDoc = appendPythonDescription(pendingDoc, doc)
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			continue
		}

		if matches := pythonClassPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
			name := matches[1]
			if isPrivatePythonName(name) {
				pendingDoc = ""
				continue
			}
			bases := ""
			if len(matches) > 2 {
				bases = strings.TrimSpace(matches[2])
			}
			blockEnd := pythonBlockEnd(lines, i, 0)
			nodeType := "class"
			if pythonBasesContainInterface(bases) {
				nodeType = "interface"
			}
			node := &ExtractedNode{
				ID:          name,
				Type:        nodeType,
				Namespace:   module,
				Language:    "python",
				Module:      module,
				FilePath:    filePath,
				Description: pendingDoc,
			}
			for _, base := range pythonDependencyTypes(bases) {
				node.Dependencies = mergeDependencies(node.Dependencies, []ExtractedDependency{{
					Target: base, Type: "class", Injection: "inherits",
				}})
			}
			parsePythonClassBody(lines, i+1, blockEnd, node)
			nodes = append(nodes, node)
			pendingDoc = ""
			i = blockEnd - 1
			continue
		}

		if pythonFunctionPattern.MatchString(trimmed) {
			signature, end := collectPythonSignature(lines, i)
			method, ok := parsePythonFunctionSignature(signature, nil)
			if ok && !isPrivatePythonName(method.Name) {
				method.Description = pendingDoc
				node := &ExtractedNode{
					ID:          method.Name,
					Type:        "function",
					Namespace:   module,
					Language:    "python",
					Module:      module,
					FilePath:    filePath,
					Description: pendingDoc,
					Methods:     []ExtractedMethod{method},
				}
				node.Dependencies = pythonDependenciesFromParameters(method.Parameters, "method")
				nodes = append(nodes, node)
			}
			pendingDoc = ""
			i = end
			continue
		}

		pendingDoc = ""
	}

	return nodes, nil
}

func parsePythonClassBody(lines []string, start, end int, node *ExtractedNode) {
	bodyIndent := pythonBodyIndent(lines, start, end)
	if bodyIndent < 0 {
		return
	}

	decorators := make([]string, 0)
	pendingDoc := ""
	for i := start; i < end; i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || pythonIndent(raw) != bodyIndent {
			continue
		}
		if doc := pythonCommentText(trimmed); doc != "" {
			pendingDoc = appendPythonDescription(pendingDoc, doc)
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			decorators = append(decorators, strings.TrimSpace(strings.TrimPrefix(trimmed, "@")))
			continue
		}

		if pythonFunctionPattern.MatchString(trimmed) {
			signature, signatureEnd := collectPythonSignature(lines, i)
			method, ok := parsePythonFunctionSignature(signature, decorators)
			isProperty := hasPythonDecorator(decorators, "property")
			isPropertySetter := hasPythonPropertySetter(decorators)
			decorators = decorators[:0]
			if !ok {
				pendingDoc = ""
				i = signatureEnd
				continue
			}
			method.Description = pendingDoc
			pendingDoc = ""

			switch {
			case method.Name == "__init__":
				node.Constructors = append(node.Constructors, ExtractedConstructor{
					Signature:   buildPythonConstructorSignature(node.ID, method.Parameters),
					Parameters:  method.Parameters,
					Description: method.Description,
				})
				node.Dependencies = mergeDependencies(node.Dependencies, pythonDependenciesFromParameters(method.Parameters, "constructor"))
			case isProperty || pythonMethodIsProperty(lines, i):
				node.Properties = mergePythonProperty(node.Properties, ExtractedProperty{
					Name: method.Name, Type: pythonReturnOrAny(method.Returns), Access: "get", Description: method.Description, IsPublic: true,
				})
			case isPropertySetter:
				node.Properties = mergePythonProperty(node.Properties, ExtractedProperty{
					Name: method.Name, Type: "Any", Access: "set", Description: method.Description, IsPublic: true,
				})
			case isPrivatePythonName(method.Name):
				// Private methods are outside the public graph contract.
			default:
				node.Methods = append(node.Methods, method)
				node.Dependencies = mergeDependencies(node.Dependencies, pythonDependenciesFromParameters(method.Parameters, "method"))
			}
			i = signatureEnd
			continue
		}

		decorators = decorators[:0]
		if name, typeRef, ok := parsePythonAnnotatedField(trimmed); ok && !isPrivatePythonName(name) {
			node.Properties = mergePythonProperty(node.Properties, ExtractedProperty{
				Name: name, Type: typeRef, Access: "get; set", Description: pendingDoc, IsPublic: true,
			})
			for _, dependency := range pythonDependencyTypes(typeRef) {
				node.Dependencies = mergeDependencies(node.Dependencies, []ExtractedDependency{{
					Target: dependency, Type: "interface", FieldName: name, Injection: "field",
				}})
			}
		}
		pendingDoc = ""
	}
}

func collectPythonSignature(lines []string, start int) (string, int) {
	parts := make([]string, 0, 2)
	parens, brackets, braces := 0, 0, 0
	for i := start; i < len(lines); i++ {
		part := strings.TrimSpace(stripPythonComment(lines[i]))
		if part == "" {
			continue
		}
		parts = append(parts, part)
		for _, char := range part {
			switch char {
			case '(':
				parens++
			case ')':
				parens--
			case '[':
				brackets++
			case ']':
				brackets--
			case '{':
				braces++
			case '}':
				braces--
			}
		}
		if parens == 0 && brackets == 0 && braces == 0 && strings.HasSuffix(part, ":") {
			return collapseWhitespace(strings.Join(parts, " ")), i
		}
	}
	return collapseWhitespace(strings.Join(parts, " ")), len(lines) - 1
}

func parsePythonFunctionSignature(signature string, decorators []string) (ExtractedMethod, bool) {
	signature = collapseWhitespace(strings.TrimSpace(signature))
	matches := pythonFunctionPattern.FindStringSubmatch(signature)
	if len(matches) < 3 {
		return ExtractedMethod{}, false
	}

	name := matches[2]
	openParen := strings.Index(signature, "(")
	closeParen := findMatchingParen(signature, openParen)
	if openParen < 0 || closeParen < 0 {
		return ExtractedMethod{}, false
	}
	parameters := parsePythonParameters(signature[openParen+1 : closeParen])
	after := strings.TrimSpace(strings.TrimSuffix(signature[closeParen+1:], ":"))
	returns := ""
	if strings.HasPrefix(after, "->") {
		returns = strings.TrimSpace(strings.TrimPrefix(after, "->"))
	}

	return ExtractedMethod{
		Name:       name,
		Signature:  buildPythonMethodSignature(name, parameters, returns),
		Parameters: parameters,
		Returns:    returns,
		IsPublic:   true,
		Async:      strings.TrimSpace(matches[1]) != "",
		Static:     hasPythonDecorator(decorators, "staticmethod"),
		Access:     "public",
	}, true
}

func parsePythonParameters(raw string) []ExtractedParameter {
	parts := splitPythonCommaList(raw)
	parameters := make([]ExtractedParameter, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "/" || part == "*" {
			continue
		}
		part = trimPythonDefault(part)
		colon := strings.Index(part, ":")
		name, typeRef := part, "Any"
		if colon >= 0 {
			name = strings.TrimSpace(part[:colon])
			typeRef = strings.TrimSpace(part[colon+1:])
		}
		name = strings.TrimLeft(strings.TrimSpace(name), "*")
		if name == "" || name == "self" || name == "cls" {
			continue
		}
		parameters = append(parameters, ExtractedParameter{Name: name, Type: typeRef})
	}
	return parameters
}

func splitPythonCommaList(value string) []string {
	parts := make([]string, 0)
	var current strings.Builder
	parens, brackets, braces := 0, 0, 0
	quote := rune(0)
	escaped := false
	for _, char := range value {
		if quote != 0 {
			current.WriteRune(char)
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			current.WriteRune(char)
			continue
		}
		switch char {
		case '(':
			parens++
		case ')':
			parens--
		case '[':
			brackets++
		case ']':
			brackets--
		case '{':
			braces++
		case '}':
			braces--
		case ',':
			if parens == 0 && brackets == 0 && braces == 0 {
				parts = append(parts, current.String())
				current.Reset()
				continue
			}
		}
		current.WriteRune(char)
	}
	if strings.TrimSpace(current.String()) != "" {
		parts = append(parts, current.String())
	}
	return parts
}

func trimPythonDefault(value string) string {
	parts := splitPythonAssignment(value)
	return strings.TrimSpace(parts)
}

func splitPythonAssignment(value string) string {
	brackets, parens, braces := 0, 0, 0
	for i, char := range value {
		switch char {
		case '[':
			brackets++
		case ']':
			brackets--
		case '(':
			parens++
		case ')':
			parens--
		case '{':
			braces++
		case '}':
			braces--
		case '=':
			if brackets == 0 && parens == 0 && braces == 0 {
				return value[:i]
			}
		}
	}
	return value
}

func buildPythonMethodSignature(name string, parameters []ExtractedParameter, returns string) string {
	parts := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		parts = append(parts, parameter.Name+": "+pythonReturnOrAny(parameter.Type))
	}
	signature := name + "(" + strings.Join(parts, ", ") + ")"
	if returns != "" {
		signature += " -> " + returns
	}
	return signature
}

func buildPythonConstructorSignature(owner string, parameters []ExtractedParameter) string {
	return strings.Replace(buildPythonMethodSignature(owner, parameters, ""), owner+"(", owner+"(", 1)
}

func pythonDependenciesFromParameters(parameters []ExtractedParameter, injection string) []ExtractedDependency {
	dependencies := make([]ExtractedDependency, 0)
	for _, parameter := range parameters {
		for _, target := range pythonDependencyTypes(parameter.Type) {
			dependencies = append(dependencies, ExtractedDependency{
				Target: target, Type: "interface", FieldName: parameter.Name, Injection: injection,
			})
		}
	}
	return dependencies
}

func pythonDependencyTypes(typeRef string) []string {
	identifiers := pythonIdentifierPattern.FindAllString(strings.Trim(typeRef, " \t\"'"), -1)
	dependencies := make([]string, 0, len(identifiers))
	seen := make(map[string]bool)
	for _, identifier := range identifiers {
		if dot := strings.LastIndex(identifier, "."); dot >= 0 {
			identifier = identifier[dot+1:]
		}
		if shouldSkipPythonDependency(identifier) || seen[identifier] {
			continue
		}
		seen[identifier] = true
		dependencies = append(dependencies, identifier)
	}
	return dependencies
}

func shouldSkipPythonDependency(identifier string) bool {
	if identifier == "" || !unicode.IsUpper(rune(identifier[0])) {
		return true
	}
	switch identifier {
	case "ABC", "Any", "AsyncIterable", "AsyncIterator", "Awaitable", "Callable", "ClassVar",
		"Collection", "Coroutine", "DefaultDict", "Deque", "Dict", "FrozenSet", "Generic",
		"Iterable", "Iterator", "List", "Literal", "Mapping", "MutableMapping", "MutableSequence",
		"None", "Optional", "Protocol", "Sequence", "Set", "Tuple", "Type", "TypeVar", "Union":
		return true
	default:
		return false
	}
}

func parsePythonAnnotatedField(line string) (string, string, bool) {
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return "", "", false
	}
	name := strings.TrimSpace(line[:colon])
	if !pythonIdentifierPattern.MatchString(name) || strings.ContainsAny(name, " .[]()") {
		return "", "", false
	}
	typeRef := strings.TrimSpace(splitPythonAssignment(line[colon+1:]))
	if typeRef == "" {
		return "", "", false
	}
	return name, typeRef, true
}

func pythonBasesContainInterface(bases string) bool {
	for _, base := range pythonIdentifierPattern.FindAllString(bases, -1) {
		if dot := strings.LastIndex(base, "."); dot >= 0 {
			base = base[dot+1:]
		}
		if base == "Protocol" || base == "ABC" {
			return true
		}
	}
	return false
}

func pythonMethodIsProperty(lines []string, index int) bool {
	for i := index - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed == "@property"
	}
	return false
}

func hasPythonPropertySetter(decorators []string) bool {
	for _, decorator := range decorators {
		if strings.HasSuffix(strings.TrimSpace(decorator), ".setter") {
			return true
		}
	}
	return false
}

func hasPythonDecorator(decorators []string, name string) bool {
	for _, decorator := range decorators {
		decorator = strings.TrimSpace(decorator)
		if decorator == name || strings.HasSuffix(decorator, "."+name) {
			return true
		}
	}
	return false
}

func mergePythonProperty(properties []ExtractedProperty, incoming ExtractedProperty) []ExtractedProperty {
	for i := range properties {
		if properties[i].Name == incoming.Name {
			if incoming.Access != properties[i].Access {
				properties[i].Access = "get; set"
			}
			if properties[i].Type == "" || properties[i].Type == "Any" {
				properties[i].Type = incoming.Type
			}
			return properties
		}
	}
	return append(properties, incoming)
}

func pythonBodyIndent(lines []string, start, end int) int {
	for i := start; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return pythonIndent(lines[i])
	}
	return -1
}

func pythonBlockEnd(lines []string, start, parentIndent int) int {
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if pythonIndent(lines[i]) <= parentIndent {
			return i
		}
	}
	return len(lines)
}

func pythonIndent(line string) int {
	indent := 0
	for _, char := range line {
		switch char {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

func pythonModuleName(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func isPrivatePythonName(name string) bool {
	return strings.HasPrefix(name, "_")
}

func pythonReturnOrAny(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Any"
	}
	return value
}

func pythonCommentText(line string) string {
	if strings.HasPrefix(line, "#") {
		return strings.TrimSpace(strings.TrimPrefix(line, "#"))
	}
	return ""
}

func appendPythonDescription(existing, line string) string {
	if existing == "" {
		return line
	}
	return existing + " " + line
}

func stripPythonComment(line string) string {
	if index := strings.Index(line, "#"); index >= 0 {
		return line[:index]
	}
	return line
}

var _ Parser = (*PythonParser)(nil)
var _ MultiNodeParser = (*PythonParser)(nil)
