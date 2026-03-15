//go:build !treesitter
// +build !treesitter

package parser

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// TypeScriptParser parses TypeScript source files using regex-based extraction
// This is the default implementation. To use tree-sitter, build with -tags treesitter
type TypeScriptParser struct {
	regexParser *RegexTypeScriptParser
}

// NewTypeScriptParser creates a new TypeScript parser
func NewTypeScriptParser() *TypeScriptParser {
	return &TypeScriptParser{
		regexParser: NewRegexTypeScriptParser(),
	}
}

// Language returns "typescript"
func (p *TypeScriptParser) Language() string {
	return "typescript"
}

// ParseFile parses a TypeScript source file
func (p *TypeScriptParser) ParseFile(filePath string) (*ExtractedNode, error) {
	// Delegate to the regex parser
	return p.regexParser.ParseFile(filePath)
}

// Ensure TypeScriptParser implements Parser interface
var _ Parser = (*TypeScriptParser)(nil)

// RegexTypeScriptParser is the regex-based parser used as default
type RegexTypeScriptParser struct{}

// NewRegexTypeScriptParser creates a new regex-based TypeScript parser
func NewRegexTypeScriptParser() *RegexTypeScriptParser {
	return &RegexTypeScriptParser{}
}

// Language returns "typescript"
func (p *RegexTypeScriptParser) Language() string {
	return "typescript"
}

// Regex patterns for TypeScript parsing
var (
	// Type detection patterns - these should match at start of line to avoid matching in comments
	tsClassPattern       = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:\s*<[^>]+>)?(?:\s+extends\s+(\w+)(?:\s*<[^>]+>)?)?(?:\s+implements\s+([^{]+))?`)
	tsInterfacePattern   = regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+(\w+)(?:\s*<[^>]+>)?(?:\s+extends\s+([^{]+))?`)
	tsFunctionPattern    = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)(?:\s*<[^>]+>)?\s*\(([^)]*)\)(?:\s*:\s*([^{;]+))?`)
	tsMethodPattern      = regexp.MustCompile(`(?:(public|private|protected)\s+)?(?:(static|abstract|readonly|async)\s+)*(?:(readonly)\s+)?(\w+)\s*\??\s*(?:\(([^)]*)\)\s*(?::\s*([^{;]+))?|:\s*([^;=]+))`)
	tsPropertyPattern    = regexp.MustCompile(`(?:(public|private|protected)\s+)?(?:(static|abstract|readonly)\s+)*(?:(readonly)\s+)?(\w+)\s*\??\s*:\s*([^;=]+)`)
	tsConstructorPattern = regexp.MustCompile(`constructor\s*\(([^)]*)\)`)
	tsJsDocPattern       = regexp.MustCompile(`(?s)/\*\*\s*(.+?)\s*\*/`)
	tsLineCommentPattern = regexp.MustCompile(`//\s*(.+)`)
	tsArrowMethodPattern = regexp.MustCompile(`(?:(public|private|protected)\s+)?(\w+)\s*\??\s*=\s*(?:async\s+)?\(([^)]*)\)\s*(?::\s*([^=]+))?\s*=>`)
	tsDecoratorPattern   = regexp.MustCompile(`@(\w+)(?:\([^)]*\))?`)
	tsGetterPattern      = regexp.MustCompile(`(?:(public|private|protected)\s+)?get\s+(\w+)\s*\(\s*\)\s*:\s*(\w+)`)
	tsSetterPattern      = regexp.MustCompile(`(?:(public|private|protected)\s+)?set\s+(\w+)\s*\(([^)]+)\)`)
	// Interface method pattern: methodName(params): returnType;
	tsInterfaceMethodPattern = regexp.MustCompile(`^\s*(\w+)\s*\(([^)]*)\)\s*(?::\s*([^;]+))?\s*;?\s*$`)
)

