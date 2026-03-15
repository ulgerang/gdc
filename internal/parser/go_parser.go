package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GoParser parses Go source files.
type GoParser struct{}

type goDependencyContext struct {
	packageName    string
	modulePath     string
	imports        map[string]string
	localTypeKinds map[string]string
}

func (c goDependencyContext) currentPackage() string {
	return c.packageName
}

// NewGoParser creates a new Go parser.
func NewGoParser() *GoParser {
	return &GoParser{}
}

// Language returns "go".
func (p *GoParser) Language() string {
	return "go"
}

// ParseFile parses a Go source file and returns the first extracted node for
// backward compatibility.
func (p *GoParser) ParseFile(filePath string) (*ExtractedNode, error) {
	pkgName, nodes, err := p.parseFileNodes(filePath)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return &ExtractedNode{
			FilePath:  filePath,
			Namespace: pkgName,
			Language:  "go",
			Package:   pkgName,
		}, nil
	}
	return nodes[0], nil
}

// ParseFileNodes parses a Go source file and extracts all named nodes.
func (p *GoParser) ParseFileNodes(filePath string) ([]*ExtractedNode, error) {
	_, nodes, err := p.parseFileNodes(filePath)
	return nodes, err
}

func (p *GoParser) parseFileNodes(filePath string) (string, []*ExtractedNode, error) {
	fset := token.NewFileSet()

	file, err := goparser.ParseFile(fset, filePath, nil, goparser.ParseComments)
	if err != nil {
		return "", nil, err
	}

	pkgName := file.Name.Name
	nodes := make([]*ExtractedNode, 0)
	nodeByID := make(map[string]*ExtractedNode)
	localTypeKinds := make(map[string]string)
	structTypes := make(map[string]*ast.StructType)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			docComment := typeDocComment(genDecl, typeSpec)
			var extracted *ExtractedNode

			switch t := typeSpec.Type.(type) {
			case *ast.StructType:
				extracted = &ExtractedNode{
					ID:        typeSpec.Name.Name,
					Type:      "class",
					Namespace: pkgName,
					Language:  "go",
					Package:   pkgName,
					FilePath:  filePath,
				}
				localTypeKinds[typeSpec.Name.Name] = "class"
				structTypes[typeSpec.Name.Name] = t
			case *ast.InterfaceType:
				extracted = &ExtractedNode{
					ID:        typeSpec.Name.Name,
					Type:      "interface",
					Namespace: pkgName,
					Language:  "go",
					Package:   pkgName,
					FilePath:  filePath,
				}
				localTypeKinds[typeSpec.Name.Name] = "interface"
				p.extractInterfaceMethods(t, extracted)
			case *ast.FuncType:
				extracted = &ExtractedNode{
					ID:        typeSpec.Name.Name,
					Type:      "interface",
					Namespace: pkgName,
					Language:  "go",
					Package:   pkgName,
					FilePath:  filePath,
				}
				localTypeKinds[typeSpec.Name.Name] = "func"
				p.extractCallableType(typeSpec.Name.Name, t, extracted, docComment)
			default:
				localTypeKinds[typeSpec.Name.Name] = "other"
			}

			if extracted == nil {
				continue
			}

			nodes = append(nodes, extracted)
			nodeByID[extracted.ID] = extracted
		}
	}

	depCtx := goDependencyContext{
		packageName:    pkgName,
		modulePath:     findGoModulePath(filePath),
		imports:        buildImportMap(file),
		localTypeKinds: localTypeKinds,
	}

	for typeName, structType := range structTypes {
		if extracted := nodeByID[typeName]; extracted != nil {
			p.extractStructFields(structType, extracted, depCtx)
		}
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			recvType := namedTypeFromExpr(funcDecl.Recv.List[0].Type)
			node := nodeByID[recvType]
			if node == nil {
				continue
			}
			p.extractMethod(funcDecl, node, depCtx)
			continue
		}

		if !strings.HasPrefix(funcDecl.Name.Name, "New") {
			functionNode := &ExtractedNode{
				ID:        funcDecl.Name.Name,
				Type:      "function",
				Namespace: pkgName,
				Language:  "go",
				Package:   pkgName,
				FilePath:  filePath,
			}
			localTypeKinds[functionNode.ID] = "function"
			p.extractTopLevelFunction(funcDecl, functionNode, depCtx)
			nodes = append(nodes, functionNode)
			nodeByID[functionNode.ID] = functionNode
			continue
		}

		ownerID := constructorTarget(funcDecl.Type.Results)
		node := nodeByID[ownerID]
		if node == nil {
			functionNode := &ExtractedNode{
				ID:        funcDecl.Name.Name,
				Type:      "function",
				Namespace: pkgName,
				Language:  "go",
				Package:   pkgName,
				FilePath:  filePath,
			}
			localTypeKinds[functionNode.ID] = "function"
			p.extractTopLevelFunction(funcDecl, functionNode, depCtx)
			nodes = append(nodes, functionNode)
			nodeByID[functionNode.ID] = functionNode
			continue
		}

		ctor := ExtractedConstructor{
			Signature:  p.buildMethodSignature(funcDecl.Name.Name, funcDecl.Type),
			Parameters: p.extractParameters(funcDecl.Type.Params),
		}
		if funcDecl.Doc != nil {
			ctor.Description = strings.TrimSpace(funcDecl.Doc.Text())
		}

		node.Constructors = append(node.Constructors, ctor)
		node.Dependencies = mergeDependencies(node.Dependencies, p.extractDependenciesFromFieldList(funcDecl.Type.Params, depCtx, ownerID, "constructor"))
	}

	return pkgName, nodes, nil
}

