package mealplan

import (
	"math/rand/v2"
	"testing"
	"time"
)

func ids(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i + 1)
	}
	return out
}

func all(n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = true
	}
	return out
}

func TestWeekendDates(t *testing.T) {
	got := WeekendDates(2026, time.October)
	if len(got) != 9 {
		t.Fatalf("October 2026 has 9 weekend days, got %d", len(got))
	}
	if got[0].Format(DateLayout) != "2026-10-03" || got[8].Format(DateLayout) != "2026-10-31" {
		t.Fatalf("unexpected first/last: %s / %s", got[0].Format(DateLayout), got[8].Format(DateLayout))
	}
	for _, d := range got {
		if !IsWeekend(d) {
			t.Fatalf("%s is not a weekend day", d.Format(DateLayout))
		}
	}
}

func TestFillNoRepeatWhenEnoughRecipes(t *testing.T) {
	for seed := range uint64(200) {
		r := rand.New(rand.NewPCG(seed, 1))
		got := Fill(make([]int64, 8), all(8), ids(8), nil, r)
		seen := map[int64]bool{}
		for _, id := range got {
			if id == 0 || seen[id] {
				t.Fatalf("seed %d: repeat or empty slot in %v", seed, got)
			}
			seen[id] = true
		}
	}
}

func TestFillSpacesRepeatsWhenTooFewRecipes(t *testing.T) {
	for seed := range uint64(200) {
		r := rand.New(rand.NewPCG(seed, 2))
		got := Fill(make([]int64, 10), all(10), ids(8), nil, r)
		counts := map[int64]int{}
		last := map[int64]int{}
		for i, id := range got {
			counts[id]++
			if prev, ok := last[id]; ok && i-prev <= recentWindow {
				t.Fatalf("seed %d: recipe %d repeated after %d slots in %v", seed, id, i-prev, got)
			}
			last[id] = i
		}
		for id, c := range counts {
			if c > 2 {
				t.Fatalf("seed %d: recipe %d used %d times in %v", seed, id, c, got)
			}
		}
	}
}

func TestFillAvoidsPreviousMonthTail(t *testing.T) {
	for seed := range uint64(200) {
		r := rand.New(rand.NewPCG(seed, 3))
		got := Fill(make([]int64, 8), all(8), ids(8), []int64{3, 5}, r)
		if got[0] == 3 || got[0] == 5 || got[1] == 5 {
			t.Fatalf("seed %d: start of month repeats previous month's end: %v", seed, got)
		}
	}
}

func TestFillKeepsFixedSlotsAndAvoidsThem(t *testing.T) {
	slots := []int64{4, 0, 0, 0}
	fill := []bool{false, true, true, true}
	for seed := range uint64(100) {
		r := rand.New(rand.NewPCG(seed, 4))
		got := Fill(slots, fill, ids(4), nil, r)
		if got[0] != 4 {
			t.Fatalf("fixed slot changed: %v", got)
		}
		for _, id := range got[1:] {
			if id == 4 {
				t.Fatalf("seed %d: fixed recipe reused: %v", seed, got)
			}
		}
	}
}

func TestFillWithoutCandidates(t *testing.T) {
	got := Fill([]int64{0, 0}, all(2), nil, nil, rand.New(rand.NewPCG(1, 1)))
	if got[0] != 0 || got[1] != 0 {
		t.Fatalf("expected empty slots, got %v", got)
	}
}

func TestRerollPicksUnusedRecipe(t *testing.T) {
	slots := []int64{1, 2, 3, 4}
	for seed := range uint64(100) {
		r := rand.New(rand.NewPCG(seed, 5))
		got := Reroll(slots, 1, ids(6), nil, r)
		if got != 5 && got != 6 {
			t.Fatalf("seed %d: expected an unused recipe (5 or 6), got %d", seed, got)
		}
	}
}

func TestRerollSingleCandidateKeepsCurrent(t *testing.T) {
	if got := Reroll([]int64{1}, 0, []int64{1}, nil, rand.New(rand.NewPCG(1, 1))); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}