// ParseFile parses a TypeScript source file using regex-based extraction
func (p *RegexTypeScriptParser) ParseFile(filePath string) (*ExtractedNode, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	extracted := &ExtractedNode{
		FilePath: filePath,
		// Language-specific fields for Hybrid Specification Strategy
		Language: "typescript",
		Module:   p.extractModulePath(filePath), // TypeScript-specific
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

	// Extract decorators/annotations
	extracted.Attributes = p.extractDecorators(content)

	// Try to find class, interface, or function
	if matches := tsClassPattern.FindStringSubmatch(content); len(matches) > 1 {
		extracted.ID = matches[1]
		extracted.Type = "class"

		// Extract extends (base class)
		if len(matches) > 2 && matches[2] != "" {
			dep := ExtractedDependency{
				Target:    strings.TrimSpace(matches[2]),
				Injection: "extends",
			}
			extracted.Dependencies = append(extracted.Dependencies, dep)
		}

		// Extract implements
		if len(matches) > 3 && matches[3] != "" {
			p.extractImplements(matches[3], extracted)
		}

		// Extract multi-line constructor from full content
		p.extractConstructorFromContent(content, extracted)
	} else if matches := tsInterfacePattern.FindStringSubmatch(content); len(matches) > 1 {
		extracted.ID = matches[1]
		extracted.Type = "interface"

		// Extract extends for interface
		if len(matches) > 2 && matches[2] != "" {
			p.extractImplements(matches[2], extracted)
		}
	} else if matches := tsFunctionPattern.FindStringSubmatch(content); len(matches) > 1 {
		extracted.ID = matches[1]
		extracted.Type = "function"
	}

	// Parse line by line
	var pendingDoc string
	var inMultiLineComment bool
	var multiLineDocBuffer strings.Builder
	braceDepth := 0
	inClass := false
	classBodyDepth := 0 // Depth at which class-level members should be extracted

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track multi-line comments (JSDoc)
		if strings.Contains(trimmed, "/**") {
			inMultiLineComment = true
			multiLineDocBuffer.Reset()
			// Check if same line ends
			if strings.Contains(trimmed, "*/") {
				inMultiLineComment = false
				if matches := tsJsDocPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
					pendingDoc = p.extractJsDocDescription(matches[1])
				}
			}
			continue
		}

		if inMultiLineComment {
			if strings.Contains(trimmed, "*/") {
				inMultiLineComment = false
				// Extract description from accumulated doc
				docContent := multiLineDocBuffer.String()
				if docContent != "" {
					pendingDoc = docContent
				}
			} else {
				// Accumulate doc content, strip leading *
				docLine := strings.TrimPrefix(trimmed, "*")
				docLine = strings.TrimSpace(docLine)
				if docLine != "" && !strings.HasPrefix(docLine, "@") {
					if multiLineDocBuffer.Len() > 0 {
						multiLineDocBuffer.WriteString(" ")
					}
					multiLineDocBuffer.WriteString(docLine)
				}
			}
			continue
		}

		// Check if we're inside the main class/interface (before brace counting)
		if !inClass && extracted.ID != "" &&
			(strings.Contains(line, "class "+extracted.ID) ||
				strings.Contains(line, "interface "+extracted.ID) ||
				strings.Contains(line, "function "+extracted.ID)) {
			inClass = true
			// Set the depth at which class-level members should be extracted
			// This is the depth AFTER the class opening brace
			classBodyDepth = braceDepth + 1
		}

		// Count braces AFTER checking for class/interface declaration
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if !inClass && extracted.Type != "" {
			continue
		}

		// Stop processing if we've exited the main class body
		// (braceDepth goes below classBodyDepth after being in the class)
		if inClass && classBodyDepth > 0 && braceDepth < classBodyDepth {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			// braceDepth already counted at end of loop iteration
			continue
		}

		// For interfaces, we don't have method bodies, so depth check is different
		// Interface members are at braceDepth == classBodyDepth, not deeper
		isInterface := extracted.Type == "interface"

		// Only extract members at class body level, not inside methods
		// For interfaces, skip this check since there are no method bodies
		// For classes, braceDepth > classBodyDepth means we're inside a method body
		// BUT: we need to handle the case where the method signature line ends with {
		// In that case, we still want to extract the method, so we check if this line
		// looks like a method/property declaration before skipping
		opensBlock := strings.HasSuffix(trimmed, "{") || strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}")
		if !isInterface && braceDepth > classBodyDepth && inClass && classBodyDepth > 0 && !opensBlock {
			// We're inside a method body or deeper - skip extraction
			pendingDoc = "" // Clear pending doc when inside method body
			continue
		}

		// Line comment for pending doc (only if no pending doc from JSDoc)
		if matches := tsLineCommentPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
			if pendingDoc == "" {
				pendingDoc = strings.TrimSpace(matches[1])
			}
			continue
		}

		// Getter
		if matches := tsGetterPattern.FindStringSubmatch(line); len(matches) > 3 {
			isPublic := matches[1] == "" || matches[1] == "public"
			prop := ExtractedProperty{
				Name:        matches[2],
				Type:        strings.TrimSpace(matches[3]),
				Access:      "get",
				Description: pendingDoc,
				IsPublic:    isPublic,
			}
			extracted.Properties = append(extracted.Properties, prop)
			pendingDoc = ""
			continue
		}

		// Setter
		if matches := tsSetterPattern.FindStringSubmatch(line); len(matches) > 3 {
			isPublic := matches[1] == "" || matches[1] == "public"
			// Check if property already exists (getter was found)
			found := false
			for i, prop := range extracted.Properties {
				if prop.Name == matches[2] {
					extracted.Properties[i].Access = "get; set"
					found = true
					break
				}
			}
			if !found {
				// Extract type from setter parameter
				paramParts := strings.Split(matches[3], ":")
				propType := ""
				if len(paramParts) > 1 {
					propType = strings.TrimSpace(paramParts[1])
				}
				prop := ExtractedProperty{
					Name:        matches[2],
					Type:        propType,
					Access:      "set",
					Description: pendingDoc,
					IsPublic:    isPublic,
				}
				extracted.Properties = append(extracted.Properties, prop)
			}
			pendingDoc = ""
			continue
		}

		// Constructor
		if matches := tsConstructorPattern.FindStringSubmatch(line); len(matches) > 1 {
			params := p.normalizeParams(matches[1])
			ctor := ExtractedConstructor{
				Signature:   "constructor(" + params + ")",
				Description: pendingDoc,
				Parameters:  p.parseParameters(params),
			}
			extracted.Constructors = append(extracted.Constructors, ctor)

			// Extract dependencies from constructor parameters
			p.extractDependenciesFromParams(params, extracted)
			pendingDoc = ""
			continue
		}

		// Arrow function method
		if matches := tsArrowMethodPattern.FindStringSubmatch(line); len(matches) > 1 {
			accessModifier := matches[1]
			if accessModifier == "" {
				accessModifier = "public"
			}
			isPublic := accessModifier == "public"
			isAsync := strings.Contains(line, "async")

			returnType := ""
			if len(matches) > 4 && matches[4] != "" {
				returnType = strings.TrimSpace(matches[4])
			}

			params := ""
			if len(matches) > 3 {
				params = matches[3]
			}
			_ = params // Extracted for potential future use

			method := ExtractedMethod{
				Name:        matches[2],
				Signature:   trimmed,
				Returns:     returnType,
				Description: pendingDoc,
				IsPublic:    isPublic,
				// TypeScript-specific fields (Hybrid Specification Body)
				Async:  isAsync,
				Access: accessModifier,
			}
			extracted.Methods = append(extracted.Methods, method)
			pendingDoc = ""
			continue
		}

		// Interface method pattern (no access modifiers): methodName(params): returnType;
		if isInterface {
			if matches := tsInterfaceMethodPattern.FindStringSubmatch(trimmed); len(matches) > 1 {
				methodName := matches[1]
				params := ""
				if len(matches) > 2 {
					params = matches[2]
				}
				returnType := ""
				if len(matches) > 3 {
					returnType = strings.TrimSpace(matches[3])
				}

				// Skip if this looks like a property (has : but no parentheses)
				if !strings.Contains(trimmed, "(") {
					// This is a property, not a method - will be handled below
				} else {
					signature := methodName + "(" + params + ")"
					if returnType != "" {
						signature += ": " + returnType
					}

					method := ExtractedMethod{
						Name:        methodName,
						Returns:     returnType,
						Signature:   signature,
						Description: pendingDoc,
						Parameters:  p.parseParameters(params),
						IsPublic:    true,
						Access:      "public",
					}
					extracted.Methods = append(extracted.Methods, method)
					pendingDoc = ""
					continue
				}
			}
		}

		// Method (but not constructor)
		// We check this first but if it's actually a property, we fall through to property check
		if matches := tsMethodPattern.FindStringSubmatch(line); len(matches) > 4 {
			// Skip constructor
			if matches[4] == "constructor" {
				pendingDoc = ""
				continue
			}

			// Check if it looks like a method (has parentheses)
			if strings.Contains(line, "(") {
				// Determine access modifier
				accessModifier := "public"
				if matches[1] == "private" {
					accessModifier = "private"
				} else if matches[1] == "protected" {
					accessModifier = "protected"
				}

				// Check for modifiers
				isStatic := strings.Contains(line, "static") || matches[2] == "static" || matches[3] == "static"
				isAsync := strings.Contains(line, "async") || matches[2] == "async"
				isAbstract := strings.Contains(line, "abstract") || matches[2] == "abstract"
				isPublic := accessModifier == "public"

				methodName := matches[4]
				params := ""
				if len(matches) > 5 {
					params = matches[5]
				}

				returnType := ""
				if len(matches) > 6 && matches[6] != "" {
					returnType = strings.TrimSpace(matches[6])
				}

				signature := p.buildMethodSignature(methodName, params, returnType, accessModifier, isStatic, isAsync, isAbstract)

				method := ExtractedMethod{
					Name:        methodName,
					Returns:     returnType,
					Signature:   signature,
					Description: pendingDoc,
					Parameters:  p.parseParameters(params),
					IsPublic:    isPublic,
					// TypeScript-specific fields (Hybrid Specification Body)
					Async:  isAsync,
					Static: isStatic,
					Access: accessModifier,
				}
				extracted.Methods = append(extracted.Methods, method)
				pendingDoc = ""
				continue
			}
			// If no parentheses, fall through to property check
		}

		// Property (check after method)
		if matches := tsPropertyPattern.FindStringSubmatch(line); len(matches) > 5 {
			// Skip if it looks like a method parameter or it's the constructor line
			if strings.Contains(line, "(") && !strings.Contains(line, "=>") {
				continue
			}
			if strings.Contains(line, "constructor") {
				continue
			}

			// Determine access modifier
			accessModifier := "public"
			if matches[1] == "private" {
				accessModifier = "private"
			} else if matches[1] == "protected" {
				accessModifier = "protected"
			}

			isReadonly := strings.Contains(line, "readonly") || matches[2] == "readonly" || matches[3] == "readonly"
			isStatic := strings.Contains(line, "static") || matches[2] == "static"
			_ = isStatic // Currently unused, but extracted for future use
			isPublic := accessModifier == "public"

			// Regex capture groups:
			// matches[1] = access modifier (public/private/protected)
			// matches[2] = first modifier (static/abstract/readonly)
			// matches[3] = second readonly (if present)
			// matches[4] = property name
			// matches[5] = property type
			propName := matches[4]
			propType := strings.TrimSpace(matches[5])
			// Handle optional marker
			if strings.HasSuffix(propName, "?") {
				propName = strings.TrimSuffix(propName, "?")
				propType = propType + " (optional)"
			}

			access := "get; set"
			if isReadonly {
				access = "get"
			}

			prop := ExtractedProperty{
				Name:        propName,
				Type:        propType,
				Access:      access,
				Description: pendingDoc,
				IsPublic:    isPublic,
			}

			extracted.Properties = append(extracted.Properties, prop)
			pendingDoc = ""
			continue
		}

		pendingDoc = ""
	}

	return extracted, nil
}

