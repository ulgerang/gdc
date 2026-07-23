package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	gdscriptClassNamePattern  = regexp.MustCompile(`^class_name\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	gdscriptExtendsPattern    = regexp.MustCompile(`^extends\s+([A-Za-z_][A-Za-z0-9_.]*)\b`)
	gdscriptFunctionPattern   = regexp.MustCompile(`^(static\s+)?func\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	gdscriptSignalPattern     = regexp.MustCompile(`^signal\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(|$)`)
	gdscriptIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// GDScriptParser extracts public Godot 4 script contracts without launching
// Godot or executing project code.
type GDScriptParser struct{}

// NewGDScriptParser creates a GDScript parser.
func NewGDScriptParser() *GDScriptParser {
	return &GDScriptParser{}
}

// Language returns "gdscript".
func (p *GDScriptParser) Language() string {
	return "gdscript"
}

// ParseFile extracts the primary script node from one .gd file.
func (p *GDScriptParser) ParseFile(filePath string) (*ExtractedNode, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	module := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	node := &ExtractedNode{
		ID:        module,
		Type:      "class",
		Namespace: module,
		Language:  "gdscript",
		Module:    module,
		FilePath:  filePath,
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	pendingDoc := ""
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if gdscriptIndent(raw) != 0 {
			continue
		}
		if doc := gdscriptDocText(trimmed); doc != "" {
			pendingDoc = appendGDScriptDescription(pendingDoc, doc)
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			continue
		}

		declaration := strings.TrimSpace(stripGDScriptComment(trimmed))
		if matches := gdscriptClassNamePattern.FindStringSubmatch(declaration); len(matches) > 1 {
			node.ID = matches[1]
			node.Description = pendingDoc
			pendingDoc = ""
			continue
		}
		if matches := gdscriptExtendsPattern.FindStringSubmatch(declaration); len(matches) > 1 {
			node.Dependencies = appendGDScriptTypeDependencies(node.Dependencies, matches[1], "", "inherits")
			pendingDoc = ""
			continue
		}
		if gdscriptFunctionPattern.MatchString(declaration) {
			signature, end := collectGDScriptDeclaration(lines, i)
			method, ok := parseGDScriptFunctionSignature(signature)
			if ok {
				method.Description = pendingDoc
				if method.Name == "_init" {
					node.Constructors = append(node.Constructors, ExtractedConstructor{
						Signature:   buildGDScriptConstructorSignature(node.ID, method.Parameters),
						Parameters:  method.Parameters,
						Description: method.Description,
					})
					node.Dependencies = appendGDScriptParameterDependencies(node.Dependencies, method.Parameters, "constructor")
				} else if !isPrivateGDScriptName(method.Name) {
					node.Methods = append(node.Methods, method)
					node.Dependencies = appendGDScriptParameterDependencies(node.Dependencies, method.Parameters, "method")
					node.Dependencies = appendGDScriptTypeDependencies(node.Dependencies, method.Returns, "", "return")
				}
			}
			pendingDoc = ""
			i = end
			continue
		}
		if gdscriptSignalPattern.MatchString(declaration) {
			signature, end := collectGDScriptDeclaration(lines, i)
			event, parameters, ok := parseGDScriptSignalSignature(signature)
			if ok {
				event.Description = pendingDoc
				node.Events = append(node.Events, event)
				node.Dependencies = appendGDScriptParameterDependencies(node.Dependencies, parameters, "event")
			}
			pendingDoc = ""
			i = end
			continue
		}
		if property, ok := parseGDScriptProperty(declaration); ok {
			property.Description = pendingDoc
			node.Properties = append(node.Properties, property)
			node.Dependencies = appendGDScriptTypeDependencies(node.Dependencies, property.Type, property.Name, "field")
			pendingDoc = ""
			continue
		}

		if !strings.HasPrefix(trimmed, "#") {
			pendingDoc = ""
		}
	}

	return node, nil
}

func collectGDScriptDeclaration(lines []string, start int) (string, int) {
	parts := make([]string, 0, 2)
	parens, brackets, braces := 0, 0, 0
	for i := start; i < len(lines); i++ {
		part := strings.TrimSpace(stripGDScriptComment(lines[i]))
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
		if parens == 0 && brackets == 0 && braces == 0 {
			return collapseWhitespace(strings.Join(parts, " ")), i
		}
	}
	return collapseWhitespace(strings.Join(parts, " ")), len(lines) - 1
}

func parseGDScriptFunctionSignature(signature string) (ExtractedMethod, bool) {
	signature = collapseWhitespace(strings.TrimSpace(signature))
	matches := gdscriptFunctionPattern.FindStringSubmatch(signature)
	if len(matches) < 3 {
		return ExtractedMethod{}, false
	}
	open := strings.Index(signature, "(")
	close := findMatchingParen(signature, open)
	if open < 0 || close < 0 {
		return ExtractedMethod{}, false
	}
	parameters := parseGDScriptParameters(signature[open+1 : close])
	after := strings.TrimSpace(strings.TrimSuffix(signature[close+1:], ":"))
	returns := ""
	if strings.HasPrefix(after, "->") {
		returns = strings.TrimSpace(strings.TrimPrefix(after, "->"))
	}
	name := matches[2]
	return ExtractedMethod{
		Name:       name,
		Signature:  buildGDScriptMethodSignature(name, parameters, returns),
		Parameters: parameters,
		Returns:    returns,
		IsPublic:   name == "_init" || !isPrivateGDScriptName(name),
		Static:     strings.TrimSpace(matches[1]) != "",
		Access:     "public",
	}, true
}

func parseGDScriptSignalSignature(signature string) (ExtractedEvent, []ExtractedParameter, bool) {
	signature = collapseWhitespace(strings.TrimSpace(signature))
	matches := gdscriptSignalPattern.FindStringSubmatch(signature)
	if len(matches) < 2 {
		return ExtractedEvent{}, nil, false
	}
	name := matches[1]
	parameters := make([]ExtractedParameter, 0)
	if open := strings.Index(signature, "("); open >= 0 {
		if close := findMatchingParen(signature, open); close >= 0 {
			parameters = parseGDScriptParameters(signature[open+1 : close])
		}
	}
	return ExtractedEvent{
		Name:      name,
		Signature: buildGDScriptMethodSignature(name, parameters, ""),
		IsPublic:  true,
	}, parameters, true
}

func parseGDScriptProperty(line string) (ExtractedProperty, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "var ") {
		return ExtractedProperty{}, false
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(line, "var "))
	nameEnd := len(remainder)
	for i, char := range remainder {
		if unicode.IsSpace(char) || char == ':' || char == '=' {
			nameEnd = i
			break
		}
	}
	name := strings.TrimSpace(remainder[:nameEnd])
	if name == "" || isPrivateGDScriptName(name) || !gdscriptIdentifierPattern.MatchString(name) {
		return ExtractedProperty{}, false
	}
	typeRef := "Variant"
	afterName := strings.TrimSpace(remainder[nameEnd:])
	if strings.HasPrefix(afterName, ":") {
		typePart := strings.TrimSpace(strings.TrimPrefix(afterName, ":"))
		typePart = gdscriptBeforeTopLevelAssignment(typePart)
		if propertyBlock := strings.Index(typePart, ":"); propertyBlock >= 0 {
			typePart = typePart[:propertyBlock]
		}
		if strings.TrimSpace(typePart) != "" {
			typeRef = strings.TrimSpace(typePart)
		}
	}
	return ExtractedProperty{Name: name, Type: typeRef, Access: "get; set", IsPublic: true}, true
}

