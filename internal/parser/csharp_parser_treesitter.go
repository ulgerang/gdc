//go:build treesitter
// +build treesitter

package parser

import (
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
)

// CSharpParser parses C# source files using tree-sitter for accurate AST-based extraction
// Falls back to regex-based parsing if tree-sitter fails
type CSharpParser struct {
	parser      *sitter.Parser
	csharpLang  *sitter.Language
	regexParser *RegexCSharpParser // fallback
}

// NewCSharpParser creates a new C# parser with tree-sitter support
func NewCSharpParser() *CSharpParser {
	csharpLang := csharp.GetLanguage()
	parser := sitter.NewParser()
	parser.SetLanguage(csharpLang)

	return &CSharpParser{
		parser:      parser,
		csharpLang:  csharpLang,
		regexParser: NewRegexCSharpParser(),
	}
}

// Language returns "csharp"
func (p *CSharpParser) Language() string {
	return "csharp"
}

// ParseFile parses a C# source file using tree-sitter
func (p *CSharpParser) ParseFile(filePath string) (*ExtractedNode, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Parse the source code
	tree, err := p.parser.Parse(nil, content)
	if err != nil {
		// Fall back to regex parser on tree-sitter error
		return p.regexParser.ParseFile(filePath)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return p.regexParser.ParseFile(filePath)
	}

	extracted := &ExtractedNode{
		FilePath: filePath,
		Language: "csharp",
	}

	// Extract namespace
	extracted.Namespace = p.extractNamespace(root, content)

	// Extract type declaration (class, interface, struct, enum)
	if !p.extractTypeDeclaration(root, content, extracted) {
		// No type found, fall back to regex
		return p.regexParser.ParseFile(filePath)
	}

	// Extract class-level attributes
	extracted.Attributes = p.extractTypeAttributes(root, content)

	// Extract members
	p.extractMembers(root, content, extracted)

	// Extract dependencies from constructor parameters
	p.extractDependencies(extracted)

	return extracted, nil
}

// extractNamespace extracts the namespace from the compilation unit
func (p *CSharpParser) extractNamespace(root *sitter.Node, content []byte) string {
	// Query for namespace declaration
	queryStr := `(namespace_declaration name: (qualified_name) @name)`
	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return ""
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			return capture.Node.Content(content)
		}
	}
	return ""
}

// extractTypeDeclaration extracts the main type (class, interface, struct, enum)
func (p *CSharpParser) extractTypeDeclaration(root *sitter.Node, content []byte, extracted *ExtractedNode) bool {
	// Query for all type declarations
	queryStr := `[
		(class_declaration name: (identifier) @class_name)
		(interface_declaration name: (identifier) @interface_name)
		(struct_declaration name: (identifier) @struct_name)
		(enum_declaration name: (identifier) @enum_name)
	]`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return false
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	// Get the first type declaration
	match, ok := cursor.NextMatch()
	if !ok {
		return false
	}

	for _, capture := range match.Captures {
		node := capture.Node
		typeNode := node.Parent()

		extracted.ID = node.Content(content)

		// Determine type
		switch typeNode.Type() {
		case "class_declaration":
			extracted.Type = "class"
		case "interface_declaration":
			extracted.Type = "interface"
		case "struct_declaration":
			extracted.Type = "struct"
		case "enum_declaration":
			extracted.Type = "enum"
		}

		// Extract base types for dependencies
		p.extractBaseTypes(typeNode, content, extracted)

		return true
	}

	return false
}

// extractTypeAttributes extracts class-level attributes
func (p *CSharpParser) extractTypeAttributes(root *sitter.Node, content []byte) []string {
	var attributes []string

	queryStr := `(class_declaration 
		(attribute_list (attribute name: (identifier) @attr_name))
	)`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return attributes
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			attrName := capture.Node.Content(content)
			// Deduplicate
			found := false
			for _, existing := range attributes {
				if existing == attrName {
					found = true
					break
				}
			}
			if !found {
				attributes = append(attributes, attrName)
			}
		}
	}

	return attributes
}