// extractModulePath extracts TypeScript module path from file path
// e.g., "src/services/UserService.ts" -> "services/UserService"
func (p *RegexTypeScriptParser) extractModulePath(filePath string) string {
	// Normalize path separators
	normalized := strings.ReplaceAll(filePath, "\\", "/")

	// Try to find src/, lib/, app/, or packages/ as module root
	for _, root := range []string{"/src/", "/lib/", "/app/", "/packages/"} {
		if idx := strings.Index(normalized, root); idx >= 0 {
			modulePath := normalized[idx+len(root):]
			// Remove extension
			if extIdx := strings.LastIndex(modulePath, "."); extIdx > 0 {
				modulePath = modulePath[:extIdx]
			}
			return modulePath
		}
	}

	// Fallback: use filename without extension
	base := strings.ReplaceAll(filePath, "\\", "/")
	if lastSlash := strings.LastIndex(base, "/"); lastSlash >= 0 {
		base = base[lastSlash+1:]
	}
	if extIdx := strings.LastIndex(base, "."); extIdx > 0 {
		base = base[:extIdx]
	}
	return base
}

// extractJsDocDescription extracts the description from JSDoc comment
func (p *RegexTypeScriptParser) extractJsDocDescription(docContent string) string {
	lines := strings.Split(docContent, "\n")
	var descriptions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip lines starting with @param, @returns, etc.
		if strings.HasPrefix(line, "@") {
			continue
		}
		// Remove leading asterisks
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" && line != "/" {
			descriptions = append(descriptions, line)
		}
	}
	return strings.Join(descriptions, " ")
}

