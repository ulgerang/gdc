//go:build !treesitter
// +build !treesitter

package parser

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// CSharpParser parses C# source files using regex-based extraction
// This is the default implementation. To use tree-sitter, build with -tags treesitter
type CSharpParser struct {
	regexParser *RegexCSharpParser
}

// NewCSharpParser creates a new C# parser
func NewCSharpParser() *CSharpParser {
	return &CSharpParser{
		regexParser: NewRegexCSharpParser(),
	}
}

// Language returns "csharp"
func (p *CSharpParser) Language() string {
	return "csharp"
}

// ParseFile parses a C# source file
func (p *CSharpParser) ParseFile(filePath string) (*ExtractedNode, error) {
	// Delegate to the regex parser
	return p.regexParser.ParseFile(filePath)
}

// ParseFileNode selects one named C# declaration from a file that may contain
// several public types.
func (p *CSharpParser) ParseFileNode(filePath, nodeID string) (*ExtractedNode, error) {
	return p.regexParser.ParseFileNode(filePath, nodeID)
}

// Ensure CSharpParser implements Parser interface
var _ Parser = (*CSharpParser)(nil)

// RegexCSharpParser is the regex-based parser used as default
type RegexCSharpParser struct{}

// NewRegexCSharpParser creates a new regex-based C# parser
func NewRegexCSharpParser() *RegexCSharpParser {
	return &RegexCSharpParser{}
}

// Language returns "csharp"
func (p *RegexCSharpParser) Language() string {
	return "csharp"
}

// ParseFileNode selects and parses one named C# type. The legacy ParseFile
// method intentionally keeps its first-type behavior for compatibility, while
// implementation verification uses this targeted surface.
func (p *RegexCSharpParser) ParseFileNode(filePath, nodeID string) (*ExtractedNode, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	typeKind, declarationStart, bodyEnd, ok := findNamedCSharpType(content, nodeID)
	if !ok {
		return nil, nil
	}

	segment := content[declarationStart:bodyEnd]
	extracted := &ExtractedNode{
		ID:         nodeID,
		Type:       normalizeCSharpTypeKind(typeKind),
		Namespace:  p.extractNamespace(content),
		Language:   "csharp",
		Attributes: p.extractAttributes(segment),
		FilePath:   filePath,
	}

	p.extractNamedConstructors(segment, extracted)
	p.extractNamedMethods(segment, extracted)
	p.extractNamedPropertiesAndEvents(segment, extracted)
	return extracted, nil
}

