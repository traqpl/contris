//go:build js && wasm

package main

import "math/rand"

// Vec2 is a row/column offset within a piece shape.
type Vec2 struct{ R, C int }

var baseShapes = map[string][]Vec2{
	"tu2": {{0, 0}, {0, 1}},
	"tu4": {{0, 0}, {0, 1}, {0, 2}, {0, 3}},
	"O":   {{0, 0}, {0, 1}, {1, 0}, {1, 1}},
	"L":   {{0, 0}, {1, 0}, {2, 0}, {2, 1}},
	"J":   {{0, 1}, {1, 1}, {2, 0}, {2, 1}},
	"T":   {{0, 0}, {0, 1}, {0, 2}, {1, 1}},
	"S":   {{0, 1}, {0, 2}, {1, 0}, {1, 1}},
	"Z":   {{0, 0}, {0, 1}, {1, 1}, {1, 2}},
}

type PieceDef struct {
	Shape []Vec2
	Wear  []int
	Co    string
	Label string
	W     int // spawn weight
}

func copyShape(s []Vec2) []Vec2 {
	out := make([]Vec2, len(s))
	copy(out, s)
	return out
}

func rotate90(shape []Vec2) []Vec2 {
	r := make([]Vec2, len(shape))
	for i, v := range shape {
		r[i] = Vec2{v.C, -v.R}
	}
	minR, minC := r[0].R, r[0].C
	for _, v := range r {
		if v.R < minR {
			minR = v.R
		}
		if v.C < minC {
			minC = v.C
		}
	}
	for i := range r {
		r[i].R -= minR
		r[i].C -= minC
	}
	return r
}

func shapeDims(shape []Vec2) (h, w int) {
	for _, v := range shape {
		if v.R+1 > h {
			h = v.R + 1
		}
		if v.C+1 > w {
			w = v.C + 1
		}
	}
	return
}

var pool []PieceDef

func init() {
	add := func(key, co, label string, w int) {
		pool = append(pool, PieceDef{
			Shape: copyShape(baseShapes[key]),
			Co:    co, Label: label, W: w,
		})
	}

	// Pomarańczowe 3× częstsze niż białe
	add("tu2", "orange", "2TU", 9)
	add("tu2", "white", "2TU", 3)
	add("tu4", "orange", "4TU", 6)
	add("tu4", "white", "4TU", 2)

	pool = append(pool, PieceDef{Shape: []Vec2{{0, 0}}, Co: "reef", Label: "REEFER", W: 2})
	pool = append(pool, PieceDef{Shape: []Vec2{{0, 0}}, Co: "haz", Label: "HAZMAT", W: 2})

	for _, key := range []string{"O", "L", "J", "T", "S", "Z"} {
		add(key, "orange", key, 3)
		add(key, "white", key, 1)
	}
}

func randDef() PieceDef {
	total := 0
	for _, d := range pool {
		total += d.W
	}
	n := rand.Intn(total)
	for _, d := range pool {
		n -= d.W
		if n < 0 {
			return newDef(d)
		}
	}
	return newDef(pool[0])
}

// newDef kopiuje definicję i przypisuje stabilny wear każdej komórce.
func newDef(d PieceDef) PieceDef {
	shape := copyShape(d.Shape)
	wear := make([]int, len(shape))
	for i := range wear {
		v := rand.Intn(100) // 0–99; tylko v>=98 (~2%) daje wear==3
		if v >= 98 {
			wear[i] = 3
		} else {
			wear[i] = v % 3 // 0–2: brak/drobne deformacje
		}
	}
	return PieceDef{Shape: shape, Wear: wear, Co: d.Co, Label: d.Label, W: d.W}
}