// extractDecorators extracts TypeScript decorators from content
func (p *RegexTypeScriptParser) extractDecorators(content string) []string {
	var decorators []string
	seen := make(map[string]bool)

	matches := tsDecoratorPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			decorator := match[1]
			if !seen[decorator] {
				seen[decorator] = true
				decorators = append(decorators, decorator)
			}
		}
	}

	return decorators
}

func (p *RegexTypeScriptParser) extractImplements(implements string, node *ExtractedNode) {
	parts := strings.Split(implements, ",")
	for _, part := range parts {
		typeName := strings.TrimSpace(part)
		// Remove generic parameters
		if idx := strings.Index(typeName, "<"); idx > 0 {
			typeName = typeName[:idx]
		}
		if typeName == "" {
			continue
		}

		dep := ExtractedDependency{
			Target:    typeName,
			Injection: "implements",
		}
		node.Dependencies = append(node.Dependencies, dep)
	}
}

func (p *RegexTypeScriptParser) normalizeParams(params string) string {
	// Normalize whitespace and remove default values
	params = regexp.MustCompile(`\s+`).ReplaceAllString(params, " ")
	params = regexp.MustCompile(`\s*=\s*[^,)]+`).ReplaceAllString(params, "")
	return strings.TrimSpace(params)
}

