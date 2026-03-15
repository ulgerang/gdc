//go:build treesitter
// +build treesitter

package parser

import (
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptParser parses TypeScript source files using tree-sitter for accurate AST-based extraction
// Falls back to regex-based parsing if tree-sitter fails
type TypeScriptParser struct {
	parser      *sitter.Parser
	tsLang      *sitter.Language
	regexParser *RegexTypeScriptParser // fallback
}

// NewTypeScriptParser creates a new TypeScript parser with tree-sitter support
func NewTypeScriptParser() *TypeScriptParser {
	tsLang := typescript.GetLanguage()
	parser := sitter.NewParser()
	parser.SetLanguage(tsLang)

	return &TypeScriptParser{
		parser:      parser,
		tsLang:      tsLang,
		regexParser: NewRegexTypeScriptParser(),
	}
}

// Language returns "typescript"
func (p *TypeScriptParser) Language() string {
	return "typescript"
}

// ParseFile parses a TypeScript source file using tree-sitter
func (p *TypeScriptParser) ParseFile(filePath string) (*ExtractedNode, error) {
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
		Language: "typescript",
		Module:   p.extractModulePath(filePath),
	}

	// Extract type declaration (class, interface, function, type alias)
	if !p.extractTypeDeclaration(root, content, extracted) {
		// No type found, fall back to regex
		return p.regexParser.ParseFile(filePath)
	}

	// Extract decorators/attributes
	extracted.Attributes = p.extractDecorators(root, content)

	// Extract members
	p.extractMembers(root, content, extracted)

	// Extract dependencies from constructor parameters and imports
	p.extractDependencies(extracted)

	return extracted, nil
}

// extractModulePath extracts TypeScript module path from file path
func (p *TypeScriptParser) extractModulePath(filePath string) string {
	// Delegate to regex parser for consistency
	return NewRegexTypeScriptParser().extractModulePath(filePath)
}

// extractTypeDeclaration extracts the main type (class, interface, function, type alias)
func (p *TypeScriptParser) extractTypeDeclaration(root *sitter.Node, content []byte, extracted *ExtractedNode) bool {
	// Query for all type declarations
	queryStr := `[
		(class_declaration name: (type_identifier) @class_name)
		(interface_declaration name: (type_identifier) @interface_name)
		(function_declaration name: (identifier) @function_name)
		(abstract_class_declaration name: (type_identifier) @abstract_class_name)
		(type_alias_declaration name: (type_identifier) @type_alias_name)
	]`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
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
		case "abstract_class_declaration":
			extracted.Type = "class"
		case "interface_declaration":
			extracted.Type = "interface"
		case "function_declaration":
			extracted.Type = "function"
		case "type_alias_declaration":
			extracted.Type = "type"
		}

		// Extract heritage clauses (extends/implements)
		p.extractHeritageClauses(typeNode, content, extracted)

		return true
	}

	return false
}

// extractHeritageClauses extracts extends and implements for inheritance
func (p *TypeScriptParser) extractHeritageClauses(typeNode *sitter.Node, content []byte, extracted *ExtractedNode) {
	if typeNode == nil {
		return
	}

	// Look for class_heritage or extends_clause
	for i := 0; i < int(typeNode.ChildCount()); i++ {
		child := typeNode.Child(i)
		switch child.Type() {
		case "class_heritage", "extends_clause", "implements_clause":
			p.processHeritageClause(child, content, extracted)
		}
	}
}

// processHeritageClause processes a heritage clause node
func (p *TypeScriptParser) processHeritageClause(heritageNode *sitter.Node, content []byte, extracted *ExtractedNode) {
	clauseType := "implements" // default

	for i := 0; i < int(heritageNode.ChildCount()); i++ {
		child := heritageNode.Child(i)

		// Check for extends keyword
		if child.Type() == "extends" {
			clauseType = "extends"
			continue
		}

		// Check for implements keyword
		if child.Type() == "implements" {
			clauseType = "implements"
			continue
		}

		// Skip punctuation
		if child.Type() == "," {
			continue
		}

		// Extract type name
		typeName := child.Content(content)
		typeName = strings.TrimSpace(typeName)

		// Remove generics
		if idx := strings.Index(typeName, "<"); idx > 0 {
			typeName = typeName[:idx]
		}

		if typeName != "" && typeName != "extends" && typeName != "implements" {
			dep := ExtractedDependency{
				Target:    typeName,
				Injection: clauseType,
			}
			extracted.Dependencies = append(extracted.Dependencies, dep)
		}
	}
}

