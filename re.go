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