func (p *RegexTypeScriptParser) parseParameters(params string) []ExtractedParameter {
	var result []ExtractedParameter
	if strings.TrimSpace(params) == "" {
		return result
	}

	parts := splitTSParameters(params)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle TypeScript destructuring and access modifiers
		// "public name: Type" -> "name: Type"
		part = regexp.MustCompile(`^(public|private|protected|readonly)\s+`).ReplaceAllString(part, "")

		// Handle destructuring: "{ name, age }" - skip these
		if strings.HasPrefix(part, "{") {
			continue
		}

		colonIdx := strings.Index(part, ":")
		if colonIdx > 0 {
			name := strings.TrimSpace(part[:colonIdx])
			// Remove optional marker
			name = strings.TrimSuffix(name, "?")
			// Remove default value if present
			if eqIdx := strings.Index(name, "="); eqIdx > 0 {
				name = strings.TrimSpace(name[:eqIdx])
			}

			typeName := strings.TrimSpace(part[colonIdx+1:])
			// Remove default value from type
			if eqIdx := strings.Index(typeName, "="); eqIdx > 0 {
				typeName = strings.TrimSpace(typeName[:eqIdx])
			}

			result = append(result, ExtractedParameter{
				Name: name,
				Type: typeName,
			})
		}
	}

	return result
}

