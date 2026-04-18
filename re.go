package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type reNode struct {
	kind     byte
	ch       rune
	a, b     *reNode
	pos      int
	nullable bool
	firstpos map[int]struct{}
	lastpos  map[int]struct{}
}

type parser struct {
	s string
	i int
}

func (p *parser) peek() rune {
	if p.i >= len(p.s) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(p.s[p.i:])
	return r
}

func (p *parser) next() rune {
	if p.i >= len(p.s) {
		return 0
	}
	r, w := utf8.DecodeRuneInString(p.s[p.i:])
	p.i += w
	return r
}

func parseRegex(s string) (*reNode, error) {
	p := &parser{s: strings.TrimSpace(s)}
	if p.peek() == 0 {
		return &reNode{kind: 'l', ch: 0}, nil
	}
	n, err := p.parseAlt()
	if err != nil {
		return nil, err
	}
	if p.peek() != 0 {
		return nil, fmt.Errorf("лишние символы после выражения")
	}
	return n, nil
}

func (p *parser) parseAlt() (*reNode, error) {
	left, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	for p.peek() == '|' {
		p.next()
		right, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		left = &reNode{kind: 'a', a: left, b: right}
	}
	return left, nil
}

func (p *parser) parseConcat() (*reNode, error) {
	var nodes []*reNode
	for {
		n, err := p.parseStar()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
		r := p.peek()
		if r == 0 || r == '|' || r == ')' {
			break
		}
	}
	if len(nodes) == 0 {
		return &reNode{kind: 'l', ch: 0}, nil
	}
	out := nodes[0]
	for i := 1; i < len(nodes); i++ {
		out = &reNode{kind: 'c', a: out, b: nodes[i]}
	}
	return out, nil
}

func (p *parser) parseStar() (*reNode, error) {
	n, err := p.parsePrim()
	if err != nil {
		return nil, err
	}
	for p.peek() == '*' {
		p.next()
		n = &reNode{kind: 's', a: n}
	}
	return n, nil
}

func (p *parser) parsePrim() (*reNode, error) {
	r := p.peek()
	switch r {
	case '(':
		p.next()
		n, err := p.parseAlt()
		if err != nil {
			return nil, err
		}
		if p.peek() != ')' {
			return nil, fmt.Errorf("ожидалась ')'")
		}
		p.next()
		return n, nil
	case '|', '*', ')':
		return nil, fmt.Errorf("неожиданный символ %q", r)
	case 0:
		return nil, fmt.Errorf("неожиданный конец строки")
	default:
		p.next()
		if r == '\\' {
			r = p.next()
			if r == 0 {
				return nil, fmt.Errorf("неполный escape")
			}
		}
		return &reNode{kind: 'l', ch: r}, nil
	}
}

func collectAlphabet(n *reNode, set map[rune]struct{}) {
	if n == nil {
		return
	}
	switch n.kind {
	case 'l':
		if n.ch != 0 {
			set[n.ch] = struct{}{}
		}
	default:
		collectAlphabet(n.a, set)
		collectAlphabet(n.b, set)
	}
}

func stateSetKey(states []int) string {
	var b strings.Builder
	for i, s := range states {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d", s)
	}
	return b.String()
}

