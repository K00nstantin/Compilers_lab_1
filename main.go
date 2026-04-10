package main

import (
	"fmt"
	"os"
)

func main() {
	reStr := os.Args[1]
	ast, err := parseRegex(reStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка разбора: %v\n", err)
		os.Exit(1)
	}
	PrintASTLine(ast)
}

func literalLabel(ch rune) string {
	switch ch {
	case 0:
		return "ε"
	case -1:
		return "#"
	default:
		return fmt.Sprintf("%q", ch)
	}
}

func ASTLine(n *reNode) string {
	if n == nil {
		return "nil"
	}
	switch n.kind {
	case 'l':
		return literalLabel(n.ch)
	case 'a':
		return "(| " + ASTLine(n.a) + " " + ASTLine(n.b) + ")"
	case 'c':
		return "(. " + ASTLine(n.a) + " " + ASTLine(n.b) + ")"
	case 's':
		return "(* " + ASTLine(n.a) + ")"
	default:
		return "?"
	}
}

func PrintASTLine(n *reNode) {
	fmt.Println(ASTLine(n))
}