// splitTSParameters handles nested generics when splitting parameters
func splitTSParameters(params string) []string {
	var result []string
	var current strings.Builder
	depth := 0
	inString := false
	stringChar := rune(0)

	for _, ch := range params {
		switch ch {
		case '"', '\'', '`':
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
				stringChar = 0
			}
			current.WriteRune(ch)
		case '<', '(', '{', '[':
			if !inString {
				depth++
			}
			current.WriteRune(ch)
		case '>', ')', '}', ']':
			if !inString {
				depth--
			}
			current.WriteRune(ch)
		case ',':
			if !inString && depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func (p *RegexTypeScriptParser) extractDependenciesFromParams(params string, node *ExtractedNode) {
	paramList := p.parseParameters(params)
	for _, param := range paramList {
		// Check for common DI patterns
		typeName := param.Type
		// Remove generics and arrays
		if idx := strings.Index(typeName, "<"); idx > 0 {
			typeName = typeName[:idx]
		}
		typeName = strings.TrimSuffix(typeName, "[]")
		typeName = strings.TrimSpace(typeName)

		// Service or interface-like types
		if strings.HasSuffix(typeName, "Service") ||
			strings.HasSuffix(typeName, "Repository") ||
			strings.HasSuffix(typeName, "Manager") ||
			strings.HasSuffix(typeName, "Client") ||
			strings.HasSuffix(typeName, "Config") ||
			strings.HasPrefix(typeName, "I") {

			dep := ExtractedDependency{
				Target:    typeName,
				FieldName: param.Name,
				Injection: "constructor",
			}
			// Avoid duplicates
			exists := false
			for _, existing := range node.Dependencies {
				if existing.Target == dep.Target && existing.FieldName == dep.FieldName {
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

// buildMethodSignature builds a full method signature string
func (p *RegexTypeScriptParser) buildMethodSignature(name, params, returnType, access string, isStatic, isAsync, isAbstract bool) string {
	var parts []string

	if access != "" && access != "public" {
		parts = append(parts, access)
	}
	if isStatic {
		parts = append(parts, "static")
	}
	if isAbstract {
		parts = append(parts, "abstract")
	}
	if isAsync {
		parts = append(parts, "async")
	}

	parts = append(parts, name+"("+params+")")

	if returnType != "" {
		parts = append(parts, ":"+returnType)
	}

	return strings.Join(parts, " ")
}

// extractConstructorFromContent finds and parses multi-line constructors
func (p *RegexTypeScriptParser) extractConstructorFromContent(content string, node *ExtractedNode) {
	// Match constructor with potentially multi-line parameters
	constructorStart := strings.Index(content, "constructor(")
	if constructorStart == -1 {
		return
	}

	// Find matching closing parenthesis
	parenDepth := 0
	paramStart := constructorStart + len("constructor(")
	paramEnd := paramStart

	for i := paramStart; i < len(content); i++ {
		ch := content[i]
		switch ch {
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 {
				paramEnd = i
				goto foundParams
			}
			parenDepth--
		}
	}
	return

foundParams:
	params := content[paramStart:paramEnd]
	// Normalize whitespace
	params = p.normalizeParams(params)

	// Find JSDoc comment before constructor
	docComment := ""
	// Look for /** ... */ before constructor
	docPattern := regexp.MustCompile(`(?s)/\*\*\s*.+?\*/\s*constructor`)
	if match := docPattern.FindStringIndex(content); match != nil && match[1] > constructorStart-100 {
		docContent := content[match[0]:match[1]]
		docComment = p.extractJsDocDescription(docContent)
	}

	// Avoid duplicate constructors
	for _, existing := range node.Constructors {
		if strings.Contains(existing.Signature, "constructor(") {
			return
		}
	}

	ctor := ExtractedConstructor{
		Signature:   "constructor(" + params + ")",
		Description: docComment,
		Parameters:  p.parseParameters(params),
	}
	node.Constructors = append(node.Constructors, ctor)

	// Extract dependencies from constructor parameters
	p.extractDependenciesFromParams(params, node)
}

// Ensure RegexTypeScriptParser implements Parser interface
var _ Parser = (*RegexTypeScriptParser)(nil)
