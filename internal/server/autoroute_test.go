package server

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

func randomGraph(n int, r *rand.Rand, density float64) *graph {
	g := newGraph()
	for i := 0; i < n; i++ {
		g.node(fmt.Sprint(i))
	}
	for a := 0; a < n; a++ {
		for b := 0; b < n; b++ {
			if a != b && r.Float64() < density {
				g.edge(fmt.Sprint(a), fmt.Sprint(b), 1+r.Float64()*99, fmt.Sprintf("n%d", b))
			}
		}
	}
	return g
}

func bruteForce(g *graph, src, dst, k int) float64 {
	var cand []int
	for i := range g.ids {
		if i != src && i != dst {
			cand = append(cand, i)
		}
	}
	best := math.Inf(1)
	used := make([]bool, len(cand))
	var rec func(last, depth int, cost float64)
	rec = func(last, depth int, cost float64) {
		if depth == k {
			if c := cost + g.cost[last][dst]; c < best {
				best = c
			}
			return
		}
		for i, c := range cand {
			if !used[i] {
				used[i] = true
				rec(c, depth+1, cost+g.cost[last][c])
				used[i] = false
			}
		}
	}
	rec(src, 0, 0)
	return best
}

func TestSolverMatchesBruteForce(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for trial := 0; trial < 200; trial++ {
		n := 3 + r.IntN(5)
		g := randomGraph(n, r, 0.85)
		k := r.IntN(n - 1)
		path, cost, err := solve(g, 0, n-1, k)
		want := math.Inf(1)
		for kk := k; kk >= 0 && math.IsInf(want, 1); kk-- {
			want = bruteForce(g, 0, n-1, kk)
		}
		if math.IsInf(want, 1) {
			if err == nil {
				t.Fatalf("trial %d: solver found a path where none exists", trial)
			}
			continue
		}
		if err != nil || math.Abs(cost-want) > 1e-9 {
			t.Fatalf("trial %d (n=%d k=%d): got %v %v, want %v", trial, n, k, cost, err, want)
		}
		if math.Abs(g.pathCost(path)-cost) > 1e-9 || path[0] != 0 || path[len(path)-1] != n-1 {
			t.Fatalf("trial %d: inconsistent path %v", trial, path)
		}
		seen := map[int]bool{}
		for _, p := range path {
			if seen[p] {
				t.Fatalf("trial %d: node visited twice %v", trial, path)
			}
			seen[p] = true
		}
	}
}

func TestSolverAllNodesFifteen(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	g := randomGraph(15, r, 1)
	path, cost, err := solve(g, 0, 14, AllHops)
	if err != nil || len(path) != 15 {
		t.Fatalf("path %v err %v", path, err)
	}
	greedy := []int{0}
	used := map[int]bool{0: true, 14: true}
	for len(greedy) < 14 {
		last, best, bc := greedy[len(greedy)-1], -1, math.Inf(1)
		for c := 1; c < 14; c++ {
			if !used[c] && g.cost[last][c] < bc {
				best, bc = c, g.cost[last][c]
			}
		}
		used[best] = true
		greedy = append(greedy, best)
	}
	greedy = append(greedy, 14)
	gc := g.pathCost(greedy)
	if cost > gc+1e-9 {
		t.Fatalf("optimal %.1f worse than greedy %.1f", cost, gc)
	}
	t.Logf("15 nodes: optimal %.1f ms vs greedy nearest-neighbour %.1f ms (%.0f%% longer)", cost, gc, (gc/cost-1)*100)
}

func TestSolverSkipsDeadLinks(t *testing.T) {
	g := newGraph()
	for _, e := range []struct {
		a, b string
		ms   float64
	}{{"A", "B", 10}, {"B", "D", 10}, {"A", "C", 1}, {"C", "D", 1}} {
		g.edge(e.a, e.b, e.ms, e.b)
	}
	path, cost, err := solve(g, g.idx["A"], g.idx["D"], 1)
	if err != nil || cost != 2 || g.ids[path[1]] != "C" {
		t.Fatalf("got %v %v %v", path, cost, err)
	}
	g.cost[g.idx["C"]][g.idx["D"]] = math.Inf(1)
	path, cost, err = solve(g, g.idx["A"], g.idx["D"], 1)
	if err != nil || cost != 20 || g.ids[path[1]] != "B" {
		t.Fatalf("after C→D died: %v %v %v", path, cost, err)
	}
	_, _, err = solve(g, g.idx["A"], g.idx["C"]+100-100, 0)
	if err != nil {
		t.Fatalf("direct A→C: %v", err)
	}
}