// extractBaseTypes extracts inheritance/implementation dependencies
func (p *CSharpParser) extractBaseTypes(typeNode *sitter.Node, content []byte, extracted *ExtractedNode) {
	// Look for base_list
	for i := 0; i < int(typeNode.ChildCount()); i++ {
		child := typeNode.Child(i)
		if child.Type() == "base_list" {
			// Extract all base types
			for j := 0; j < int(child.ChildCount()); j++ {
				baseTypeNode := child.Child(j)
				if baseTypeNode.Type() == ":" || baseTypeNode.Type() == "," {
					continue
				}
				baseType := baseTypeNode.Content(content)
				baseType = strings.TrimSpace(baseType)

				if baseType != "" {
					// Determine injection type based on naming convention
					injection := "implements"
					if !strings.HasPrefix(baseType, "I") || len(baseType) <= 1 {
						// Could be class inheritance
						injection = "inherits"
					}

					dep := ExtractedDependency{
						Target:    baseType,
						Injection: injection,
					}
					extracted.Dependencies = append(extracted.Dependencies, dep)
				}
			}
		}
	}
}

// extractMembers extracts all members (methods, properties, constructors, events, fields)
func (p *CSharpParser) extractMembers(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	// Extract constructors
	p.extractConstructors(root, content, extracted)

	// Extract methods
	p.extractMethods(root, content, extracted)

	// Extract properties
	p.extractProperties(root, content, extracted)

	// Extract events
	p.extractEvents(root, content, extracted)

	// Extract fields (for potential dependencies)
	p.extractFields(root, content, extracted)
}

// extractConstructors extracts constructor declarations
func (p *CSharpParser) extractConstructors(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(constructor_declaration
		type: (identifier) @ctor_name
		parameters: (parameter_list) @params
		body: (_)? @body
	) @ctor`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var ctorName, params string
		var ctorNode *sitter.Node

		for _, capture := range match.Captures {
			switch capture.Node.Type() {
			case "identifier":
				ctorName = capture.Node.Content(content)
			case "parameter_list":
				params = capture.Node.Content(content)
			case "constructor_declaration":
				ctorNode = capture.Node
			}
		}

		// Only extract constructors for the main type
		if ctorName != extracted.ID {
			continue
		}

		// Get modifiers and documentation
		modifiers := p.getModifiers(ctorNode, content)
		docComment := p.getDocumentationComment(ctorNode, content)

		// Skip private/protected constructors unless they're the only ones
		isPublic := modifiers["public"]

		ctor := ExtractedConstructor{
			Signature:   ctorName + params,
			Description: docComment,
			Parameters:  p.parseParametersFromNode(ctorNode, content),
		}

		if isPublic || len(extracted.Constructors) == 0 {
			extracted.Constructors = append(extracted.Constructors, ctor)
		}

		// Extract dependencies from constructor parameters
		p.extractDependenciesFromConstructor(ctor, extracted)
	}
}

// extractMethods extracts method declarations
func (p *CSharpParser) extractMethods(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(method_declaration
		(type)? @return_type
		(name) @method_name
		parameters: (parameter_list) @params
	) @method`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var methodName, returnType, params string
		var methodNode *sitter.Node

		for _, capture := range match.Captures {
			node := capture.Node
			switch {
			case node.Parent() != nil && node.Parent().Type() == "method_declaration":
				// This is a child of method_declaration
				parentType := node.Type()
				if parentType == "identifier" {
					methodName = node.Content(content)
				} else if parentType == "predefined_type" || parentType == "generic_name" {
					returnType = node.Content(content)
				}
			case node.Type() == "parameter_list":
				params = node.Content(content)
			case node.Type() == "method_declaration":
				methodNode = node
			}
		}

		if methodNode == nil || methodName == "" {
			continue
		}

		// Get full return type from method node
		returnType = p.getMethodReturnType(methodNode, content)

		// Get modifiers and documentation
		modifiers := p.getModifiers(methodNode, content)
		docComment := p.getDocumentationComment(methodNode, content)

		isPublic := modifiers["public"]
		isStatic := modifiers["static"]
		isAsync := modifiers["async"]
		isVirtual := modifiers["virtual"]
		isOverride := modifiers["override"]
		isAbstract := modifiers["abstract"]

		// Determine access modifier
		accessModifier := "private"
		if isPublic {
			accessModifier = "public"
		} else if modifiers["protected"] {
			accessModifier = "protected"
		} else if modifiers["internal"] {
			accessModifier = "internal"
		}

		// Extract attributes
		attributes := p.getAttributes(methodNode, content)

		// Build signature
		signature := p.buildMethodSignature(returnType, methodName, params, modifiers)

		method := ExtractedMethod{
			Name:        methodName,
			Signature:   signature,
			Returns:     returnType,
			Description: docComment,
			Parameters:  p.parseParametersFromNode(methodNode, content),
			IsPublic:    isPublic,
			Access:      accessModifier,
			Static:      isStatic,
			Async:       isAsync,
			Attributes:  attributes,
		}

		// Add modifier info to signature for clarity
		if isVirtual {
			method.Signature = "virtual " + method.Signature
		}
		if isOverride {
			method.Signature = "override " + method.Signature
		}
		if isAbstract {
			method.Signature = "abstract " + method.Signature
		}

		extracted.Methods = append(extracted.Methods, method)
	}
}