func (p *GoParser) extractTopLevelFunction(decl *ast.FuncDecl, node *ExtractedNode, depCtx goDependencyContext) {
	if decl == nil || node == nil {
		return
	}

	isExported := ast.IsExported(decl.Name.Name)
	method := ExtractedMethod{
		Name:       decl.Name.Name,
		Signature:  p.buildMethodSignature(decl.Name.Name, decl.Type),
		Parameters: p.extractParameters(decl.Type.Params),
		Returns:    p.extractReturnType(decl.Type.Results),
		IsPublic:   isExported,
		Exported:   isExported,
	}
	if decl.Doc != nil {
		method.Description = strings.TrimSpace(decl.Doc.Text())
	}
	node.Methods = append(node.Methods, method)
	node.Dependencies = mergeDependencies(node.Dependencies, p.extractDependenciesFromFieldList(decl.Type.Params, depCtx, node.ID, "method"))
	node.Dependencies = mergeDependencies(node.Dependencies, p.extractDependenciesFromFieldList(decl.Type.Results, depCtx, node.ID, "method"))
}

func (p *GoParser) extractStructFields(st *ast.StructType, node *ExtractedNode, depCtx goDependencyContext) {
	if st.Fields == nil {
		return
	}

	for _, field := range st.Fields.List {
		if dep := classifyGoFieldDependency(field.Type, depCtx, node.ID); dep != nil {
			if len(field.Names) == 0 {
				node.Dependencies = mergeDependencies(node.Dependencies, []ExtractedDependency{*dep})
			} else {
				deps := make([]ExtractedDependency, 0, len(field.Names))
				for _, name := range field.Names {
					fieldDep := *dep
					fieldDep.FieldName = name.Name
					deps = append(deps, fieldDep)
				}
				node.Dependencies = mergeDependencies(node.Dependencies, deps)
			}
		}

		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			if !ast.IsExported(name.Name) {
				continue
			}

			prop := ExtractedProperty{
				Name:     name.Name,
				Type:     exprToString(field.Type),
				Access:   "get; set",
				IsPublic: true,
			}

			if field.Doc != nil {
				prop.Description = strings.TrimSpace(field.Doc.Text())
			} else if field.Comment != nil {
				prop.Description = strings.TrimSpace(field.Comment.Text())
			}

			node.Properties = append(node.Properties, prop)
		}
	}
}

