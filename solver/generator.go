package main

import (
	"math/rand"
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

// dropRandomTile drops a random tile from the board.
// If no further tiles can be dropped without resulting in a board with multiple
// solutions, it returns false;
func (b *Board) dropRandomTile(rng *rand.Rand) bool {
	tis := []uint8{}
	for ti, t := range b.Tiles {
		if t.isKnown() {
			tis = append(tis, uint8(ti))
		}
	}
	for len(tis) > 0 {
		i := rng.Intn(len(tis))
		ti := tis[i]
		tis = append(tis[:i], tis[i+1:]...)

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