// extractProperties extracts property declarations
func (p *CSharpParser) extractProperties(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(property_declaration
		type: (_) @prop_type
		name: (identifier) @prop_name
		(accessor_list)? @accessors
	) @property`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var propName, propType string
		var accessors []string
		var propNode *sitter.Node

		for _, capture := range match.Captures {
			node := capture.Node
			switch node.Type() {
			case "property_declaration":
				propNode = node
				// Extract type and name from property node
				propType = p.getPropertyType(node, content)
				propName = p.getPropertyName(node, content)
			case "accessor_list":
				// Extract accessors
				for i := 0; i < int(node.ChildCount()); i++ {
					child := node.Child(i)
					if child.Type() == "get_accessor_declaration" {
						accessors = append(accessors, "get")
					} else if child.Type() == "set_accessor_declaration" {
						accessors = append(accessors, "set")
					} else if child.Type() == "init_accessor_declaration" {
						accessors = append(accessors, "init")
					}
				}
			}
		}

		if propNode == nil || propName == "" {
			continue
		}

		// Get modifiers and documentation
		modifiers := p.getModifiers(propNode, content)
		docComment := p.getDocumentationComment(propNode, content)

		isPublic := modifiers["public"]

		access := strings.Join(accessors, "; ")
		if access == "" {
			access = "get; set"
		}

		prop := ExtractedProperty{
			Name:        propName,
			Type:        propType,
			Access:      access,
			Description: docComment,
			IsPublic:    isPublic,
		}

		extracted.Properties = append(extracted.Properties, prop)
	}
}

// extractEvents extracts event declarations
func (p *CSharpParser) extractEvents(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(event_declaration
		type: (_) @event_type
		(variable_declarator name: (identifier) @event_name)
	) @event`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var eventName, eventType string
		var eventNode *sitter.Node

		for _, capture := range match.Captures {
			node := capture.Node
			switch node.Type() {
			case "event_declaration":
				eventNode = node
			case "identifier":
				// Could be event name or type name
				if eventName == "" && node.Parent() != nil && node.Parent().Type() == "variable_declarator" {
					eventName = node.Content(content)
				}
			}
		}

		if eventNode == nil || eventName == "" {
			continue
		}

		// Get event type
		eventType = p.getEventType(eventNode, content)

		// Get modifiers and documentation
		modifiers := p.getModifiers(eventNode, content)
		docComment := p.getDocumentationComment(eventNode, content)

		isPublic := modifiers["public"]

		event := ExtractedEvent{
			Name:        eventName,
			Signature:   "event " + eventType + " " + eventName,
			Description: docComment,
			IsPublic:    isPublic,
		}

		extracted.Events = append(extracted.Events, event)
	}
}