func (p *GoParser) extractInterfaceMethods(iface *ast.InterfaceType, node *ExtractedNode) {
	if iface.Methods == nil {
		return
	}

	for _, method := range iface.Methods.List {
		if len(method.Names) == 0 {
			continue
		}

		funcType, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}

		for _, name := range method.Names {
			isExported := ast.IsExported(name.Name)
			extracted := ExtractedMethod{
				Name:       name.Name,
				Signature:  p.buildMethodSignature(name.Name, funcType),
				Parameters: p.extractParameters(funcType.Params),
				Returns:    p.extractReturnType(funcType.Results),
				IsPublic:   isExported,
				Exported:   isExported,
			}

			if method.Doc != nil {
				extracted.Description = strings.TrimSpace(method.Doc.Text())
			} else if method.Comment != nil {
				extracted.Description = strings.TrimSpace(method.Comment.Text())
			}

			node.Methods = append(node.Methods, extracted)
		}
	}
}

func (p *GoParser) extractCallableType(typeName string, funcType *ast.FuncType, node *ExtractedNode, docComment string) {
	isExported := ast.IsExported(typeName)
	node.Methods = append(node.Methods, ExtractedMethod{
		Name:        "Invoke",
		Signature:   p.buildMethodSignature("Invoke", funcType),
		Parameters:  p.extractParameters(funcType.Params),
		Returns:     p.extractReturnType(funcType.Results),
		Description: docComment,
		IsPublic:    isExported,
		Exported:    isExported,
	})
}

func (p *GoParser) extractMethod(decl *ast.FuncDecl, node *ExtractedNode, depCtx goDependencyContext) {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return
	}

	recvType := namedTypeFromExpr(decl.Recv.List[0].Type)
	if recvType != node.ID && node.ID != "" {
		return
	}

	isExported := ast.IsExported(decl.Name.Name)
	extracted := ExtractedMethod{
		Name:       decl.Name.Name,
		Signature:  p.buildMethodSignature(decl.Name.Name, decl.Type),
		Parameters: p.extractParameters(decl.Type.Params),
		Returns:    p.extractReturnType(decl.Type.Results),
		IsPublic:   isExported,
		Exported:   isExported,
	}

	if decl.Doc != nil {
		extracted.Description = strings.TrimSpace(decl.Doc.Text())
	}

	node.Methods = append(node.Methods, extracted)
	node.Dependencies = mergeDependencies(node.Dependencies, p.extractDependenciesFromFieldList(decl.Type.Params, depCtx, node.ID, "method"))
	node.Dependencies = mergeDependencies(node.Dependencies, p.extractDependenciesFromFieldList(decl.Type.Results, depCtx, node.ID, "method"))
}

func (p *GoParser) extractDependenciesFromFieldList(fields *ast.FieldList, ctx goDependencyContext, ownerID, injection string) []ExtractedDependency {
	if fields == nil {
		return nil
	}

	deps := make([]ExtractedDependency, 0)
	for _, field := range fields.List {
		target, depType, depNamespace, ok := classifyGoDependency(field.Type, ctx)
		if !ok {
			continue
		}

		if target == ownerID {
			continue
		}

		if len(field.Names) == 0 {
			deps = append(deps, ExtractedDependency{
				Target:    target,
				Type:      depType,
				Namespace: depNamespace,
				Injection: injection,
			})
			continue
		}

		for _, name := range field.Names {
			deps = append(deps, ExtractedDependency{
				Target:    target,
				Type:      depType,
				Namespace: depNamespace,
				FieldName: name.Name,
				Injection: injection,
			})
		}
	}

	return deps
}

func classifyGoFieldDependency(expr ast.Expr, ctx goDependencyContext, ownerID string) *ExtractedDependency {
	target, depType, depNamespace, ok := classifyGoDependency(expr, ctx)
	if !ok {
		typeName := namedTypeFromExpr(expr)
		if typeName == "" || typeName == ownerID {
			return nil
		}
		switch ctx.localTypeKinds[typeName] {
		case "interface":
			target, depType, depNamespace, ok = typeName, "interface", ctx.currentPackage(), true
		case "class":
			target, depType, depNamespace, ok = typeName, "class", ctx.currentPackage(), true
		default:
			return nil
		}
	}
	if !ok || target == ownerID {
		return nil
	}
	return &ExtractedDependency{
		Target:    target,
		Type:      depType,
		Namespace: depNamespace,
		Injection: "field",
	}
}