func setUnion(a, b map[int]struct{}) map[int]struct{} {
	out := make(map[int]struct{}, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

func setToSortedSlice(s map[int]struct{}) []int {
	out := make([]int, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

func addFollow(follow map[int]map[int]struct{}, from int, toSet map[int]struct{}) {
	if _, ok := follow[from]; !ok {
		follow[from] = make(map[int]struct{})
	}
	for to := range toSet {
		follow[from][to] = struct{}{}
	}
}

func numberPositions(n *reNode, next *int, posSym map[int]rune) {
	if n == nil {
		return
	}
	numberPositions(n.a, next, posSym)
	numberPositions(n.b, next, posSym)
	if n.kind == 'l' && n.ch != 0 {
		*next++
		n.pos = *next
		posSym[n.pos] = n.ch
	}
}

func computeFunctions(n *reNode, follow map[int]map[int]struct{}) {
	if n == nil {
		return
	}
	computeFunctions(n.a, follow)
	computeFunctions(n.b, follow)

	switch n.kind {
	case 'l':
		if n.ch == 0 {
			n.nullable = true
			n.firstpos = map[int]struct{}{}
			n.lastpos = map[int]struct{}{}
			return
		}
		n.nullable = false
		n.firstpos = map[int]struct{}{n.pos: {}}
		n.lastpos = map[int]struct{}{n.pos: {}}
	case 'a':
		n.nullable = n.a.nullable || n.b.nullable
		n.firstpos = setUnion(n.a.firstpos, n.b.firstpos)
		n.lastpos = setUnion(n.a.lastpos, n.b.lastpos)
	case 'c':
		n.nullable = n.a.nullable && n.b.nullable
		if n.a.nullable {
			n.firstpos = setUnion(n.a.firstpos, n.b.firstpos)
		} else {
			n.firstpos = setUnion(n.a.firstpos, map[int]struct{}{})
		}
		if n.b.nullable {
			n.lastpos = setUnion(n.a.lastpos, n.b.lastpos)
		} else {
			n.lastpos = setUnion(n.b.lastpos, map[int]struct{}{})
		}
		for i := range n.a.lastpos {
			addFollow(follow, i, n.b.firstpos)
		}
	case 's':
		n.nullable = true
		n.firstpos = setUnion(n.a.firstpos, map[int]struct{}{})
		n.lastpos = setUnion(n.a.lastpos, map[int]struct{}{})
		for i := range n.a.lastpos {
			addFollow(follow, i, n.a.firstpos)
		}
	}
}

func regexToDFA(ast *reNode, alphabet []rune) *DFA {
	const endMarker rune = -1
	root := &reNode{
		kind: 'c',
		a:    ast,
		b:    &reNode{kind: 'l', ch: endMarker},
	}

	sigma := make([]rune, len(alphabet))
	copy(sigma, alphabet)
	sort.Slice(sigma, func(i, j int) bool { return sigma[i] < sigma[j] })
	symIdx := make(map[rune]int, len(sigma))
	for i, r := range sigma {
		symIdx[r] = i
	}
	m := len(sigma)

	posSym := make(map[int]rune)
	nextPos := 0
	numberPositions(root, &nextPos, posSym)
	follow := make(map[int]map[int]struct{})
	computeFunctions(root, follow)

	markerPos := root.b.pos
	startSet := setToSortedSlice(root.firstpos)
	startKey := stateSetKey(startSet)

	stateSets := [][]int{startSet}
	stateMap := map[string]int{startKey: 0}
	queue := []int{0}
	transOut := []map[rune]int{make(map[rune]int)}
	final := make(map[int]bool)

	for len(queue) > 0 {
		sid := queue[0]
		queue = queue[1:]
		S := stateSets[sid]

		containsMarker := false
		for _, p := range S {
			if p == markerPos {
				containsMarker = true
				break
			}
		}
		if containsMarker {
			final[sid] = true
		}

		for _, a := range sigma {
			Uset := make(map[int]struct{})
			for _, p := range S {
				if posSym[p] != a {
					continue
				}
				for fp := range follow[p] {
					Uset[fp] = struct{}{}
				}
			}
			if len(Uset) == 0 {
				continue
			}
			U := setToSortedSlice(Uset)
			k := stateSetKey(U)
			tid, ok := stateMap[k]
			if !ok {
				tid = len(stateSets)
				stateMap[k] = tid
				stateSets = append(stateSets, U)
				transOut = append(transOut, make(map[rune]int))
				queue = append(queue, tid)
			}
			transOut[sid][a] = tid
		}
	}

	n := len(stateSets)
	dead := -1
	if m > 0 {
		needDead := false
		for s := 0; s < n; s++ {
			for _, a := range sigma {
				if _, ok := transOut[s][a]; !ok {
					needDead = true
					break
				}
			}
			if needDead {
				break
			}
		}
		if needDead {
			dead = n
			n++
			transOut = append(transOut, make(map[rune]int))
			for _, a := range sigma {
				transOut[dead][a] = dead
			}
			for s := 0; s < dead; s++ {
				for _, a := range sigma {
					if _, ok := transOut[s][a]; !ok {
						transOut[s][a] = dead
					}
				}
			}
		}
	}

	trans := make([][]int, n)
	for s := 0; s < n; s++ {
		trans[s] = make([]int, m)
		for j, a := range sigma {
			if t, ok := transOut[s][a]; ok {
				trans[s][j] = t
			} else {
				trans[s][j] = s
			}
		}
	}
	_ = dead

	startDFA := 0
	return &DFA{
		NumStates: n,
		Start:     startDFA,
		Final:     final,
		Trans:     trans,
		Sigma:     sigma,
		SymIdx:    symIdx,
	}
}