// extractFields extracts field declarations (for dependency detection)
func (p *CSharpParser) extractFields(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(field_declaration
		type: (_) @field_type
		(variable_declarator name: (identifier) @field_name)
	) @field`

	query, err := sitter.NewQuery(p.csharpLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var fieldName, fieldType string
		var fieldNode *sitter.Node

		for _, capture := range match.Captures {
			node := capture.Node
			switch node.Type() {
			case "field_declaration":
				fieldNode = node
			case "identifier":
				if node.Parent() != nil && node.Parent().Type() == "variable_declarator" {
					fieldName = node.Content(content)
				}
			}
		}

		if fieldNode == nil || fieldName == "" {
			continue
		}

		// Get field type and modifiers
		fieldType = p.getFieldType(fieldNode, content)
		modifiers := p.getModifiers(fieldNode, content)

		// Check for injected dependencies (readonly private fields with interface types)
		if modifiers["private"] && (modifiers["readonly"] || modifiers["const"]) {
			if strings.HasPrefix(fieldType, "I") && len(fieldType) > 1 {
				dep := ExtractedDependency{
					Target:    fieldType,
					FieldName: fieldName,
					Injection: "field",
				}
				// Avoid duplicates
				exists := false
				for _, existing := range extracted.Dependencies {
					if existing.Target == dep.Target && existing.FieldName == dep.FieldName {
						exists = true
						break
					}
				}
				if !exists {
					extracted.Dependencies = append(extracted.Dependencies, dep)
				}
			}
		}
	}
}

// getModifiers extracts access and other modifiers from a declaration node
func (p *CSharpParser) getModifiers(node *sitter.Node, content []byte) map[string]bool {
	modifiers := make(map[string]bool)

	if node == nil {
		return modifiers
	}

	// Look for modifier_list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "modifier" || child.Type() == "modifier_list" {
			// Get all modifier keywords
			for j := 0; j < int(child.ChildCount()); j++ {
				modChild := child.Child(j)
				if modChild.Type() == "modifier" {
					modText := strings.ToLower(modChild.Content(content))
					modifiers[modText] = true
				}
			}
		}

		// Also check for modifier directly as child
		if child.Type() == "modifier" || child.Type() == "public" ||
			child.Type() == "private" || child.Type() == "protected" ||
			child.Type() == "internal" || child.Type() == "static" ||
			child.Type() == "async" || child.Type() == "virtual" ||
			child.Type() == "override" || child.Type() == "abstract" ||
			child.Type() == "readonly" || child.Type() == "const" {
			modText := strings.ToLower(child.Content(content))
			modifiers[modText] = true
		}
	}

	return modifiers
}

// getDocumentationComment extracts XML documentation comment
func (p *CSharpParser) getDocumentationComment(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for documentation_comment
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "documentation_comment" {
			comment := child.Content(content)
			// Parse summary content
			return p.parseXmlSummary(comment)
		}
	}

	return ""
}

// parseXmlSummary extracts the summary text from XML documentation
func (p *CSharpParser) parseXmlSummary(comment string) string {
	// Remove leading /// and trim
	lines := strings.Split(comment, "\n")
	var summaryLines []string
	inSummary := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "///")
		line = strings.TrimSpace(line)

		if strings.Contains(line, "<summary>") {
			inSummary = true
			// Extract content after <summary>
			idx := strings.Index(line, "<summary>")
			if idx != -1 {
				content := line[idx+9:] // len("<summary>") = 9
				content = strings.TrimSuffix(content, "</summary>")
				content = strings.TrimSpace(content)
				if content != "" {
					summaryLines = append(summaryLines, content)
				}
			}
		} else if strings.Contains(line, "</summary>") {
			content := strings.TrimSuffix(line, "</summary>")
			content = strings.TrimSpace(content)
			if content != "" {
				summaryLines = append(summaryLines, content)
			}
			break
		} else if inSummary {
			line = strings.TrimPrefix(line, "<")
			line = strings.TrimSuffix(line, ">")
			// Remove XML tags
			for {
				startIdx := strings.Index(line, "<")
				if startIdx == -1 {
					break
				}
				endIdx := strings.Index(line[startIdx:], ">")
				if endIdx == -1 {
					break
				}
				line = line[:startIdx] + " " + line[startIdx+endIdx+1:]
			}
			line = strings.TrimSpace(line)
			if line != "" {
				summaryLines = append(summaryLines, line)
			}
		}
	}

	return strings.Join(summaryLines, " ")
}

// getAttributes extracts attributes from a declaration node
func (p *CSharpParser) getAttributes(node *sitter.Node, content []byte) []string {
	var attributes []string

	if node == nil {
		return attributes
	}

	// Look for attribute_list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "attribute_list" {
			// Extract attribute names
			for j := 0; j < int(child.ChildCount()); j++ {
				attrChild := child.Child(j)
				if attrChild.Type() == "attribute" {
					// Get attribute name
					for k := 0; k < int(attrChild.ChildCount()); k++ {
						nameChild := attrChild.Child(k)
						if nameChild.Type() == "identifier" || nameChild.Type() == "name" {
							attrName := nameChild.Content(content)
							if attrName != "" {
								attributes = append(attributes, attrName)
							}
						}
					}
				}
			}
		}
	}

	return attributes
}

// getMethodReturnType extracts the return type from a method declaration
func (p *CSharpParser) getMethodReturnType(methodNode *sitter.Node, content []byte) string {
	for i := 0; i < int(methodNode.ChildCount()); i++ {
		child := methodNode.Child(i)
		switch child.Type() {
		case "type", "predefined_type", "identifier", "generic_name":
			return child.Content(content)
		}
	}
	return "void"
}

// getPropertyType extracts the type from a property declaration
func (p *CSharpParser) getPropertyType(propNode *sitter.Node, content []byte) string {
	for i := 0; i < int(propNode.ChildCount()); i++ {
		child := propNode.Child(i)
		switch child.Type() {
		case "type", "predefined_type", "identifier", "generic_name", "array_type":
			return child.Content(content)
		}
	}
	return ""
}

// getPropertyName extracts the name from a property declaration
func (p *CSharpParser) getPropertyName(propNode *sitter.Node, content []byte) string {
	for i := 0; i < int(propNode.ChildCount()); i++ {
		child := propNode.Child(i)
		if child.Type() == "identifier" || child.Type() == "name" {
			return child.Content(content)
		}
	}
	return ""
}

// getEventType extracts the type from an event declaration
func (p *CSharpParser) getEventType(eventNode *sitter.Node, content []byte) string {
	for i := 0; i < int(eventNode.ChildCount()); i++ {
		child := eventNode.Child(i)
		switch child.Type() {
		case "type", "predefined_type", "identifier", "generic_name":
			return child.Content(content)
		}
	}
	return "EventHandler"
}

// getFieldType extracts the type from a field declaration
func (p *CSharpParser) getFieldType(fieldNode *sitter.Node, content []byte) string {
	for i := 0; i < int(fieldNode.ChildCount()); i++ {
		child := fieldNode.Child(i)
		switch child.Type() {
		case "type", "predefined_type", "identifier", "generic_name", "array_type":
			return child.Content(content)
		}
	}
	return ""
}

// parseParametersFromNode extracts parameters from a method/constructor node
func (p *CSharpParser) parseParametersFromNode(node *sitter.Node, content []byte) []ExtractedParameter {
	var params []ExtractedParameter

	if node == nil {
		return params
	}

	// Find parameter_list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_list" {
			// Parse each parameter
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "parameter" {
					param := p.parseParameter(paramChild, content)
					if param.Name != "" {
						params = append(params, param)
					}
				}
			}
		}
	}

	return params
}

// parseParameter extracts a single parameter
func (p *CSharpParser) parseParameter(paramNode *sitter.Node, content []byte) ExtractedParameter {
	param := ExtractedParameter{}

	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		switch child.Type() {
		case "identifier":
			param.Name = child.Content(content)
		case "type", "predefined_type", "generic_name", "array_type":
			param.Type = child.Content(content)
		}
	}

	return param
}

// buildMethodSignature constructs a method signature string
func (p *CSharpParser) buildMethodSignature(returnType, name, params string, modifiers map[string]bool) string {
	var parts []string

	if modifiers["public"] {
		parts = append(parts, "public")
	} else if modifiers["private"] {
		parts = append(parts, "private")
	} else if modifiers["protected"] {
		parts = append(parts, "protected")
	} else if modifiers["internal"] {
		parts = append(parts, "internal")
	}

	if modifiers["static"] {
		parts = append(parts, "static")
	}
	if modifiers["async"] {
		parts = append(parts, "async")
	}
	if modifiers["virtual"] {
		parts = append(parts, "virtual")
	}
	if modifiers["override"] {
		parts = append(parts, "override")
	}
	if modifiers["abstract"] {
		parts = append(parts, "abstract")
	}

	if returnType != "" && returnType != "void" {
		parts = append(parts, returnType)
	}

	parts = append(parts, name+params)

	return strings.Join(parts, " ")
}

// extractDependenciesFromConstructor extracts dependencies from constructor parameters
func (p *CSharpParser) extractDependenciesFromConstructor(ctor ExtractedConstructor, extracted *ExtractedNode) {
	for _, param := range ctor.Parameters {
		// Interface types (starting with I) are likely dependencies
		if strings.HasPrefix(param.Type, "I") && len(param.Type) > 1 {
			dep := ExtractedDependency{
				Target:    param.Type,
				FieldName: param.Name,
				Injection: "constructor",
			}
			// Avoid duplicates
			exists := false
			for _, existing := range extracted.Dependencies {
				if existing.Target == dep.Target && existing.FieldName == dep.FieldName {
					exists = true
					break
				}
			}
			if !exists {
				extracted.Dependencies = append(extracted.Dependencies, dep)
			}
		}
	}
}

// extractDependencies deduplicates and consolidates all dependencies
func (p *CSharpParser) extractDependencies(extracted *ExtractedNode) {
	// Remove duplicate dependencies
	seen := make(map[string]bool)
	var uniqueDeps []ExtractedDependency

	for _, dep := range extracted.Dependencies {
		key := dep.Target + ":" + dep.FieldName
		if !seen[key] {
			seen[key] = true
			uniqueDeps = append(uniqueDeps, dep)
		}
	}

	extracted.Dependencies = uniqueDeps
}

// Ensure CSharpParser implements Parser interface
var _ Parser = (*CSharpParser)(nil)
