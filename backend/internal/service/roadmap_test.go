package service

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ahmadhasan/ai-career-compass/backend/internal/model"
	"github.com/ahmadhasan/ai-career-compass/backend/internal/repository"
)

func drafts(ids ...int) []*draftNode {
	out := make([]*draftNode, len(ids))
	for i, id := range ids {
		out[i] = &draftNode{skill: repository.SkillInfo{ID: id}, rationale: "fallback", estWeeks: 4}
	}
	return out
}

func TestApplyExplanationsRejectsSkillsOutsideInput(t *testing.T) {
	ds := drafts(1, 2, 3)
	applied, dropped := ApplyExplanations(ds, []model.ExplainedNode{
		{SkillID: 1, Rationale: "  Fondasi semua skill lain.  ", EstWeeks: 2.2},
		{SkillID: 99, Rationale: "skill karangan LLM", EstWeeks: 1},
		{SkillID: 1, Rationale: "duplikat diabaikan", EstWeeks: 9},
		{SkillID: 2, Rationale: "", EstWeeks: 100},
		{SkillID: 42, Rationale: "lagi", EstWeeks: 1},
	})
	if applied != 2 || !reflect.DeepEqual(dropped, []int{99, 42}) {
		t.Fatalf("applied = %d, dropped = %v", applied, dropped)
	}
	if ds[0].rationale != "Fondasi semua skill lain." || ds[0].estWeeks != 2 {
		t.Errorf("node 1 = %q %v", ds[0].rationale, ds[0].estWeeks)
	}
	if ds[1].rationale != "fallback" || ds[1].estWeeks != maxEstWeeks {
		t.Errorf("node 2: rationale kosong tetap fallback, est di-clamp: %q %v", ds[1].rationale, ds[1].estWeeks)
	}
	if ds[2].rationale != "fallback" || ds[2].estWeeks != 4 {
		t.Errorf("node 3 tanpa penjelasan harus tetap fallback: %q %v", ds[2].rationale, ds[2].estWeeks)
	}
}

func TestApplyExplanationsIgnoresInvalidWeeksAndTruncates(t *testing.T) {
	ds := drafts(1, 2)
	ApplyExplanations(ds, []model.ExplainedNode{
		{SkillID: 1, Rationale: strings.Repeat("a", 800), EstWeeks: math.NaN()},
		{SkillID: 2, Rationale: "ok", EstWeeks: -3},
	})
	if n := len([]rune(ds[0].rationale)); n != maxRationaleChars {
		t.Errorf("rationale tidak dipotong: %d karakter", n)
	}
	if ds[0].estWeeks != 4 || ds[1].estWeeks != 4 {
		t.Errorf("est_weeks tidak valid harus diabaikan: %v %v", ds[0].estWeeks, ds[1].estWeeks)
	}
}

func TestClampWeeks(t *testing.T) {
	for in, want := range map[float64]float64{0.1: 0.5, 1.26: 1.5, 3.74: 3.5, 40: 12} {
		if got := clampWeeks(in); got != want {
			t.Errorf("clampWeeks(%v) = %v, ingin %v", in, got, want)
		}
	}
}

func TestFallbackRationale(t *testing.T) {
	pct := 62.5
	cases := []struct {
		d    draftNode
		want string
	}{
		{draftNode{demandPct: &pct}, "Diminta di 62% lowongan AI Engineer."},
		{draftNode{requiredFor: []string{"FastAPI", "Flask"}}, "Prasyarat untuk FastAPI, Flask."},
		{draftNode{demandPct: &pct, requiredFor: []string{"FastAPI"}}, "Diminta di 62% lowongan AI Engineer dan menjadi prasyarat FastAPI."},
	}
	for _, c := range cases {
		if got := fallbackRationale(&c.d, "AI Engineer"); got != c.want {
			t.Errorf("got %q, ingin %q", got, c.want)
		}
	}
}