func (p *GoParser) buildMethodSignature(name string, funcType *ast.FuncType) string {
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteString("(")

	if funcType.Params != nil {
		params := make([]string, 0)
		for _, param := range funcType.Params.List {
			typeStr := exprToString(param.Type)
			for _, pname := range param.Names {
				params = append(params, pname.Name+" "+typeStr)
			}
			if len(param.Names) == 0 {
				params = append(params, typeStr)
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}

	sb.WriteString(")")

	if funcType.Results != nil && len(funcType.Results.List) > 0 {
		sb.WriteString(" ")
		if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
			sb.WriteString(exprToString(funcType.Results.List[0].Type))
		} else {
			sb.WriteString("(")
			results := make([]string, 0)
			for _, result := range funcType.Results.List {
				typeStr := exprToString(result.Type)
				for _, rname := range result.Names {
					results = append(results, rname.Name+" "+typeStr)
				}
				if len(result.Names) == 0 {
					results = append(results, typeStr)
				}
			}
			sb.WriteString(strings.Join(results, ", "))
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func (p *GoParser) extractParameters(params *ast.FieldList) []ExtractedParameter {
	var result []ExtractedParameter
	if params == nil {
		return result
	}

	for _, param := range params.List {
		typeStr := exprToString(param.Type)
		for _, name := range param.Names {
			result = append(result, ExtractedParameter{
				Name: name.Name,
				Type: typeStr,
			})
		}
	}

	return result
}

func (p *GoParser) extractReturnType(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}

	if len(results.List) == 1 {
		return exprToString(results.List[0].Type)
	}

	types := make([]string, 0, len(results.List))
	for _, result := range results.List {
		types = append(types, exprToString(result.Type))
	}
	return "(" + strings.Join(types, ", ") + ")"
}

// exprToString converts an AST expression to a string representation.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprToString(e.Value)
	default:
		return "unknown"
	}
}

// ParseDirectory parses all Go files in a directory.
func (p *GoParser) ParseDirectory(dirPath string) ([]*ExtractedNode, error) {
	fset := token.NewFileSet()
	pkgs, err := goparser.ParseDir(fset, dirPath, nil, goparser.ParseComments)
	if err != nil {
		return nil, err
	}

	nodes := make([]*ExtractedNode, 0)
	for _, pkg := range pkgs {
		for filePath := range pkg.Files {
			if strings.HasSuffix(filepath.Base(filePath), "_test.go") {
				continue
			}

			extractedNodes, err := p.ParseFileNodes(filePath)
			if err != nil {
				continue
			}
			nodes = append(nodes, extractedNodes...)
		}
	}

	return nodes, nil
}

func typeDocComment(genDecl *ast.GenDecl, typeSpec *ast.TypeSpec) string {
	if typeSpec.Doc != nil {
		return strings.TrimSpace(typeSpec.Doc.Text())
	}
	if genDecl.Doc != nil {
		return strings.TrimSpace(genDecl.Doc.Text())
	}
	return ""
}

func constructorTarget(results *ast.FieldList) string {
	if results == nil {
		return ""
	}
	for _, result := range results.List {
		if target := namedTypeFromExpr(result.Type); target != "" {
			return target
		}
	}
	return ""
}

func namedTypeFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return namedTypeFromExpr(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.ArrayType:
		return namedTypeFromExpr(e.Elt)
	case *ast.IndexExpr:
		return namedTypeFromExpr(e.X)
	case *ast.IndexListExpr:
		return namedTypeFromExpr(e.X)
	default:
		return ""
	}
}

func mergeDependencies(existing, incoming []ExtractedDependency) []ExtractedDependency {
	if len(incoming) == 0 {
		return existing
	}

	seen := make(map[string]bool, len(existing))
	for _, dep := range existing {
		seen[dependencyIdentity(dep)] = true
	}

	for _, dep := range incoming {
		identity := dependencyIdentity(dep)
		if dep.Target == "" || seen[identity] {
			continue
		}
		existing = append(existing, dep)
		seen[identity] = true
	}

	return existing
}

func dependencyIdentity(dep ExtractedDependency) string {
	return dep.Namespace + "::" + dep.Target
}