// extractDecorators extracts TypeScript decorators
func (p *TypeScriptParser) extractDecorators(root *sitter.Node, content []byte) []string {
	var decorators []string
	seen := make(map[string]bool)

	queryStr := `[
		(decorator (call_expression function: (identifier) @decorator_name))
		(decorator (identifier) @decorator_name)
	]`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
	if err != nil {
		return decorators
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
			decoratorName := capture.Node.Content(content)
			if !seen[decoratorName] {
				seen[decoratorName] = true
				decorators = append(decorators, decoratorName)
			}
		}
	}

	return decorators
}

// extractMembers extracts all members (methods, properties, constructors)
func (p *TypeScriptParser) extractMembers(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	// Extract constructors
	p.extractConstructors(root, content, extracted)

	// Extract methods (including abstract methods)
	p.extractMethods(root, content, extracted)

	// Extract properties
	p.extractProperties(root, content, extracted)

	// Extract getters and setters
	p.extractAccessors(root, content, extracted)
}

// extractConstructors extracts constructor declarations
func (p *TypeScriptParser) extractConstructors(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `(constructor_signature
		(parameters: (formal_parameters) @params
		body: (_)? @body)
	) @ctor`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
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

		var params string
		var ctorNode *sitter.Node

		for _, capture := range match.Captures {
			switch capture.Node.Type() {
			case "formal_parameters":
				params = capture.Node.Content(content)
			case "constructor_signature":
				ctorNode = capture.Node
			}
		}

		if ctorNode == nil {
			continue
		}

		// Get modifiers and documentation
		modifiers := p.getModifiers(ctorNode, content)
		docComment := p.getJsDocComment(ctorNode, content)

		// Skip private/protected constructors unless they're the only ones
		isPublic := !modifiers["private"] && !modifiers["protected"]

		ctor := ExtractedConstructor{
			Signature:   "constructor" + params,
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
func (p *TypeScriptParser) extractMethods(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	// Query for methods, abstract methods, and method signatures (in interfaces)
	queryStr := `[
		(method_definition
			name: (property_identifier) @method_name
			parameters: (formal_parameters) @params
		) @method
		(abstract_method_signature
			name: (property_identifier) @method_name
			parameters: (formal_parameters) @params
		) @abstract_method
		(method_signature
			name: (property_identifier) @method_name
			parameters: (formal_parameters) @params
		) @interface_method
		(public_field_definition
			name: (property_identifier) @arrow_method_name
			value: (arrow_function)
		) @arrow_method
	]`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
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

		var methodName, params string
		var methodNode *sitter.Node
		isArrow := false

		for _, capture := range match.Captures {
			node := capture.Node
			switch capture.Node.Type() {
			case "property_identifier":
				methodName = node.Content(content)
			case "formal_parameters":
				params = node.Content(content)
			case "method_definition", "abstract_method_signature",
				"method_signature", "public_field_definition":
				methodNode = node
				if node.Type() == "public_field_definition" {
					isArrow = true
				}
			}
		}

		if methodNode == nil || methodName == "" {
			continue
		}

		// Get modifiers and documentation
		modifiers := p.getModifiers(methodNode, content)
		docComment := p.getJsDocComment(methodNode, content)

		isPublic := modifiers["public"] || (!modifiers["private"] && !modifiers["protected"])
		isStatic := modifiers["static"]
		isAsync := modifiers["async"]
		isAbstract := modifiers["abstract"] || methodNode.Type() == "abstract_method_signature"

		// Determine access modifier
		accessModifier := "public"
		if modifiers["private"] {
			accessModifier = "private"
		} else if modifiers["protected"] {
			accessModifier = "protected"
		}

		// Get return type
		returnType := p.getMethodReturnType(methodNode, content)

		// Build signature
		signature := p.buildMethodSignature(methodName, params, returnType, accessModifier, isStatic, isAsync, isAbstract)

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
		}

		// Handle arrow methods
		if isArrow {
			// For arrow functions, check if they have async modifier
			if p.isArrowFunctionAsync(methodNode, content) {
				method.Async = true
			}
		}

		extracted.Methods = append(extracted.Methods, method)
	}
}

