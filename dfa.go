package main

import (
	"fmt"
	"strings"
)

type DFA struct {
	NumStates int
	Start     int
	Final     map[int]bool
	Trans     [][]int
	Sigma     []rune
	SymIdx    map[rune]int
}

func (d *DFA) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "состояний: %d, старт: %d\n", d.NumStates, d.Start)
	fmt.Fprintf(&b, "терминальные: ")
	first := true
	for s := 0; s < d.NumStates; s++ {
		if d.Final[s] {
			if !first {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%d", s)
			first = false
		}
	}
	b.WriteByte('\n')
	fmt.Fprintf(&b, "алфавит: %q\n", string(d.Sigma))
	for s := 0; s < d.NumStates; s++ {
		for j, a := range d.Sigma {
			t := d.Trans[s][j]
			fmt.Fprintf(&b, "δ(%d,%q)=%d\n", s, a, t)
		}
	}
	return b.String()
}

func (d *DFA) Minimize() *DFA {
	n := d.NumStates
	m := len(d.Sigma)
	if n == 0 {
		return &DFA{
			NumStates: 0,
			Start:     0,
			Final:     map[int]bool{},
			Trans:     [][]int{},
			Sigma:     append([]rune(nil), d.Sigma...),
			SymIdx:    mapsClone(d.SymIdx),
		}
	}

	inv := make([][][]int, n)
	for i := range inv {
		inv[i] = make([][]int, m)
	}
	for s := 0; s < n; s++ {
		for a := 0; a < m; a++ {
			t := d.Trans[s][a]
			inv[t][a] = append(inv[t][a], s)
		}
	}

	final := make([]bool, n)
	for s := 0; s < n; s++ {
		final[s] = d.Final[s]
	}
	var blocks [][]int
	var classOf []int
	nonF := make([]int, 0, n)
	F := make([]int, 0, n)
	for s := 0; s < n; s++ {
		if final[s] {
			F = append(F, s)
		} else {
			nonF = append(nonF, s)
		}
	}
	switch {
	case len(F) == 0:
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		blocks = [][]int{all}
		classOf = make([]int, n)
		for s := 0; s < n; s++ {
			classOf[s] = 0
		}
	case len(nonF) == 0:
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		blocks = [][]int{all}
		classOf = make([]int, n)
		for s := 0; s < n; s++ {
			classOf[s] = 0
		}
	default:
		blocks = [][]int{append([]int(nil), F...), append([]int(nil), nonF...)}
		classOf = make([]int, n)
		for _, s := range blocks[0] {
			classOf[s] = 0
		}
		for _, s := range blocks[1] {
			classOf[s] = 1
		}
	}

	type pair struct{ blk, sym int }
	queue := make([]pair, 0)
	enqueue := func(blk int) {
		for a := 0; a < m; a++ {
			queue = append(queue, pair{blk, a})
		}
	}
	for b := range blocks {
		enqueue(b)
	}

	involvedStates := make([][]int, len(blocks))
	resetInvolved := func() {
		for i := range involvedStates {
			involvedStates[i] = involvedStates[i][:0]
		}
	}

	for qi := 0; qi < len(queue); qi++ {
		C := queue[qi].blk
		a := queue[qi].sym
		for len(involvedStates) < len(blocks) {
			involvedStates = append(involvedStates, nil)
		}
		resetInvolved()
		for _, q := range blocks[C] {
			for _, r := range inv[q][a] {
				ci := classOf[r]
				involvedStates[ci] = append(involvedStates[ci], r)
			}
		}
		for ci := range blocks {
			splitSet := involvedStates[ci]
			if len(splitSet) == 0 || len(splitSet) == len(blocks[ci]) {
				continue
			}
			oldBlk := blocks[ci]
			inSplit := make(map[int]struct{}, len(splitSet))
			for _, s := range splitSet {
				inSplit[s] = struct{}{}
			}
			var stay, move []int
			for _, s := range oldBlk {
				if _, ok := inSplit[s]; ok {
					move = append(move, s)
				} else {
					stay = append(stay, s)
				}
			}
			if len(move) > len(stay) {
				stay, move = move, stay
			}
			j := len(blocks)
			blocks = append(blocks, move)
			blocks[ci] = stay
			for _, s := range move {
				classOf[s] = j
			}
			for len(involvedStates) < len(blocks) {
				involvedStates = append(involvedStates, nil)
			}
			enqueue(j)
		}
	}

	rep := make([]int, len(blocks))
	for bi, blk := range blocks {
		rep[bi] = blk[0]
	}
	startMin := classOf[d.Start]
	finalMin := make(map[int]bool, len(blocks))
	for bi, blk := range blocks {
		for _, s := range blk {
			if d.Final[s] {
				finalMin[bi] = true
				break
			}
		}
	}
	transMin := make([][]int, len(blocks))
	for bi := range blocks {
		transMin[bi] = make([]int, m)
		s0 := rep[bi]
		for a := 0; a < m; a++ {
			transMin[bi][a] = classOf[d.Trans[s0][a]]
		}
	}

	return &DFA{
		NumStates: len(blocks),
		Start:     startMin,
		Final:     finalMin,
		Trans:     transMin,
		Sigma:     append([]rune(nil), d.Sigma...),
		SymIdx:    mapsClone(d.SymIdx),
	}
}

func mapsClone(m map[rune]int) map[rune]int {
	if m == nil {
		return nil
	}
	out := make(map[rune]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (d *DFA) Simulate(w string) (steps []string, ok bool) {
	s := d.Start
	var trace []string
	trace = append(trace, fmt.Sprintf("q%d", s))
	for _, r := range w {
		j, ok := d.SymIdx[r]
		if !ok {
			trace = append(trace, fmt.Sprintf("символ %q не в алфавите", r))
			return trace, false
		}
		s = d.Trans[s][j]
		trace = append(trace, fmt.Sprintf("%q → q%d", r, s))
	}
	return trace, d.Final[s]
}
