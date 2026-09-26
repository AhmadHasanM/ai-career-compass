package service

import (
	"errors"
	"reflect"
	"testing"
)

func byID(a, b int) bool { return a < b }

func TestTopoSortRespectsPrerequisites(t *testing.T) {
	// 1 Python -> 2 FastAPI -> 3 Docker(?) ; 4 SQL berdiri sendiri; 5 butuh 2 dan 4
	prereqs := map[int][]int{2: {1}, 3: {2}, 5: {2, 4}}
	order, depth, err := TopoSort([]int{5, 3, 4, 2, 1}, prereqs, byID)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[int]int{}
	for i, n := range order {
		pos[n] = i
	}
	for n, ps := range prereqs {
		for _, p := range ps {
			if pos[p] > pos[n] {
				t.Errorf("prerequisite %d muncul setelah %d: %v", p, n, order)
			}
		}
	}
	want := map[int]int{1: 0, 4: 0, 2: 1, 3: 2, 5: 2}
	if !reflect.DeepEqual(depth, want) {
		t.Errorf("depth = %v, ingin %v", depth, want)
	}
}

func TestTopoSortWithoutPrerequisitesUsesTieBreak(t *testing.T) {
	score := map[int]float64{10: 30, 20: 90, 30: 60}
	higherFirst := func(a, b int) bool { return score[a] > score[b] }
	order, depth, err := TopoSort([]int{10, 20, 30}, nil, higherFirst)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []int{20, 30, 10}) {
		t.Errorf("order = %v, ingin urut skor tertinggi", order)
	}
	for _, d := range depth {
		if d != 0 {
			t.Errorf("depth tanpa prerequisite harus 0: %v", depth)
		}
	}
}

func TestTopoSortIgnoresEdgesOutsideSetAndSelfLoops(t *testing.T) {
	order, _, err := TopoSort([]int{2, 3}, map[int][]int{2: {1, 2}, 3: {2, 2}}, byID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []int{2, 3}) {
		t.Errorf("order = %v", order)
	}
}

func TestTopoSortDetectsCycle(t *testing.T) {
	_, _, err := TopoSort([]int{1, 2, 3, 4}, map[int][]int{1: {3}, 2: {1}, 3: {2}}, byID)
	var ce *CycleError
	if !errors.As(err, &ce) || !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v, ingin CycleError", err)
	}
	if !reflect.DeepEqual(ce.SkillIDs, []int{1, 2, 3}) {
		t.Errorf("skill dalam siklus = %v, ingin [1 2 3]", ce.SkillIDs)
	}
}

func TestTopoSortDuplicateNodes(t *testing.T) {
	order, _, err := TopoSort([]int{2, 1, 2}, map[int][]int{2: {1}}, byID)
	if err != nil || !reflect.DeepEqual(order, []int{1, 2}) {
		t.Errorf("order = %v, err = %v", order, err)
	}
}

func TestTopoSortEmpty(t *testing.T) {
	order, _, err := TopoSort(nil, nil, byID)
	if err != nil || len(order) != 0 {
		t.Errorf("order = %v, err = %v", order, err)
	}
}

func TestPrerequisiteClosure(t *testing.T) {
	prereqs := map[int][]int{3: {2}, 2: {1}, 5: {4}, 6: {7}, 7: {6}}
	owned := map[int]bool{4: true}
	got := PrerequisiteClosure([]int{3, 5, 6}, prereqs, owned)
	want := []int{3, 2, 1, 5, 6, 7}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("closure = %v, ingin %v (4 dimiliki, siklus 6<->7 tidak loop)", got, want)
	}
}

func TestCappedClosure(t *testing.T) {
	// target 10 butuh 1 -> 2 (3 node); target 20 butuh 1 (berbagi); target 30 butuh 4,5,6 (4 node)
	prereqs := map[int][]int{10: {2}, 2: {1}, 20: {1}, 30: {4}, 4: {5}, 5: {6}}
	got := CappedClosure([]int{10, 20, 30}, prereqs, nil, 5)
	// 10 (+2,+1) = 3 node, 20 (+0 baru prasyarat) = 4 node, 30 butuh 4 node lagi -> dilewati utuh
	if !reflect.DeepEqual(got, []int{10, 2, 1, 20}) {
		t.Errorf("capped = %v, ingin [10 2 1 20]", got)
	}
	if got := CappedClosure([]int{30, 10}, prereqs, map[int]bool{5: true}, 10); !reflect.DeepEqual(got, []int{30, 4, 10, 2, 1}) {
		t.Errorf("owned memotong rantai: %v", got)
	}
}
