package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	args := os.Args[1:]
	reStr := ""
	if len(args) >= 1 {
		reStr = args[0]
	}
	var chain string
	var haveChain bool
	if len(args) >= 2 {
		haveChain = true
		chain = strings.Join(args[1:], " ")
	}

	ast, err := parseRegex(reStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка разбора: %v\n", err)
		os.Exit(1)
	}

	alphaSet := map[rune]struct{}{}
	collectAlphabet(ast, alphaSet)
	var alphabet []rune
	for r := range alphaSet {
		alphabet = append(alphabet, r)
	}
	sort.Slice(alphabet, func(i, j int) bool { return alphabet[i] < alphabet[j] })

	dfa := regexToDFA(ast, alphabet)
	fmt.Println(" ДКА из regex")
	fmt.Print(dfa.String())

	min := dfa.Minimize()
	fmt.Println("Минимальный эквивалентный ДКА")
	fmt.Printf("число состояний: было %d, стало %d\n", dfa.NumStates, min.NumStates)
	fmt.Print(min.String())

	if haveChain {
		fmt.Println("Минимальный ДКА для цепочки")
		steps, ok := min.Simulate(chain)
		for _, line := range steps {
			fmt.Println(line)
		}
		if ok {
			fmt.Println("результат: цепочка допускается")
		} else {
			fmt.Println("результат: цепочка не допускается")
		}
	}
}
