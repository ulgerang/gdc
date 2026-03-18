package parser

import "strings"

// NormalizeTypeReference reduces a language type reference to its base symbol.
// Examples:
//
//	ILogger<OrderService> -> ILogger
//	Example.Repositories.IRepository<Order>[] -> Example.Repositories.IRepository
func NormalizeTypeReference(typeRef string) string {
	typeRef = strings.TrimSpace(typeRef)
	if typeRef == "" {
		return ""
	}

	typeRef = strings.TrimPrefix(typeRef, "global::")

	for {
		trimmed := strings.TrimSpace(typeRef)
		switch {
		case strings.HasSuffix(trimmed, "[]"):
			typeRef = strings.TrimSpace(strings.TrimSuffix(trimmed, "[]"))
		case strings.HasSuffix(trimmed, "?"):
			typeRef = strings.TrimSpace(strings.TrimSuffix(trimmed, "?"))
		default:
			typeRef = trimmed
			goto done
		}
	}

done:
	depth := 0
	for i, r := range typeRef {
		switch r {
		case '<':
			if depth == 0 {
				return strings.TrimSpace(typeRef[:i])
			}
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		}
	}

	return strings.TrimSpace(typeRef)
}