func parseGDScriptParameters(raw string) []ExtractedParameter {
	parts := splitGDScriptCommaList(raw)
	parameters := make([]ExtractedParameter, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(gdscriptBeforeTopLevelAssignment(part))
		if part == "" {
			continue
		}
		name, typeRef := part, "Variant"
		if colon := strings.Index(part, ":"); colon >= 0 {
			name = strings.TrimSpace(part[:colon])
			typeRef = strings.TrimSpace(part[colon+1:])
		}
		if name == "" {
			continue
		}
		parameters = append(parameters, ExtractedParameter{Name: name, Type: gdscriptTypeOrVariant(typeRef)})
	}
	return parameters
}

func splitGDScriptCommaList(value string) []string {
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

func gdscriptBeforeTopLevelAssignment(value string) string {
	parens, brackets, braces := 0, 0, 0
	for i, char := range value {
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
		case '=':
			if parens == 0 && brackets == 0 && braces == 0 {
				return strings.TrimSpace(value[:i])
			}
		}
	}
	return strings.TrimSpace(value)
}

func buildGDScriptMethodSignature(name string, parameters []ExtractedParameter, returns string) string {
	parts := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		parts = append(parts, parameter.Name+": "+gdscriptTypeOrVariant(parameter.Type))
	}
	signature := name + "(" + strings.Join(parts, ", ") + ")"
	if strings.TrimSpace(returns) != "" {
		signature += " -> " + strings.TrimSpace(returns)
	}
	return signature
}

