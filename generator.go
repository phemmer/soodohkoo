package main

import (
	"math/rand"
	"sort"
	"time"
)

type algoGenerateShuffle struct {
	rand *rand.Rand
}

func (a algoGenerateShuffle) Name() string { return "algoGenerateShuffle" }

func (a algoGenerateShuffle) Stats() *AlgorithmStats { return &AlgorithmStats{} }

func (a algoGenerateShuffle) EvaluateChanges(b *Board, changes []uint8) bool {
	// guess() picks the first value from a set in MaskBits. This would result in a
	// very non-random board. So we shuffle the MaskBits around each loop through
	// evaluateAlgorithms.
	// This is a little heavy as we're shuffling the entire MaskBits, when we
	// really just need to shuffle a single set. But this won't be called often, so
	// we go with simplicity.
	for _, mbs := range MaskBits[:] {
		// Fisher–Yates shuffle.
		for i := len(mbs) - 1; i > 0; i-- {
			j := a.rand.Intn(i + 1)
			mbs[i], mbs[j] = mbs[j], mbs[i]
		}
	}
	return true
}

func NewRandomBoard(difficulty int) Board {
	b := NewBoard()

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	algos := b.Algorithms
	b.Algorithms = []Algorithm{
		&algoKnownValueElimination{},
		&algoOnePossibleTile{},
		&algoOnlyRow{},
		&algoGenerateShuffle{rng},
	}

	b.Set(0, 1<<uint(rng.Intn(9)))
	b.guess()

	// We now have a fully filled out board.
	// Drop some values.
	for i := 0; i < difficulty; i++ {
		if !b.dropRandomTile(rng) {
			break
		}
	}

	b.Algorithms = algos
	return b
}

type dropCandidate struct {
	ti    uint8
	score int
}
type dropCandidates []dropCandidate

func (dcs dropCandidates) Len() int           { return len(dcs) }
func (dcs dropCandidates) Less(i, j int) bool { return dcs[i].score < dcs[j].score }
func (dcs dropCandidates) Swap(i, j int)      { dcs[i], dcs[j] = dcs[j], dcs[i] }
func (dcs dropCandidates) Sort()              { sort.Sort(dcs) }
func (dcs *dropCandidates) Remove(i int)      { *dcs = append((*dcs)[:i], (*dcs)[i+1:]...) }
func (dcs dropCandidates) Shuffle() {
	for i := len(dcs) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		dcs[i], dcs[j] = dcs[j], dcs[i]
	}
}

// dropRandomTile drops a random tile from the board.
// If no further tiles can be dropped without resulting in a board with multiple
// solutions, it returns false;
func (b *Board) dropRandomTile(rng *rand.Rand) bool {
	dcs := dropCandidates{}
	for ti, t := range b.Tiles {
		if !t.isKnown() {
			continue
		}
		ti := uint8(ti)

		// score is the number of possible values in neighboring tiles
		score := 0

		rgnIdx := indexToRegionIndex(ti)
		for _, nti := range RegionIndices[rgnIdx][:] {
			score += len(MaskBits[b.Tiles[nti]])
		}

		colIdx, rowIdx := indexToXY(ti)
		for _, nti := range RowIndices[rowIdx][:] {
			score += len(MaskBits[b.Tiles[nti]])
		}
		for _, nti := range ColumnIndices[colIdx][:] {
			score += len(MaskBits[b.Tiles[nti]])
		}

		dcs = append(dcs, dropCandidate{ti: ti, score: score})
	}

	dcs.Shuffle()
	dcs.Sort()

	for dcs.Len() > 0 {
		dc := dcs[0]
		dcs.Remove(0)
		ti := dc.ti

		// Try to solve the board with the current value excluded as a possibility.
		// If we have a solution, then clearing this tile would result in a board with
		// multiple solutions. So retry with a different tile.
		bTest := *b
		bTest.Tiles[ti] = (^bTest.Tiles[ti]) & tAny
		bTest.changeSet[ti/27] |= 1 << (ti % 27)
		if bTest.Solve() {
			// Have multiple solutions. Try again
			continue
		}
		// Still just a single solution, so we're good to remove this tile.
		b.Tiles[ti] = tAny
		return true
	}
	return false
}
