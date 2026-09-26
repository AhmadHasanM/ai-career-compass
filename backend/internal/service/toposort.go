package service

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
)

// ErrCycle berarti prerequisite di antara skill yang dipilih membentuk siklus.
var ErrCycle = errors.New("siklus prerequisite")

type CycleError struct{ SkillIDs []int }

func (e *CycleError) Error() string {
	return fmt.Sprintf("%v di antara skill %v", ErrCycle, e.SkillIDs)
}
func (e *CycleError) Unwrap() error { return ErrCycle }

// TopoSort mengurutkan nodes sehingga setiap prerequisite muncul sebelum skill yang membutuhkannya.
// Hanya edge di antara nodes yang diperhitungkan. Di antara skill yang sama-sama siap, before(a, b)
// menentukan urutan (misal skor prioritas lebih tinggi dulu), sehingga hasilnya deterministik.
//
// depth[id] = panjang rantai prerequisite terpanjang menuju skill itu (0 = tanpa prerequisite).
func TopoSort(nodes []int, prereqs map[int][]int, before func(a, b int) bool) (order []int, depth map[int]int, err error) {
	inSet := make(map[int]bool, len(nodes))
	unique := make([]int, 0, len(nodes))
	for _, n := range nodes {
		if !inSet[n] {
			inSet[n] = true
			unique = append(unique, n)
		}
	}
	nodes = unique
	indegree := make(map[int]int, len(nodes))
	dependents := make(map[int][]int, len(nodes))
	for _, n := range nodes {
		if _, ok := indegree[n]; !ok {
			indegree[n] = 0
		}
		seen := map[int]bool{}
		for _, p := range prereqs[n] {
			if inSet[p] && p != n && !seen[p] {
				seen[p] = true
				indegree[n]++
				dependents[p] = append(dependents[p], n)
			}
		}
	}

	ready := &intHeap{before: before}
	depth = make(map[int]int, len(nodes))
	queued := map[int]bool{}
	for _, n := range nodes {
		if indegree[n] == 0 && !queued[n] {
			queued[n] = true
			depth[n] = 0
			heap.Push(ready, n)
		}
	}
	for ready.Len() > 0 {
		n := heap.Pop(ready).(int)
		order = append(order, n)
		for _, d := range dependents[n] {
			if depth[n]+1 > depth[d] {
				depth[d] = depth[n] + 1
			}
			indegree[d]--
			if indegree[d] == 0 {
				heap.Push(ready, d)
			}
		}
	}

	if len(order) < len(inSet) {
		var stuck []int
		for n, deg := range indegree {
			if deg > 0 {
				stuck = append(stuck, n)
			}
		}
		sort.Ints(stuck)
		return nil, nil, &CycleError{SkillIDs: stuck}
	}
	return order, depth, nil
}

// PrerequisiteClosure menambahkan prerequisite (transitif) dari targets yang belum dimiliki.
// Guard visited mencegah loop tak berujung bila data prerequisite ternyata bersiklus.
func PrerequisiteClosure(targets []int, prereqs map[int][]int, owned map[int]bool) []int {
	included := map[int]bool{}
	var out []int
	var visit func(int)
	visit = func(n int) {
		if included[n] {
			return
		}
		included[n] = true
		out = append(out, n)
		for _, p := range prereqs[n] {
			if !owned[p] {
				visit(p)
			}
		}
	}
	for _, t := range targets {
		visit(t)
	}
	return out
}

// CappedClosure menambahkan target satu per satu (urut prioritas) beserta prasyaratnya, dan berhenti
// sebelum total node melebihi max. Target yang prasyaratnya tidak muat dilewati utuh, sehingga
// setiap skill di hasil selalu membawa semua prasyarat yang belum dimiliki.
func CappedClosure(targets []int, prereqs map[int][]int, owned map[int]bool, max int) []int {
	included := map[int]bool{}
	var out []int
	for _, t := range targets {
		var added []int
		for _, n := range PrerequisiteClosure([]int{t}, prereqs, owned) {
			if !included[n] {
				added = append(added, n)
			}
		}
		if len(out)+len(added) > max {
			continue
		}
		for _, n := range added {
			included[n] = true
		}
		out = append(out, added...)
	}
	return out
}

type intHeap struct {
	items  []int
	before func(a, b int) bool
}

func (h intHeap) Len() int           { return len(h.items) }
func (h intHeap) Less(i, j int) bool { return h.before(h.items[i], h.items[j]) }
func (h intHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *intHeap) Push(x any)        { h.items = append(h.items, x.(int)) }
func (h *intHeap) Pop() any {
	n := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return n
}