func classifyGoDependency(expr ast.Expr, ctx goDependencyContext) (string, string, string, bool) {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return classifyGoDependency(e.X, ctx)
	case *ast.SelectorExpr:
		pkgIdent, ok := e.X.(*ast.Ident)
		if !ok {
			return "", "", "", false
		}
		importPath := ctx.imports[pkgIdent.Name]
		if importPath == "" || !isLocalGoImport(importPath, ctx.modulePath) {
			return "", "", "", false
		}
		return classifyNamedDependency(e.Sel.Name, importPath, ctx)
	case *ast.Ident:
		return classifyNamedDependency(e.Name, "", ctx)
	default:
		return "", "", "", false
	}
}

func classifyNamedDependency(typeName, importPath string, ctx goDependencyContext) (string, string, string, bool) {
	if typeName == "" || isPredeclaredGoType(typeName) {
		return "", "", "", false
	}

	namespace := ctx.currentPackage()
	if importPath != "" {
		namespace = importAlias(importPath)
	}

	if importPath == "" {
		kind := ctx.localTypeKinds[typeName]
		switch kind {
		case "interface":
			return typeName, "interface", namespace, true
		case "class":
			if isConfigLikeName(typeName) {
				return "", "", "", false
			}
			return typeName, "class", namespace, true
		case "func", "other":
			return "", "", "", false
		}
	}

	if !looksLikeDependencyName(typeName) || isConfigLikeName(typeName) {
		return "", "", "", false
	}

	if importPath != "" && !isLocalGoImport(importPath, ctx.modulePath) {
		return "", "", "", false
	}

	return typeName, "interface", namespace, true
}

func looksLikeDependencyName(typeName string) bool {
	patterns := []string{
		"Adapter",
		"Cache",
		"Client",
		"Closer",
		"Executor",
		"Factory",
		"Handler",
		"Interactor",
		"Logger",
		"Mailbox",
		"Manager",
		"Monitor",
		"Provider",
		"Queue",
		"Reader",
		"Registry",
		"Repository",
		"Resolver",
		"Runner",
		"Service",
		"Store",
		"Tool",
		"Validator",
		"Writer",
	}

	for _, pattern := range patterns {
		if strings.HasSuffix(typeName, pattern) {
			return true
		}
	}

	if strings.HasPrefix(typeName, "I") && len(typeName) > 1 {
		second := typeName[1]
		return second >= 'A' && second <= 'Z'
	}

	return false
}

func isConfigLikeName(typeName string) bool {
	patterns := []string{
		"Args",
		"Config",
		"Entry",
		"Event",
		"Info",
		"Input",
		"Metadata",
		"Options",
		"Output",
		"Params",
		"Record",
		"Request",
		"Response",
		"Result",
		"Snapshot",
		"State",
	}

	for _, pattern := range patterns {
		if strings.HasSuffix(typeName, pattern) {
			return true
		}
	}

	return false
}

func isPredeclaredGoType(typeName string) bool {
	predeclared := map[string]bool{
		"any":         true,
		"bool":        true,
		"byte":        true,
		"comparable":  true,
		"complex128":  true,
		"complex64":   true,
		"error":       true,
		"float32":     true,
		"float64":     true,
		"int":         true,
		"int16":       true,
		"int32":       true,
		"int64":       true,
		"int8":        true,
		"rune":        true,
		"string":      true,
		"uint":        true,
		"uint16":      true,
		"uint32":      true,
		"uint64":      true,
		"uint8":       true,
		"uintptr":     true,
		"interface{}": true,
	}
	return predeclared[typeName]
}

func buildImportMap(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, imp := range file.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		alias := importAlias(pathValue)
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			alias = imp.Name.Name
		}

		imports[alias] = pathValue
	}
	return imports
}

func importAlias(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) == 0 {
		return importPath
	}
	return parts[len(parts)-1]
}

func isLocalGoImport(importPath, modulePath string) bool {
	if importPath == "" || modulePath == "" {
		return false
	}
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func findGoModulePath(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(goModPath); err == nil {
			modulePath := parseGoModulePath(string(data))
			if modulePath != "" {
				return modulePath
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func parseGoModulePath(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "module "))
		}
	}
	return ""
}
