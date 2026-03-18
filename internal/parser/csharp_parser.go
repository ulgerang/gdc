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

// Regex patterns for C# parsing
var (
	csClassPattern       = regexp.MustCompile(`(?:public|internal|private|protected)?\s*(?:abstract|sealed|static|partial)?\s*class\s+(\w+)(?:\s*:\s*([^{]+))?`)
	csInterfacePattern   = regexp.MustCompile(`(?:public|internal)?\s*interface\s+(\w+)(?:\s*:\s*([^{]+))?`)
	csMethodPattern      = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(virtual|override|abstract|static|async)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*\(([^)]*)\)`)
	csPropertyPattern    = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(virtual|override|abstract|static)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*\{\s*([^}]*)\}`)
	csEventPattern       = regexp.MustCompile(`(?:public|protected|private|internal)?\s*event\s+(\w+(?:<[^>]+>)?)\s+(\w+)\s*;`)
	csConstructorPattern = regexp.MustCompile(`(?:public|protected|private|internal)\s+(\w+)\s*\(([^)]*)\)`)
	csXmlDocPattern      = regexp.MustCompile(`///\s*<summary>\s*(.+?)\s*</summary>`)
	csFieldPattern       = regexp.MustCompile(`(?:public|protected|private|internal)?\s*(?:readonly|const)?\s*(\w+(?:<[^>]+>)?(?:\[\])?)\s+(\w+)\s*(?:=|;)`)
)

// ParseFile parses a C# source file using regex-based extraction
func (p *RegexCSharpParser) ParseFile(filePath string) (*ExtractedNode, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
			if matches := csXmlDocPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
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