// isArrowFunctionAsync checks if an arrow function is async
func (p *TypeScriptParser) isArrowFunctionAsync(node *sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	// Look for arrow_function child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "arrow_function" {
			// Check for async keyword
			for j := 0; j < int(child.ChildCount()); j++ {
				grandChild := child.Child(j)
				if grandChild.Type() == "async" {
					return true
				}
			}
		}
	}

	return false
}

// extractProperties extracts property definitions
func (p *TypeScriptParser) extractProperties(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `[
		(public_field_definition
			name: (property_identifier) @prop_name
		) @property
		(property_signature
			name: (property_identifier) @prop_name
		) @interface_property
	]`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
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

		var propName string
		var propNode *sitter.Node

		for _, capture := range match.Captures {
			node := capture.Node
			switch node.Type() {
			case "public_field_definition", "property_signature":
				propNode = node
			case "property_identifier":
				propName = node.Content(content)
			}
		}

		if propNode == nil || propName == "" {
			continue
		}

		// Skip if this is an arrow function method (handled in extractMethods)
		if p.isArrowFunctionProperty(propNode) {
			continue
		}

		// Get modifiers and documentation
		modifiers := p.getModifiers(propNode, content)
		docComment := p.getJsDocComment(propNode, content)

		isPublic := modifiers["public"] || (!modifiers["private"] && !modifiers["protected"])
		isReadonly := modifiers["readonly"]
		isStatic := modifiers["static"]

		// Get type
		propType := p.getPropertyType(propNode, content)

		// Check for optional marker
		isOptional := p.isPropertyOptional(propNode)
		if isOptional {
			propType = propType + " (optional)"
		}

		access := "get; set"
		if isReadonly {
			access = "get"
		}

		// Skip static properties (usually utility)
		if isStatic && !isPublic {
			continue
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

// isArrowFunctionProperty checks if a public_field_definition is an arrow function
func (p *TypeScriptParser) isArrowFunctionProperty(node *sitter.Node) bool {
	if node == nil || node.Type() != "public_field_definition" {
		return false
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "arrow_function" {
			return true
		}
	}

	return false
}

// isPropertyOptional checks if a property has the optional marker
func (p *TypeScriptParser) isPropertyOptional(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	// Look for "?" token
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "?" {
			return true
		}
	}

	return false
}

// extractAccessors extracts getter and setter declarations
func (p *TypeScriptParser) extractAccessors(root *sitter.Node, content []byte, extracted *ExtractedNode) {
	queryStr := `[
		(getter_signature
			name: (property_identifier) @getter_name
		) @getter
		(setter_signature
			name: (property_identifier) @setter_name
		) @setter
	]`

	query, err := sitter.NewQuery(p.tsLang, queryStr)
	if err != nil {
		return
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)

	getters := make(map[string]*ExtractedProperty)
	setters := make(map[string]*ExtractedProperty)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var propName string
		var accessorNode *sitter.Node
		isGetter := false

		for _, capture := range match.Captures {
			node := capture.Node
			switch node.Type() {
			case "getter_signature":
				accessorNode = node
				isGetter = true
			case "setter_signature":
				accessorNode = node
				isGetter = false
			case "property_identifier":
				propName = node.Content(content)
			}
		}

		if accessorNode == nil || propName == "" {
			continue
		}

		// Get modifiers and type
		modifiers := p.getModifiers(accessorNode, content)
		isPublic := modifiers["public"] || (!modifiers["private"] && !modifiers["protected"])
		propType := p.getAccessorType(accessorNode, content, isGetter)

		prop := &ExtractedProperty{
			Name:     propName,
			Type:     propType,
			IsPublic: isPublic,
		}

		if isGetter {
			getters[propName] = prop
		} else {
			setters[propName] = prop
		}
	}

	// Merge getters and setters
	for name, getter := range getters {
		if setter, ok := setters[name]; ok {
			getter.Access = "get; set"
			if setter.Type != "" {
				getter.Type = setter.Type
			}
			delete(setters, name)
		} else {
			getter.Access = "get"
		}
		extracted.Properties = append(extracted.Properties, *getter)
	}

	// Add remaining setters
	for name, setter := range setters {
		setter.Access = "set"
		setter.Name = name
		extracted.Properties = append(extracted.Properties, *setter)
	}
}

