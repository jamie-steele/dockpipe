package reasoning

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type EvidenceExtractor struct{}

func (EvidenceExtractor) Extract(root, relPath string, searchTerms []string) (EvidenceRecord, error) {
	body, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		return EvidenceRecord{}, err
	}
	language := strings.TrimPrefix(strings.ToLower(filepath.Ext(relPath)), ".")
	switch language {
	case "go":
		return extractGoEvidence(relPath, string(body), searchTerms)
	default:
		return extractFallbackEvidence(relPath, string(body), searchTerms), nil
	}
}

type scoredNode struct {
	node  EvidenceNode
	score int
}

func extractGoEvidence(relPath, body string, searchTerms []string) (EvidenceRecord, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, relPath, body, parser.ParseComments)
	if err != nil {
		return extractFallbackEvidence(relPath, body, searchTerms), nil
	}
	record := EvidenceRecord{
		Nodes: []EvidenceNode{{
			ID:       "file:" + relPath,
			Kind:     "file",
			File:     relPath,
			Summary:  summarizeBody(body),
			Language: "go",
		}},
	}
	edges := []EvidenceEdge{}
	nodes := []scoredNode{}
	symbolIDs := map[string]string{}
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.FuncDecl:
			name := item.Name.Name
			if item.Recv != nil && len(item.Recv.List) > 0 {
				name = receiverName(item.Recv.List[0].Type) + "." + name
			}
			start := fset.Position(item.Pos()).Line
			end := fset.Position(item.End()).Line
			node := EvidenceNode{
				ID:        "symbol:" + relPath + ":" + name,
				Kind:      "symbol",
				File:      relPath,
				Symbol:    name,
				Summary:   "Go declaration block.",
				StartLine: start,
				EndLine:   end,
				Language:  "go",
				Tags:      []string{"declaration", "go"},
			}
			nodes = append(nodes, scoredNode{node: node, score: scoreEvidenceNode(node, body, searchTerms)})
			symbolIDs[name] = node.ID
		case *ast.GenDecl:
			for _, spec := range item.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					start := fset.Position(typed.Pos()).Line
					end := fset.Position(typed.End()).Line
					node := EvidenceNode{
						ID:        "symbol:" + relPath + ":" + typed.Name.Name,
						Kind:      "symbol",
						File:      relPath,
						Symbol:    typed.Name.Name,
						Summary:   "Go type declaration block.",
						StartLine: start,
						EndLine:   end,
						Language:  "go",
						Tags:      []string{"declaration", "type", "go"},
					}
					nodes = append(nodes, scoredNode{node: node, score: scoreEvidenceNode(node, body, searchTerms)})
					symbolIDs[typed.Name.Name] = node.ID
				}
			}
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].score == nodes[j].score {
			return nodes[i].node.StartLine < nodes[j].node.StartLine
		}
		return nodes[i].score > nodes[j].score
	})
	limit := len(nodes)
	if limit > 20 {
		limit = 20
	}
	selected := map[string]struct{}{}
	for _, item := range nodes[:limit] {
		record.Nodes = append(record.Nodes, item.node)
		edges = append(edges, EvidenceEdge{From: "file:" + relPath, To: item.node.ID, Kind: "contains"})
		selected[item.node.Symbol] = struct{}{}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			name = receiverName(fn.Recv.List[0].Type) + "." + name
		}
		if _, ok := selected[name]; !ok {
			continue
		}
		fromID := symbolIDs[name]
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := ""
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				callee = fun.Name
			case *ast.SelectorExpr:
				callee = fun.Sel.Name
			}
			toID, exists := symbolIDs[callee]
			if exists && toID != fromID {
				edges = append(edges, EvidenceEdge{From: fromID, To: toID, Kind: "references"})
			}
			return true
		})
	}
	record.Edges = uniqueEvidenceEdges(edges)
	return record, nil
}

var fallbackDeclPattern = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(?:func|function|class|interface|type)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func extractFallbackEvidence(relPath, body string, searchTerms []string) EvidenceRecord {
	record := EvidenceRecord{
		Nodes: []EvidenceNode{{
			ID:      "file:" + relPath,
			Kind:    "file",
			File:    relPath,
			Summary: summarizeBody(body),
		}},
	}
	lines := strings.Split(body, "\n")
	var scored []scoredNode
	for idx, line := range lines {
		match := fallbackDeclPattern.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		start := idx + 1
		end := start + 12
		if end > len(lines) {
			end = len(lines)
		}
		node := EvidenceNode{
			ID:        "symbol:" + relPath + ":" + match[1],
			Kind:      "symbol",
			File:      relPath,
			Symbol:    match[1],
			Summary:   "Declaration-shaped block.",
			StartLine: start,
			EndLine:   end,
			Language:  strings.TrimPrefix(strings.ToLower(filepath.Ext(relPath)), "."),
			Tags:      []string{"fallback"},
		}
		scored = append(scored, scoredNode{node: node, score: scoreEvidenceNode(node, strings.Join(lines[start-1:end], "\n"), searchTerms)})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].node.StartLine < scored[j].node.StartLine
		}
		return scored[i].score > scored[j].score
	})
	limit := len(scored)
	if limit > 6 {
		limit = 6
	}
	for _, item := range scored[:limit] {
		record.Nodes = append(record.Nodes, item.node)
		record.Edges = append(record.Edges, EvidenceEdge{From: "file:" + relPath, To: item.node.ID, Kind: "contains"})
	}
	return record
}

func summarizeBody(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return strings.Join(lines, " ")
}

func receiverName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		if ident, ok := value.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return "receiver"
}

func scoreEvidenceNode(node EvidenceNode, excerpt string, searchTerms []string) int {
	score := 1
	lowerPath := strings.ToLower(node.File)
	lowerSymbol := strings.ToLower(node.Symbol)
	lowerExcerpt := strings.ToLower(excerpt)
	for _, term := range searchTerms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		if strings.Contains(lowerPath, normalized) {
			score += 2
		}
		if strings.Contains(lowerSymbol, normalized) {
			score += 4
		}
		if strings.Contains(lowerExcerpt, normalized) {
			score += 1
		}
	}
	return score
}

func uniqueEvidenceEdges(edges []EvidenceEdge) []EvidenceEdge {
	seen := map[string]struct{}{}
	out := make([]EvidenceEdge, 0, len(edges))
	for _, edge := range edges {
		key := edge.From + "->" + edge.To + ":" + edge.Kind
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}
	return out
}
