package service

import (
	"testing"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
)

func demandItem(id int, name, category string, demand, required float64) model.DemandItem {
	return model.DemandItem{SkillID: id, Name: name, Category: category, DemandPct: demand, RequiredPct: required}
}

func TestComputeGap(t *testing.T) {
	demand := []model.DemandItem{
		demandItem(1, "Python", "language", 90, 85),
		demandItem(2, "RAG", "llm", 60, 40),
		demandItem(3, "Docker", "mlops", 70, 50),
		demandItem(4, "AWS", "cloud", 25, 10),
		demandItem(5, "Kafka", "data", 10, 5), // di bawah ambang 20%
	}
	owned := map[int]bool{1: true}
	gaps, have, coverage := ComputeGap(demand, owned, DefaultGapConfig)

	// RAG 60×1.2 = 72 > Docker 70×1.0 > AWS 25×0.9 = 22.5
	wantOrder := []string{"RAG", "Docker", "AWS"}
	if len(gaps) != len(wantOrder) {
		t.Fatalf("gaps = %+v", gaps)
	}
	for i, name := range wantOrder {
		if gaps[i].Name != name {
			t.Errorf("gaps[%d] = %s, ingin %s", i, gaps[i].Name, name)
		}
	}
	if gaps[0].PriorityScore != 72 || gaps[2].PriorityScore != 22.5 {
		t.Errorf("skor = %v / %v", gaps[0].PriorityScore, gaps[2].PriorityScore)
	}
	if len(have) != 1 || have[0].Name != "Python" {
		t.Errorf("owned = %+v", have)
	}
	// 90 / (90+60+70+25) = 36.73%
	if coverage != 36.73 {
		t.Errorf("coverage = %v, ingin 36.73", coverage)
	}
}

func TestComputeGapTieBreaksByRequiredThenName(t *testing.T) {
	cfg := GapConfig{ThresholdPct: 0}
	gaps, _, _ := ComputeGap([]model.DemandItem{
		demandItem(1, "B", "x", 50, 10),
		demandItem(2, "A", "x", 50, 10),
		demandItem(3, "C", "x", 50, 30),
	}, nil, cfg)
	if gaps[0].Name != "C" || gaps[1].Name != "A" || gaps[2].Name != "B" {
		t.Errorf("urutan = %s %s %s, ingin C A B", gaps[0].Name, gaps[1].Name, gaps[2].Name)
	}
}

func TestComputeGapEmpty(t *testing.T) {
	gaps, have, coverage := ComputeGap(nil, nil, DefaultGapConfig)
	if gaps == nil || have == nil || coverage != 0 {
		t.Errorf("hasil kosong harus slice kosong (bukan nil) dan coverage 0: %v %v %v", gaps, have, coverage)
	}
}
