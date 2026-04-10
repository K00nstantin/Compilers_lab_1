package main

import (
	"fmt"
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