func buildGDScriptConstructorSignature(owner string, parameters []ExtractedParameter) string {
	return buildGDScriptMethodSignature(owner, parameters, "")
}

func appendGDScriptParameterDependencies(existing []ExtractedDependency, parameters []ExtractedParameter, injection string) []ExtractedDependency {
	for _, parameter := range parameters {
		existing = appendGDScriptTypeDependencies(existing, parameter.Type, parameter.Name, injection)
	}
	return existing
}

func appendGDScriptTypeDependencies(existing []ExtractedDependency, typeRef, fieldName, injection string) []ExtractedDependency {
	for _, identifier := range gdscriptIdentifierPattern.FindAllString(typeRef, -1) {
		if shouldSkipGDScriptDependency(identifier) {
			continue
		}
		existing = mergeDependencies(existing, []ExtractedDependency{{
			Target: identifier, Type: "class", FieldName: fieldName, Injection: injection,
		}})
	}
	return existing
}

func shouldSkipGDScriptDependency(identifier string) bool {
	if identifier == "" || !unicode.IsUpper(rune(identifier[0])) {
		return true
	}
	switch identifier {
	case "AABB", "Array", "ArrayMesh", "Basis", "Bool", "Callable", "Color", "Dictionary",
		"Engine", "FileAccess", "Float", "Image", "Input", "Int", "JSON", "Mutex", "Node",
		"Node2D", "Node3D", "NodePath", "Object", "PackedByteArray", "PackedColorArray",
		"PackedFloat32Array", "PackedFloat64Array", "PackedInt32Array", "PackedInt64Array",
		"PackedStringArray", "PackedVector2Array", "PackedVector3Array", "Plane", "Projection",
		"Quaternion", "RandomNumberGenerator", "Rect2", "Rect2i", "RefCounted", "Resource",
		"RID", "SceneTree", "Signal", "String", "StringName", "Thread", "Transform2D",
		"Transform3D", "Tween", "Variant", "Vector2", "Vector2i", "Vector3", "Vector3i",
		"Vector4", "Vector4i", "Void":
		return true
	default:
		return false
	}
}

func isPrivateGDScriptName(name string) bool {
	return strings.HasPrefix(name, "_")
}

func gdscriptTypeOrVariant(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Variant"
	}
	return strings.TrimSpace(value)
}

func gdscriptIndent(line string) int {
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

func gdscriptDocText(line string) string {
	if strings.HasPrefix(line, "##") {
		return strings.TrimSpace(strings.TrimPrefix(line, "##"))
	}
	return ""
}

func appendGDScriptDescription(existing, line string) string {
	if existing == "" {
		return line
	}
	return existing + " " + line
}

func stripGDScriptComment(line string) string {
	quote := rune(0)
	escaped := false
	for i, char := range line {
		if quote != 0 {
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
			continue
		}
		if char == '#' {
			return line[:i]
		}
	}
	return line
}

var _ Parser = (*GDScriptParser)(nil)
