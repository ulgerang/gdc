package main

import (
	"fmt"
	"log"

	"github.com/gdc-tools/gdc/internal/parser"
)

func main() {
	p := parser.NewTypeScriptParser()
	extracted, err := p.ParseFile("fixtures/p1/sample.ts")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("=== TypeScript Parser Results for fixtures/p1/sample.ts ===\n\n")
	fmt.Printf("ID: %s\n", extracted.ID)
	fmt.Printf("Type: %s\n", extracted.Type)
	fmt.Printf("Language: %s\n", extracted.Language)
	fmt.Printf("Module: %s\n", extracted.Module)
	fmt.Printf("Attributes: %v\n", extracted.Attributes)

	fmt.Printf("\n--- Constructors (%d) ---\n", len(extracted.Constructors))
	for i, ctor := range extracted.Constructors {
		fmt.Printf("%d. %s\n", i+1, ctor.Signature)
		fmt.Printf("   Params: %v\n", ctor.Parameters)
	}

	fmt.Printf("\n--- Methods (%d, showing public only) ---\n", len(extracted.Methods))
	count := 0
	for _, method := range extracted.Methods {
		if method.IsPublic {
			count++
			fmt.Printf("%d. %s (async=%v, static=%v)\n", count, method.Name, method.Async, method.Static)
			fmt.Printf("   Signature: %s\n", method.Signature)
			fmt.Printf("   Returns: %s\n", method.Returns)
			fmt.Printf("   Description: %s\n", method.Description)
		}
	}

	fmt.Printf("\n--- Properties (%d, showing public only) ---\n", len(extracted.Properties))
	count = 0
	for _, prop := range extracted.Properties {
		if prop.IsPublic {
			count++
			fmt.Printf("%d. %s: %s (access: %s)\n", count, prop.Name, prop.Type, prop.Access)
			fmt.Printf("   Description: %s\n", prop.Description)
		}
	}

	fmt.Printf("\n--- Dependencies (%d) ---\n", len(extracted.Dependencies))
	for i, dep := range extracted.Dependencies {
		fmt.Printf("%d. %s (via %s)\n", i+1, dep.Target, dep.Injection)
	}
}