// getModifiers extracts access and other modifiers from a declaration node
func (p *TypeScriptParser) getModifiers(node *sitter.Node, content []byte) map[string]bool {
	modifiers := make(map[string]bool)

	if node == nil {
		return modifiers
	}

	// Look for access_modifier and other modifiers
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()
		childContent := strings.ToLower(child.Content(content))

		switch childType {
		case "access_modifier", "public", "private", "protected":
			modifiers[childContent] = true
		case "readonly":
			modifiers["readonly"] = true
		case "static":
			modifiers["static"] = true
		case "async":
			modifiers["async"] = true
		case "abstract":
			modifiers["abstract"] = true
		case "override":
			modifiers["override"] = true
		}
	}

	return modifiers
}

// getJsDocComment extracts JSDoc comment for a node
func (p *TypeScriptParser) getJsDocComment(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Look for leading comment
	// Get the node's start position
	startPoint := node.StartPoint()

	// Extract lines before the node
	lines := strings.Split(string(content), "\n")
	if startPoint.Row == 0 {
		return ""
	}

	var docLines []string
	inComment := false

	// Look backwards from the line before the node
	for i := int(startPoint.Row) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "*/") {
			inComment = true
			continue
		}

		if inComment {
			if strings.HasPrefix(line, "/*") {
				break
			}
			// Remove leading *
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "@") {
				docLines = append([]string{line}, docLines...)
			}
		} else if strings.HasPrefix(line, "//") {
			// Single line comment
			comment := strings.TrimPrefix(line, "//")
			comment = strings.TrimSpace(comment)
			if comment != "" {
				docLines = append([]string{comment}, docLines...)
			}
		} else if line != "" {
			// Non-empty, non-comment line - stop looking
			break
		}
	}

	return strings.Join(docLines, " ")
}

// getMethodReturnType extracts the return type from a method node
func (p *TypeScriptParser) getMethodReturnType(methodNode *sitter.Node, content []byte) string {
	if methodNode == nil {
		return ""
	}

	// Look for type_annotation
	for i := 0; i < int(methodNode.ChildCount()); i++ {
		child := methodNode.Child(i)
		if child.Type() == "type_annotation" {
			// Skip the ":" and get the type
			for j := 0; j < int(child.ChildCount()); j++ {
				typeChild := child.Child(j)
				if typeChild.Type() != ":" {
					return typeChild.Content(content)
				}
			}
		}
		// Also check for return type in predefined types
		if child.Type() == "predefined_type" || child.Type() == "type_identifier" ||
			child.Type() == "union_type" || child.Type() == "intersection_type" {
			return child.Content(content)
		}
	}

	return ""
}

// getPropertyType extracts the type from a property node
func (p *TypeScriptParser) getPropertyType(propNode *sitter.Node, content []byte) string {
	if propNode == nil {
		return ""
	}

	// Look for type_annotation
	for i := 0; i < int(propNode.ChildCount()); i++ {
		child := propNode.Child(i)
		if child.Type() == "type_annotation" {
			// Skip the ":" and get the type
			for j := 0; j < int(child.ChildCount()); j++ {
				typeChild := child.Child(j)
				if typeChild.Type() != ":" {
					return typeChild.Content(content)
				}
			}
		}
		// Also check for type directly
		if child.Type() == "predefined_type" || child.Type() == "type_identifier" ||
			child.Type() == "union_type" || child.Type() == "intersection_type" ||
			child.Type() == "array_type" || child.Type() == "generic_type" {
			return child.Content(content)
		}
	}

	// Try to infer from initializer
	for i := 0; i < int(propNode.ChildCount()); i++ {
		child := propNode.Child(i)
		if child.Type() == "=" {
			// Look for the value
			if i+1 < int(propNode.ChildCount()) {
				value := propNode.Child(i + 1)
				return p.inferTypeFromValue(value, content)
			}
		}
	}

	return ""
}