// Regex patterns for C# parsing
var (
	csClassPattern       = regexp.MustCompile(`(?:public|internal|private|protected)?\s*(?:abstract|sealed|static|partial)?\s*class\s+(\w+)(?:\s*:\s*([^{]+))?`)
	csInterfacePattern   = regexp.MustCompile(`(?:public|internal)?\s*interface\s+(\w+)(?:\s*:\s*([^{]+))?`)
	csMethodPattern      = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(virtual|override|abstract|static|async)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*\(([^)]*)\)`)
	csPropertyPattern    = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(virtual|override|abstract|static)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*\{\s*([^}]*)\}`)
	csEventPattern       = regexp.MustCompile(`(?:public|protected|private|internal)?\s*event\s+(\w+(?:<[^>]+>)?)\s+(\w+)\s*;`)
	csConstructorPattern = regexp.MustCompile(`(?:public|protected|private|internal)\s+(\w+)\s*\(([^)]*)\)`)
	csXMLDocPattern      = regexp.MustCompile(`///\s*<summary>\s*(.+?)\s*</summary>`)
	csFieldPattern       = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(?:readonly|const)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*(?:=|;)`)
)

func findNamedCSharpType(content, nodeID string) (kind string, declarationStart, bodyEnd int, ok bool) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return "", 0, 0, false
	}

	pattern := regexp.MustCompile(`(?m)(?:^|\n)[\t ]*(?:(?:public|internal|private|protected|abstract|sealed|static|partial|readonly|ref)[\t ]+)*(class|interface|struct|record(?:[\t ]+(?:class|struct))?|enum)[\t ]+` + regexp.QuoteMeta(nodeID) + `(?:[\t ]*<[^>{}\r\n]+>)?(?:[\t ]*:[^{]+)?\s*\{`)
	match := pattern.FindStringSubmatchIndex(content)
	if match == nil {
		return "", 0, 0, false
	}

	openOffset := strings.LastIndex(content[match[0]:match[1]], "{")
	if openOffset < 0 {
		return "", 0, 0, false
	}
	openBrace := match[0] + openOffset
	closeBrace := findMatchingCSharpBrace(content, openBrace)
	if closeBrace < 0 {
		return "", 0, 0, false
	}

	kind = content[match[2]:match[3]]
	return kind, match[0], closeBrace + 1, true
}

func normalizeCSharpTypeKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "record struct":
		return "struct"
	case "record", "record class":
		return "class"
	default:
		return kind
	}
}

func findMatchingCSharpBrace(content string, openBrace int) int {
	depth := 0
	inLineComment := false
	inBlockComment := false
	inString := false
	inVerbatimString := false
	inChar := false
	escaped := false

	for i := openBrace; i < len(content); i++ {
		ch := content[i]
		next := byte(0)
		if i+1 < len(content) {
			next = content[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inVerbatimString {
			if ch == '"' {
				if next == '"' {
					i++
				} else {
					inVerbatimString = false
				}
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inChar = false
			}
			continue
		}

		switch {
		case ch == '/' && next == '/':
			inLineComment = true
			i++
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
		case ch == '@' && next == '"':
			inVerbatimString = true
			i++
		case ch == '"':
			inString = true
		case ch == '\'':
			inChar = true
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func (p *RegexCSharpParser) extractNamedConstructors(content string, extracted *ExtractedNode) {
	pattern := regexp.MustCompile(`(?ms)^[\t ]*(?:\[[^\]\r\n]+\][\t ]*\r?\n[\t ]*)*(public|protected|private|internal)[\t ]+` + regexp.QuoteMeta(extracted.ID) + `(?:[\t ]*<[^>{}\r\n]+>)?[\t ]*\(([^)]*)\)`)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		params := normalizeCSharpDeclarationWhitespace(match[2])
		extracted.Constructors = append(extracted.Constructors, ExtractedConstructor{
			Signature:  strings.TrimSpace(match[1]) + " " + extracted.ID + "(" + params + ")",
			Parameters: p.parseParameters(params),
		})
		p.extractDependenciesFromParams(params, extracted)
	}
}

func (p *RegexCSharpParser) extractNamedMethods(content string, extracted *ExtractedNode) {
	pattern := regexp.MustCompile(`(?ms)^[\t ]*(?:\[[^\]\r\n]+\][\t ]*\r?\n[\t ]*)*(public|protected|private|internal)[\t ]+((?:(?:static|virtual|override|abstract|async|sealed|new|extern|partial)[\t ]+)*)([A-Za-z_][A-Za-z0-9_.]*(?:[\t ]*<[^;{}()]+>)?(?:[\t ]*\[\])?\??)[\t ]+([A-Za-z_][A-Za-z0-9_]*)[\t ]*\(([^)]*)\)`)
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		access := strings.TrimSpace(match[1])
		modifiers := strings.Fields(match[2])
		returnType := normalizeCSharpDeclarationWhitespace(match[3])
		methodName := strings.TrimSpace(match[4])
		params := normalizeCSharpDeclarationWhitespace(match[5])

		sigParts := []string{access}
		sigParts = append(sigParts, modifiers...)
		sigParts = append(sigParts, returnType, methodName+"("+params+")")
		extracted.Methods = append(extracted.Methods, ExtractedMethod{
			Name:       methodName,
			Signature:  strings.Join(sigParts, " "),
			Parameters: p.parseParameters(params),
			Returns:    returnType,
			IsPublic:   access == "public",
			Static:     containsString(modifiers, "static"),
			Async:      containsString(modifiers, "async"),
			Access:     access,
		})
	}
}

func (p *RegexCSharpParser) extractNamedPropertiesAndEvents(content string, extracted *ExtractedNode) {
	for _, match := range csPropertyPattern.FindAllStringSubmatch(content, -1) {
		extracted.Properties = append(extracted.Properties, ExtractedProperty{
			Name:     match[3],
			Type:     match[2],
			Access:   strings.TrimSpace(match[4]),
			IsPublic: strings.Contains(match[0], "public"),
		})
	}
	for _, match := range csEventPattern.FindAllStringSubmatch(content, -1) {
		extracted.Events = append(extracted.Events, ExtractedEvent{
			Name:      match[2],
			Signature: "event " + match[1] + " " + match[2],
			IsPublic:  strings.Contains(match[0], "public"),
		})
	}
}

func normalizeCSharpDeclarationWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ParseFile parses a C# source file using regex-based extraction
func (p *RegexCSharpParser) ParseFile(filePath string) (*ExtractedNode, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	extracted := &ExtractedNode{
		FilePath: filePath,
		Language: "csharp",
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	content := strings.Join(lines, "\n")

	// Extract namespace
	if ns := p.extractNamespace(content); ns != "" {
		extracted.Namespace = ns
	}

	// Try to find class or interface
	if matches := csClassPattern.FindStringSubmatch(content); len(matches) > 1 {
		extracted.ID = matches[1]
		extracted.Type = "class"
		if len(matches) > 2 && matches[2] != "" {
			p.extractBaseTypes(matches[2], extracted)
		}
	} else if matches := csInterfacePattern.FindStringSubmatch(content); len(matches) > 1 {
		extracted.ID = matches[1]
		extracted.Type = "interface"
	}

	// Extract class-level attributes
	extracted.Attributes = p.extractAttributes(content)

	// Parse line by line for better context
	var pendingDoc string
	braceDepth := 0
	inClass := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track brace depth for better nesting detection
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		braceDepth += openBraces - closeBraces

		// Check if we're inside the main class/interface
		if !inClass {
			if strings.Contains(line, "class "+extracted.ID) ||
				strings.Contains(line, "interface "+extracted.ID) ||
				strings.Contains(line, "struct "+extracted.ID) {
				inClass = true
				continue
			}
		}

		if !inClass {
			continue
		}

		// Collect XML doc comments
		if strings.HasPrefix(trimmed, "///") {
			if matches := csXMLDocPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
				pendingDoc = strings.TrimSpace(matches[1])
			} else {
				// Multi-line doc comment
				if pendingDoc == "" {
					docLine := strings.TrimPrefix(trimmed, "///")
					docLine = strings.TrimSpace(docLine)
					// Remove XML tags for summary extraction
					docLine = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(docLine, " ")
					docLine = strings.TrimSpace(docLine)
					if docLine != "" {
						pendingDoc = docLine
					}
				}
			}
			continue
		}

		// Skip empty lines while preserving doc
		if trimmed == "" {
			continue
		}

		// Only process members at class level (braceDepth should be at or below class level)
		// Skip nested types by checking if we're still in the main type scope

		// Constructor
		if matches := csConstructorPattern.FindStringSubmatch(line); len(matches) > 1 {
			if matches[1] == extracted.ID {
				ctor := ExtractedConstructor{
					Signature:   matches[1] + "(" + matches[2] + ")",
					Description: pendingDoc,
					Parameters:  p.parseParameters(matches[2]),
				}
				extracted.Constructors = append(extracted.Constructors, ctor)
				p.extractDependenciesFromParams(matches[2], extracted)
			}
			pendingDoc = ""
			continue
		}

		// Event
		if matches := csEventPattern.FindStringSubmatch(line); len(matches) > 2 {
			event := ExtractedEvent{
				Name:        matches[2],
				Signature:   "event " + matches[1] + " " + matches[2],
				Description: pendingDoc,
				IsPublic:    strings.Contains(line, "public"),
			}
			extracted.Events = append(extracted.Events, event)
			pendingDoc = ""
			continue
		}

		// Property - check before method since they have similar patterns
		if matches := csPropertyPattern.FindStringSubmatch(line); len(matches) > 3 {
			prop := ExtractedProperty{
				Name:        matches[3],
				Type:        matches[2],
				Access:      strings.TrimSpace(matches[4]),
				Description: pendingDoc,
				IsPublic:    strings.Contains(line, "public"),
			}
			extracted.Properties = append(extracted.Properties, prop)
			pendingDoc = ""
			continue
		}

		// Method
		if matches := csMethodPattern.FindStringSubmatch(line); len(matches) > 3 {
			// Skip if it looks like a constructor (same name as class and no return type in usual sense)
			if matches[3] == extracted.ID {
				continue
			}

			// Extract C#-specific modifiers
			isPublic := strings.Contains(line, "public")
			isStatic := matches[1] == "static" || strings.Contains(line, " static ")
			isAsync := matches[1] == "async" || strings.Contains(line, " async ")
			isVirtual := matches[1] == "virtual" || strings.Contains(line, " virtual ")
			isOverride := matches[1] == "override" || strings.Contains(line, " override ")
			isAbstract := matches[1] == "abstract" || strings.Contains(line, " abstract ")

			accessModifier := "private"
			if isPublic {
				accessModifier = "public"
			} else if strings.Contains(line, "protected") {
				accessModifier = "protected"
			} else if strings.Contains(line, "internal") {
				accessModifier = "internal"
			}

			// Extract attributes from preceding lines (simple approach)
			attributes := p.extractAttributesFromLines(lines, i)

			// Build signature with modifiers
			sigParts := []string{}
			if isPublic {
				sigParts = append(sigParts, "public")
			}
			if isStatic {
				sigParts = append(sigParts, "static")
			}
			if isAsync {
				sigParts = append(sigParts, "async")
			}
			if isVirtual {
				sigParts = append(sigParts, "virtual")
			}
			if isOverride {
				sigParts = append(sigParts, "override")
			}
			if isAbstract {
				sigParts = append(sigParts, "abstract")
			}
			sigParts = append(sigParts, matches[2], matches[3]+"("+matches[4]+")")

			method := ExtractedMethod{
				Name:        matches[3],
				Returns:     matches[2],
				Signature:   strings.Join(sigParts, " "),
				Description: pendingDoc,
				Parameters:  p.parseParameters(matches[4]),
				IsPublic:    isPublic,
				Access:      accessModifier,
				Static:      isStatic,
				Async:       isAsync,
				Attributes:  attributes,
			}
			extracted.Methods = append(extracted.Methods, method)
			pendingDoc = ""
			continue
		}

		// Field (potential dependency)
		if matches := csFieldPattern.FindStringSubmatch(line); len(matches) > 2 {
			if strings.Contains(line, "private") && strings.Contains(line, "readonly") {
				typeName := NormalizeTypeReference(matches[1])
				if strings.HasPrefix(typeName, "I") && len(typeName) > 1 {
					dep := ExtractedDependency{
						Target:    typeName,
						FieldName: matches[2],
						Injection: "field",
					}
					// Avoid duplicates
					exists := false
					for _, existing := range extracted.Dependencies {
						if existing.Target == dep.Target {
							exists = true
							break
						}
					}
					if !exists {
						extracted.Dependencies = append(extracted.Dependencies, dep)
					}
				}
			}
			pendingDoc = ""
			continue
		}

		// Clear pending doc on any other non-empty, non-comment line
		_ = i
		if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "//") {
			pendingDoc = ""
		}
	}

	return extracted, nil
}

func (p *RegexCSharpParser) extractNamespace(content string) string {
	nsPattern := regexp.MustCompile(`namespace\s+([\w.]+)`)
	if matches := nsPattern.FindStringSubmatch(content); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (p *RegexCSharpParser) extractBaseTypes(baseTypes string, node *ExtractedNode) {
	parts := strings.Split(baseTypes, ",")
	for _, part := range parts {
		typeName := NormalizeTypeReference(part)
		if typeName == "" {
			continue
		}
		// Interfaces typically start with I
		if strings.HasPrefix(typeName, "I") && len(typeName) > 1 {
			dep := ExtractedDependency{
				Target:    typeName,
				Injection: "implements",
			}
			node.Dependencies = append(node.Dependencies, dep)
		} else {
			// Could be class inheritance
			dep := ExtractedDependency{
				Target:    typeName,
				Injection: "inherits",
			}
			node.Dependencies = append(node.Dependencies, dep)
		}
	}
}

// extractAttributes extracts class-level attributes from content
func (p *RegexCSharpParser) extractAttributes(content string) []string {
	var attributes []string
	// Match attributes like [Service], [ApiController], etc.
	attrPattern := regexp.MustCompile(`\[([\w]+)`)
	matches := attrPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			attr := match[1]
			if !seen[attr] {
				seen[attr] = true
				attributes = append(attributes, attr)
			}
		}
	}
	return attributes
}

// extractAttributesFromLines extracts attributes from lines preceding the given index
func (p *RegexCSharpParser) extractAttributesFromLines(lines []string, idx int) []string {
	var attributes []string
	attrPattern := regexp.MustCompile(`\[([\w]+)`)

	// Look back up to 5 lines for attributes
	for i := idx - 1; i >= 0 && i >= idx-5; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "[") {
			if matches := attrPattern.FindStringSubmatch(line); len(matches) > 1 {
				attributes = append(attributes, matches[1])
			}
		} else if line != "" && !strings.HasPrefix(line, "///") {
			// Stop looking when we hit non-empty, non-comment, non-attribute lines
			break
		}
	}

	// Reverse to get correct order
	for i, j := 0, len(attributes)-1; i < j; i, j = i+1, j-1 {
		attributes[i], attributes[j] = attributes[j], attributes[i]
	}

	return attributes
}

func (p *RegexCSharpParser) parseParameters(params string) []ExtractedParameter {
	var result []ExtractedParameter
	if strings.TrimSpace(params) == "" {
		return result
	}

	parts := strings.Split(params, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle "Type name" or "Type name = default" or generic types like "List<T> name"
		// First, try to split by last space to separate type and name
		lastSpace := strings.LastIndex(part, " ")
		if lastSpace > 0 {
			paramName := strings.TrimSpace(part[lastSpace:])
			paramType := strings.TrimSpace(part[:lastSpace])
			// Remove default value if present
			if eqIdx := strings.Index(paramName, "="); eqIdx > 0 {
				paramName = strings.TrimSpace(paramName[:eqIdx])
			}
			result = append(result, ExtractedParameter{
				Type: paramType,
				Name: paramName,
			})
		}
	}

	return result
}

func (p *RegexCSharpParser) extractDependenciesFromParams(params string, node *ExtractedNode) {
	paramList := p.parseParameters(params)
	for _, param := range paramList {
		typeName := NormalizeTypeReference(param.Type)
		// Interface types are likely dependencies
		if strings.HasPrefix(typeName, "I") && len(typeName) > 1 {
			dep := ExtractedDependency{
				Target:    typeName,
				FieldName: param.Name,
				Injection: "constructor",
			}
			// Avoid duplicates
			exists := false
			for _, existing := range node.Dependencies {
				if existing.Target == dep.Target {
					exists = true
					break
				}
			}
			if !exists {
				node.Dependencies = append(node.Dependencies, dep)
			}
		}
	}
}

// Ensure RegexCSharpParser implements Parser interface
var _ Parser = (*RegexCSharpParser)(nil)
var _ NamedNodeParser = (*RegexCSharpParser)(nil)
