// Package mealplan assigns recipes to weekend days so that, within a month,
// no recipe repeats while unused ones are still available.
package mealplan

import (
	"math/rand/v2"
	"slices"
	"time"
)

// DateLayout is the storage and API format of a plan date.
const DateLayout = "2006-01-02"

// recentWindow is how many preceding slots (across the month boundary) a
// recipe should preferably not appear in again.
const recentWindow = 2

// WeekendDates returns every Saturday and Sunday of the given month, in order.
func WeekendDates(year int, month time.Month) []time.Time {
	var out []time.Time
	for d := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC); d.Month() == month; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			out = append(out, d)
		}
	}
	return out
}

// IsWeekend reports whether t falls on a Saturday or Sunday.
func IsWeekend(t time.Time) bool {
	wd := t.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

type candidate struct {
	id     int64
	count  int
	recent bool
	last   int
	key    float64
}

// rank orders candidates by: fewest uses this month, not among the recent
// dishes, longest since last use, random.
func rank(ids []int64, counts map[int64]int, recent []int64, lastIdx map[int64]int, r *rand.Rand) []int64 {
	cs := make([]candidate, 0, len(ids))
	for _, id := range ids {
		last, ok := lastIdx[id]
		if !ok {
			last = -1 << 30
		}
		cs = append(cs, candidate{id: id, count: counts[id], recent: slices.Contains(recent, id), last: last, key: r.Float64()})
	}
	slices.SortFunc(cs, func(a, b candidate) int {
		switch {
		case a.count != b.count:
			return a.count - b.count
		case a.recent != b.recent:
			if a.recent {
				return 1
			}
			return -1
		case a.last != b.last:
			return a.last - b.last
		case a.key < b.key:
			return -1
		case a.key > b.key:
			return 1
		}
		return 0
	})
	out := make([]int64, len(cs))
	for i, c := range cs {
		out[i] = c.id
	}
	return out
}

// Fill assigns a recipe to every slot where fill[i] is true, keeping all other
// slots as they are. slots holds the month's current recipe ids in date order
// (0 = none). prevTail holds the last recipe ids of the previous month, oldest
// first. Already assigned slots count towards the monthly usage, so a filled
// slot avoids recipes that are already planned elsewhere in the month.
func Fill(slots []int64, fill []bool, candidates []int64, prevTail []int64, r *rand.Rand) []int64 {
	out := slices.Clone(slots)
	if len(candidates) == 0 {
		return out
	}
	counts := map[int64]int{}
	for i, id := range out {
		if id != 0 && !fill[i] {
			counts[id]++
		}
	}
	lastIdx := map[int64]int{}
	for i := range out {
		if fill[i] {
			recent := neighbours(out, i, prevTail)
			id := rank(candidates, counts, recent, lastIdx, r)[0]
			out[i] = id
			counts[id]++
		}
		if out[i] != 0 {
			lastIdx[out[i]] = i
		}
	}
	return out
}

// Reroll picks a different recipe for slot i, preferring recipes not yet
// planned this month and not planned on the neighbouring days. It returns
// the current recipe if there is no alternative.
func Reroll(slots []int64, i int, candidates []int64, prevTail []int64, r *rand.Rand) int64 {
	cur := slots[i]
	others := make([]int64, 0, len(candidates))
	for _, id := range candidates {
		if id != cur {
			others = append(others, id)
		}
	}
	if len(others) == 0 {
		return cur
	}
	counts := map[int64]int{}
	for j, id := range slots {
		if j != i && id != 0 {
			counts[id]++
		}
	}
	recent := neighbours(slots, i, prevTail)
	if i+1 < len(slots) && slots[i+1] != 0 {
		recent = append(recent, slots[i+1])
	}
	return rank(others, counts, recent, map[int64]int{}, r)[0]
}

// neighbours returns the recipe ids of the recentWindow slots before i,
// reaching into prevTail when i is near the start of the month.
func neighbours(slots []int64, i int, prevTail []int64) []int64 {
	seq := append(slices.Clone(prevTail), slots[:i]...)
	if len(seq) > recentWindow {
		seq = seq[len(seq)-recentWindow:]
	}
	return seq
}