// getAccessorType extracts the type from a getter/setter node
func (p *TypeScriptParser) getAccessorType(accessorNode *sitter.Node, content []byte, isGetter bool) string {
	if accessorNode == nil {
		return ""
	}

	if isGetter {
		// Getter returns a type
		for i := 0; i < int(accessorNode.ChildCount()); i++ {
			child := accessorNode.Child(i)
			if child.Type() == "type_annotation" {
				for j := 0; j < int(child.ChildCount()); j++ {
					typeChild := child.Child(j)
					if typeChild.Type() != ":" {
						return typeChild.Content(content)
					}
				}
			}
		}
	} else {
		// Setter has parameter with type
		for i := 0; i < int(accessorNode.ChildCount()); i++ {
			child := accessorNode.Child(i)
			if child.Type() == "formal_parameters" {
				// Get the first parameter's type
				for j := 0; j < int(child.ChildCount()); j++ {
					paramChild := child.Child(j)
					if paramChild.Type() == "required_parameter" || paramChild.Type() == "optional_parameter" {
						return p.getParameterType(paramChild, content)
					}
				}
			}
		}
	}

	return ""
}

// inferTypeFromValue attempts to infer type from an initializer value
func (p *TypeScriptParser) inferTypeFromValue(valueNode *sitter.Node, content []byte) string {
	if valueNode == nil {
		return ""
	}

	switch valueNode.Type() {
	case "string":
		return "string"
	case "number":
		return "number"
	case "true", "false":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	case "null":
		return "null"
	case "undefined":
		return "undefined"
	case "arrow_function":
		return "function"
	case "call_expression":
		// Try to infer from the called function
		return "unknown"
	default:
		return ""
	}
}

// getParameterType extracts type from a parameter node
func (p *TypeScriptParser) getParameterType(paramNode *sitter.Node, content []byte) string {
	if paramNode == nil {
		return ""
	}

	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		if child.Type() == "type_annotation" {
			for j := 0; j < int(child.ChildCount()); j++ {
				typeChild := child.Child(j)
				if typeChild.Type() != ":" {
					return typeChild.Content(content)
				}
			}
		}
	}

	return ""
}

// parseParametersFromNode extracts parameters from a method/constructor node
func (p *TypeScriptParser) parseParametersFromNode(node *sitter.Node, content []byte) []ExtractedParameter {
	var params []ExtractedParameter

	if node == nil {
		return params
	}

	// Find formal_parameters
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "formal_parameters" {
			// Parse each parameter
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "required_parameter" ||
					paramChild.Type() == "optional_parameter" ||
					paramChild.Type() == "rest_parameter" {
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
func (p *TypeScriptParser) parseParameter(paramNode *sitter.Node, content []byte) ExtractedParameter {
	param := ExtractedParameter{}

	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		switch child.Type() {
		case "identifier":
			param.Name = child.Content(content)
		case "type_annotation":
			for j := 0; j < int(child.ChildCount()); j++ {
				typeChild := child.Child(j)
				if typeChild.Type() != ":" {
					param.Type = typeChild.Content(content)
				}
			}
		case "predefined_type", "type_identifier", "union_type", "intersection_type":
			param.Type = child.Content(content)
		case "...":
			// Rest parameter
			if i+1 < int(paramNode.ChildCount()) {
				nextChild := paramNode.Child(i + 1)
				if nextChild.Type() == "identifier" {
					param.Name = "..." + nextChild.Content(content)
				}
			}
		}
	}

	return param
}

// buildMethodSignature constructs a method signature string
func (p *TypeScriptParser) buildMethodSignature(name, params, returnType, access string, isStatic, isAsync, isAbstract bool) string {
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

	parts = append(parts, name+params)

	if returnType != "" {
		parts = append(parts, ":"+returnType)
	}

	return strings.Join(parts, " ")
}

// extractDependenciesFromConstructor extracts dependencies from constructor parameters
func (p *TypeScriptParser) extractDependenciesFromConstructor(ctor ExtractedConstructor, extracted *ExtractedNode) {
	for _, param := range ctor.Parameters {
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
func (p *TypeScriptParser) extractDependencies(extracted *ExtractedNode) {
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

// Ensure TypeScriptParser implements Parser interface
var _ Parser = (*TypeScriptParser)(nil)
