package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

type Tile uint16 // 9-bit mask of the possible digits
type Board struct {
	Tiles [9 * 9]Tile

	// changeSet is a bit mask representing which tiles have changed.
	// Each row of regions is a uint32 (27 tiles, so 5 bytes unused).
	changeSet [3]uint32

	// changes is an array used as the backing store for the slice returned by
	// Changes(). This is to reduce heap allocations.
	changes [9 * 9]uint8
}

const tAny = Tile((1 << 9) - 1)

var byteToTileMap = map[byte]Tile{
	'1': 1 << 0, // 0b000000001
	'2': 1 << 1, // 0b000000010
	'3': 1 << 2, // 0b000000100
	'4': 1 << 3, // 0b000001000
	'5': 1 << 4, // 0b000010000
	'6': 1 << 5, // 0b000100000
	'7': 1 << 6, // 0b001000000
	'8': 1 << 7, // 0b010000000
	'9': 1 << 8, // 0b100000000
	'_': tAny,   // 0b111111111
}

// returns true if the tile only has a single possible value
func (t Tile) isKnown() bool {
	// http://graphics.stanford.edu/~seander/bithacks.html#DetermineIfPowerOf2
	return (t & (t - 1)) == 0
}

func (t Tile) Num() uint8 {
	if !t.isKnown() {
		return 0
	}
	// http://graphics.stanford.edu/~seander/bithacks.html#IntegerLogDeBruijn
	lookupTable := [32]uint8{
		0, 1, 28, 2, 29, 14, 24, 3, 30, 22, 20, 15, 25, 17, 4, 8,
		31, 27, 13, 23, 21, 19, 16, 7, 26, 12, 18, 6, 11, 5, 10, 9,
	}
	//TODO adjust the table to remove the <<1
	// the table is also larger than we need
	// it should also be global so it's not redeclared
	return lookupTable[(uint32(t<<1)*0x077CB531)>>27]
}

// converts board x,y coordinates into an index
func xyToIndex(x, y uint8) (idx uint8) {
	return y*9 + x
}

func indexToXY(idx uint8) (x, y uint8) {
	return idx % 9, idx / 9
}

//TODO bench indexToX/indexToY

func indexToRegionIndex(idx uint8) uint8 {
	return idx/3%3 + idx/(9*3)*3
}

var RegionIndices [9][9]uint8 = func() (idcs [9][9]uint8) {
	for ri := range idcs {
		idx0 := uint8((ri / 3 * 27) + (ri % 3 * 3))
		idcs[ri] = [9]uint8{
			idx0 + 9*0 + 0,
			idx0 + 9*0 + 1,
			idx0 + 9*0 + 2,
			idx0 + 9*1 + 0,
			idx0 + 9*1 + 1,
			idx0 + 9*1 + 2,
			idx0 + 9*2 + 0,
			idx0 + 9*2 + 1,
			idx0 + 9*2 + 2,
		}
	}
	return
}()

var RowIndices [9][9]uint8 = func() (idcs [9][9]uint8) {
	for y := range idcs {
		idx0 := uint8(y * 9)
		idcs[y] = [9]uint8{
			idx0 + 0,
			idx0 + 1,
			idx0 + 2,
			idx0 + 3,
			idx0 + 4,
			idx0 + 5,
			idx0 + 6,
			idx0 + 7,
			idx0 + 8,
		}
	}
	return
}()

var MaskBits [512][]uint8 = func() (mbs [512][]uint8) {
	for i := uint16(0); i < 512; i++ {
		for j := uint8(0); j < 9; j++ {
			if i&(1<<j) != 0 {
				mbs[i] = append(mbs[i], j)
			}
		}
	}
	return
}()

var ColumnIndices [9][9]uint8 = func() (idcs [9][9]uint8) {
	for x := range idcs {
		idcs[x] = [9]uint8{
			uint8(x) + 9*0,
			uint8(x) + 9*1,
			uint8(x) + 9*2,
			uint8(x) + 9*3,
			uint8(x) + 9*4,
			uint8(x) + 9*5,
			uint8(x) + 9*6,
			uint8(x) + 9*7,
			uint8(x) + 9*8,
		}
	}
	return
}()

func NewBoard() Board {
	return Board{Tiles: [81]Tile{
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
		tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny, tAny,
	}}
}

// Set tries to set the given indices to the given Tile.
// The tile set on the board might be different than the one provided if
// possiblities can be eliminated.
// Returns whether the operation was successful or not. The operation will be
// unsuccessful if the value results in an invalid board.
func (b *Board) Set(ti uint8, t Tile) bool {
	b0 := *b

	if !b.set(ti, t) {
		*b = b0
		return false
	}
	for b.HasChanges() {
		if !b.evaluateAlgorithms() {
			*b = b0
			return false
		}
	}
	return true
}

func (b *Board) set(ti uint8, t Tile) bool {
	t0 := b.Tiles[ti]

	// discard possible values based on the current tile mask
	t &= t0
	if t == 0 {
		// not possible captain
		return false
	}

	if t == t0 {
		// no change
		return true
	}

	b.Tiles[ti] = t
	b.changeSet[ti/27] |= 1 << (ti % 27)

	return true
}

func (b *Board) HasChanges() bool {
	return b.changeSet[0] != 0 || b.changeSet[1] != 0 || b.changeSet[2] != 0
}
func (b *Board) ClearChanges() {
	b.changeSet[0] = 0
	b.changeSet[1] = 0
	b.changeSet[2] = 0
}
func (b *Board) Changes() []uint8 {
	changes := b.changes[:0]
	for rri, rrm := range b.changeSet {
		for i := uint8(0); i < 27; i++ {
			if rrm&1 != 0 {
				changes = append(changes, uint8(rri)*27+i)
			}
			rrm = rrm >> 1
		}
	}
	return changes
}

func (b *Board) evaluateAlgorithms() bool {
	type algoFunc func([]uint8) bool
	algoFuncs := []algoFunc{
		b.algoKnownValueElimination,
		b.algoOnePossibleTile,
		b.algoOnlyRow,
		b.algoNakedSubset,
		b.algoHiddenSubset,
	}

	cs := b.changeSet
AlgorithmsLoop:
	for b.HasChanges() {
		for _, af := range algoFuncs {
			changes := b.Changes()
			b.ClearChanges()
			if !af(changes) {
				return false
			}

			if b.HasChanges() {
				// add any changes just made to the backed-up changeset since the next algo
				// hasn't seen them yet.
				cs[0] |= b.changeSet[0]
				cs[1] |= b.changeSet[1]
				cs[2] |= b.changeSet[2]

				// restart from the first algorithm
				continue AlgorithmsLoop
			}
			// no changes, restore the change set for the next algo
			b.changeSet = cs
		}
		// we made it through all the algorithms with no changes
		b.ClearChanges()
		break
	}

	return true
}

// algoKnownValueElimination looks for tiles which have a known value. If any
// are found remove that value as a possibility from its neighbors.
func (b *Board) algoKnownValueElimination(changes []uint8) bool {
	for _, ti := range changes {
		t := b.Tiles[ti]
		if !t.isKnown() {
			continue
		}

		x, y := indexToXY(ti)
		rgnIdx := indexToRegionIndex(ti)
		rgnIndices := RegionIndices[rgnIdx][:]
		rowIndices := RowIndices[y][:]
		colIndices := ColumnIndices[x][:]

		// iterate over the region
		for _, nti := range rgnIndices {
			if nti == ti {
				// skip ourself
				continue
			}
			if !b.set(nti, b.Tiles[nti]&^t) {
				// invalid board configuration
				return false
			}
		}

		// iterate over the row
		for _, nti := range rowIndices {
			if nti == ti {
				// skip ourself
				continue
			}
			if !b.set(nti, b.Tiles[nti]&^t) {
				// invalid board configuration
				return false
			}
		}

		// iterate over the column
		for _, nti := range colIndices {
			if nti == ti {
				// skip ourself
				continue
			}
			if !b.set(nti, b.Tiles[nti]&^t) {
				// invalid board configuration
				return false
			}
		}
	}

	return true
}

// algoOnePossibleTile scans each set of neighbors for any values which have
// only one possible tile.
func (b *Board) algoOnePossibleTile(changes []uint8) bool {
	var regionsSeen uint16
	var rowsSeen uint16
	var columnsSeen uint16

	for _, ti := range changes {
		x, y := indexToXY(ti)
		rgnIdx := indexToRegionIndex(ti)

		// Iterate over the region.
		// But first, see if we've already done so for this specific region.
		regionMask := uint16(1 << rgnIdx)
		if regionsSeen&regionMask == 0 {
			regionsSeen |= regionMask

			rgnIndices := RegionIndices[rgnIdx][:]
		OnePossibleTileRegionLoop:
			for v := Tile(1); v < tAny; v = v << 1 {
				//TODO this feels like there should have an optimized way to find which bits are set in only one of a set of numbers
				tcIdx := uint8(255)
				for _, nti := range rgnIndices {
					nt := b.Tiles[nti]
					if nt == v {
						// this value already has been found
						continue OnePossibleTileRegionLoop
					}
					if nt&v == 0 {
						// not a possible tile
						continue
					}
					// is a candidate
					if tcIdx != 255 {
						// this is the second candidate
						continue OnePossibleTileRegionLoop
					}
					tcIdx = nti
				}
				if tcIdx == 255 {
					// no possible tiles for this value
					//TODO does this ever happen?
					return false
				}
				if !b.set(tcIdx, v) {
					// invalid board configuration
					//TODO does this ever happen?
					return false
				}
			}
		}

		// Now iterate over the row.
		// Again, seeing if we've already done so.
		rowMask := uint16(1 << y)
		if rowsSeen&rowMask == 0 {
			rowsSeen |= rowMask

			rowIndices := RowIndices[y][:]
		OnePossibleTileRowLoop:
			for v := Tile(1); v < tAny; v = v << 1 {
				tcIdx := uint8(255)
				for _, nti := range rowIndices {
					nt := b.Tiles[nti]
					if nt == v {
						continue OnePossibleTileRowLoop
					}
					if nt&v == 0 {
						continue
					}
					if tcIdx != 255 {
						continue OnePossibleTileRowLoop
					}
					tcIdx = nti
				}
				if tcIdx == 255 {
					return false
				}
				if !b.set(tcIdx, v) {
					return false
				}
			}
		}

		// And now the column.
		columnMask := uint16(1 << x)
		if columnsSeen&columnMask == 0 {
			columnsSeen |= columnMask

			colIndices := ColumnIndices[x][:]
		OnePossibleTileColumnLoop:
			for v := Tile(1); v < tAny; v = v << 1 {
				tcIdx := uint8(255)
				for _, nti := range colIndices {
					nt := b.Tiles[nti]
					if nt == v {
						continue OnePossibleTileColumnLoop
					}
					if nt&v == 0 {
						continue
					}
					if tcIdx != 255 {
						continue OnePossibleTileColumnLoop
					}
					tcIdx = nti
				}
				if tcIdx == 255 {
					return false
				}
				if !b.set(tcIdx, v) {
					return false
				}
			}
		}
	}

	return true
}

// algoOnlyRow checks if there is only a single row or column within a region
// which can hold a value. If so, it eliminates the value from the
// possibilities within the same row/column of neighboring regions.
func (b *Board) algoOnlyRow(changes []uint8) bool {
	var regionsSeen uint16
	for _, ti := range changes {
		// skip any regions we've already seen this round
		rgnIdx := indexToRegionIndex(ti)
		regionMask := uint16(1 << rgnIdx)
		if regionsSeen&regionMask != 0 {
			continue
		}
		regionsSeen |= regionMask

		rgnIndices := RegionIndices[rgnIdx][:]

		// row first
	OnePossibleRowLoop:
		for v := Tile(1); v < tAny; v = v << 1 {
			tcRow := uint8(255)
			for _, nti := range rgnIndices {
				nt := b.Tiles[nti]
				if nt == v {
					// this value has already been found
					continue OnePossibleRowLoop
				}
				if nt&v == 0 {
					// not a possible tile
					continue
				}
				_, y := indexToXY(nti)
				if tcRow == y {
					// row already a candidate
					continue
				}
				if tcRow != 255 {
					// multiple candidate rows
					continue OnePossibleRowLoop
				}
				tcRow = y
			}
			if tcRow == 255 {
				// no candidate rows. Wat?
				return false
			}

			// iterate over the candidate row, excluding the value from tiles in other regions
			for _, nti := range RowIndices[tcRow][:] {
				if indexToRegionIndex(nti) == rgnIdx {
					// skip our region
					continue
				}
				if !b.set(nti, b.Tiles[nti]&^v) {
					// invalid board configuration
					return false
				}
			}
		}

	OnePossibleColumnLoop:
		for v := Tile(1); v < tAny; v = v << 1 {
			tcCol := uint8(255)
			for _, nti := range rgnIndices {
				nt := b.Tiles[nti]
				if nt == v {
					// this value has already been found
					continue OnePossibleColumnLoop
				}
				if nt&v == 0 {
					// not a possible tile
					continue
				}
				x, _ := indexToXY(nti)
				if tcCol == x {
					// column already a candidate
					continue
				}
				if tcCol != 255 {
					// multiple candidate columns
					continue OnePossibleColumnLoop
				}
				tcCol = x
			}
			if tcCol == 255 {
				// no candidate columns. Wat?
				return false
			}

			for _, nti := range ColumnIndices[tcCol][:] {
				if indexToRegionIndex(nti) == rgnIdx {
					// skip our region
					continue
				}
				if !b.set(nti, b.Tiles[nti]&^v) {
					// invalid board configuration
					return false
				}
			}
		}
	}

	return true
}

// algoNakedSubset finds any tiles within a neighbor set for which the number of
// possible values within the tile is the same as the number of tiles with the
// same possible values, and eliminates the values in those tiles from all other
// tiles within the set.
//
// https://www.kristanix.com/sudokuepic/sudoku-solving-techniques.php "Naked Subset"
//
func (b *Board) algoNakedSubset(changes []uint8) bool {
	var regionsSeen uint16
	var rowsSeen uint16
	var columnsSeen uint16

	for _, ti := range changes {
		rgnIdx := indexToRegionIndex(ti)
		x, y := indexToXY(ti)

		// first scan the region
		regionMask := uint16(1 << rgnIdx)
		if regionsSeen&regionMask == 0 {
			regionsSeen |= regionMask

			rgnIndices := RegionIndices[rgnIdx][:]
			rgnSetCounts := map[Tile]uint8{}
			for _, nti := range rgnIndices {
				nt := b.Tiles[nti]
				if nt.isKnown() {
					// Technically this is one such case. One possible value within the tile,
					// and one tile with this possible set. But this is already handled by
					// algoOnePossibleTile.
					continue
				}

				rgnSetCounts[nt]++
			}
			for t, setCount := range rgnSetCounts {
				possibilityCount := uint8(len(MaskBits[t]))
				if possibilityCount != setCount {
					continue
				}
				// if we're here, then we have a combination of N tiles with N possibilities.
				for _, nti := range rgnIndices {
					nt := b.Tiles[nti]
					if nt == t {
						// this is one of the N tiles
						continue
					}
					if nt&t != 0 {
						// this tile has some of the possibilities, remove them
						if !b.set(nti, nt&^t) {
							return false
						}
					}
				}
			}
		}

		// now scan the row
		rowMask := uint16(1 << y)
		if rowsSeen&rowMask == 0 {
			rowsSeen |= rowMask

			rowIndices := RowIndices[y][:]
			rowSetCounts := map[Tile]uint8{}
			for _, nti := range rowIndices {
				nt := b.Tiles[nti]
				if nt.isKnown() {
					// Technically this is one such case. One possible value within the tile,
					// and one tile with this possible set. But as we already know the value we
					// don't need to do anything.
					continue
				}

				rowSetCounts[nt]++
			}
			for t, setCount := range rowSetCounts {
				possibilityCount := uint8(len(MaskBits[t]))
				if possibilityCount != setCount {
					continue
				}
				// if we're here, then we have a combination of N tiles with N possibilities.
				for _, nti := range rowIndices {
					nt := b.Tiles[nti]
					if nt == t {
						// this is one of the N tiles
						continue
					}
					if nt&t != 0 {
						// this tile has some of the possibilities, remove them
						if !b.set(nti, nt&^t) {
							return false
						}
					}
				}
			}
		}

		// now scan the column
		columnMask := uint16(1 << x)
		if columnsSeen&columnMask == 0 {
			columnsSeen |= columnMask

			colIndices := ColumnIndices[x][:]
			colSetCounts := map[Tile]uint8{}
			for _, nti := range colIndices {
				nt := b.Tiles[nti]
				if nt.isKnown() {
					// Technically this is one such case. One possible value within the tile,
					// and one tile with this possible set. But as we already know the value we
					// don't need to do anything.
					continue
				}

				colSetCounts[nt]++
			}
			for t, setCount := range colSetCounts {
				possibilityCount := uint8(len(MaskBits[t]))
				if possibilityCount != setCount {
					continue
				}
				// if we're here, then we have a combination of N tiles with N possibilities.
				for _, nti := range colIndices {
					nt := b.Tiles[nti]
					if nt == t {
						// this is one of the N tiles
						continue
					}
					if nt&t != 0 {
						// this tile has some of the possibilities, remove them
						if !b.set(nti, nt&^t) {
							return false
						}
					}
				}
			}
		}
	}

	return true
}

// algoHiddenSubset finds all subsets which have only the same number of possible tiles as the number of possible values within the set.
// Think of 2 tiles with possiblities [1,4,7] and [1,4,9], where these are the only to tiles to contain possibilities for 1 & 4. Because of that, we can exempt 7 & 9 from the possibilities of these 2 tiles.
// Likewise for [1,4,7,9],[1,3,4,7],[1,2,4,7], if no other tile has 1, 4, or 7, we can set all 3 tiles to [1,4,7].
func (b *Board) algoHiddenSubset(changes []uint8) bool {
	// the algorithm works like this:
	// 1. Iterate over the values 1-9
	// 1.1. Find each tile which can hold that value.
	// 2. Group the values together which have the same candidate tiles.
	// 2.1 If the number of grouped values is the same as the number of candidate
	//     tiles, that is a hidden subset.
	// 2.2 Remove all other possible values from the candidate tiles.

	//TODO this algorithm chews up a lot of CPU time. Implementing it makes the solver about 10x slower.

	var regionsSeen uint16
	var rowsSeen uint16
	var columnsSeen uint16

	for _, ti := range changes {
		x, y := indexToXY(ti)
		rgnIdx := indexToRegionIndex(ti)

		// iterate over the region
		regionMask := uint16(1 << rgnIdx)
		if regionsSeen&regionMask == 0 {
			regionsSeen |= regionMask

			// valueTileIndices is a list of values to a bit mask of tile indices which hold that value.
			// E.G. `3 => 0b001000010` means that the value 3 is a possibility for tiles 2 & 7.
			valueTileIndices := [9]uint16{}
			rgnIndices := RegionIndices[rgnIdx][:]
			// 1. Iterate over the values 1-9
			for v := uint8(0); v < 9; v++ { // v is one less than the actual number we're dealing with
				// 1.1. Find each tile which can hold that value.
				for i, nti := range rgnIndices {
					nt := b.Tiles[nti]
					if nt&(1<<v) == 0 {
						continue
					}
					valueTileIndices[v] |= 1 << uint16(i)
				}
			}

			// 2. Group the values together which have the same candidate tiles.
			// We basically reverse the valueTileIndices list.
			// sets is a map of a set of indices (as a bit mask) to a bit mask of the
			// values in that set.
			// E.G. `0b001000010 => 0b000000101` means that tiles 2 & 7 are both the only
			// candidates for values 1 & 3.
			sets := make(map[uint16]Tile, 9)
			for v, rtiMask := range valueTileIndices[:] {
				sets[rtiMask] |= 1 << uint8(v)
			}

			// 2.1 If the number of grouped values is the same as the number of candidate
			// tiles, that is a hidden subset.
			for rtiMask, valuesMask := range sets {
				// break the tile indicies bitmask out into separate indicies
				tileIndices := MaskBits[rtiMask]

				valuesCount := len(MaskBits[valuesMask])
				if valuesCount != len(tileIndices) {
					// not a hidden subset
					continue
				}

				// 2.2 Remove all other possible values from the candidate tiles.
				for _, rti := range tileIndices {
					if !b.set(rgnIndices[rti], valuesMask) {
						return false
					}
				}
			}
		}

		// iterate over the row
		rowMask := uint16(1 << y)
		if rowsSeen&rowMask == 0 {
			rowsSeen |= rowMask

			// valueTileIndices is a list of values to a bit mask of tile indices which hold that value.
			// E.G. `3 => 0b001000010` means that the value 3 is a possibility for tiles 2 & 7.
			valueTileIndices := [9]uint16{}
			rowIndices := RowIndices[y][:]
			// 1. Iterate over the values 1-9
			for v := uint8(0); v < 9; v++ { // v is one less than the actual number we're dealing with
				// 1.1. Find each tile which can hold that value.
				for i, nti := range rowIndices {
					nt := b.Tiles[nti]
					if nt&(1<<v) == 0 {
						continue
					}
					valueTileIndices[v] |= 1 << uint16(i)
				}
			}

			// 2. Group the values together which have the same candidate tiles.
			// We basically reverse the valueTileIndices list.
			// sets is a map of a set of indices (as a bit mask) to a bit mask of the
			// values in that set.
			// E.G. `0b001000010 => 0b000000101` means that tiles 2 & 7 are both the only
			// candidates for values 1 & 3.
			sets := make(map[uint16]Tile, 9)
			for v, rtiMask := range valueTileIndices {
				sets[rtiMask] |= 1 << uint8(v)
			}

			// 2.1 If the number of grouped values is the same as the number of candidate
			// tiles, that is a hidden subset.
			for rtiMask, valuesMask := range sets {
				// break the tile indicies bitmask out into separate indicies
				tileIndices := MaskBits[rtiMask]

				valuesCount := len(MaskBits[valuesMask])
				if valuesCount != len(tileIndices) {
					// not a hidden subset
					continue
				}

				// 2.2 Remove all other possible values from the candidate tiles.
				for _, rti := range tileIndices {
					if !b.set(rowIndices[rti], valuesMask) {
						return false
					}
				}
			}
		}

		// iterate over the column
		colMask := uint16(1 << x)
		if columnsSeen&colMask == 0 {
			columnsSeen |= colMask

			// valueTileIndices is a list of values to a bit mask of tile indices which hold that value.
			// E.G. `3 => 0b001000010` means that the value 3 is a possibility for tiles 2 & 7.
			valueTileIndices := [9]uint16{}
			colIndices := ColumnIndices[x][:]
			// 1. Iterate over the values 1-9
			for v := uint8(0); v < 9; v++ { // v is one less than the actual number we're dealing with
				// 1.1. Find each tile which can hold that value.
				for i, nti := range colIndices {
					nt := b.Tiles[nti]
					if nt&(1<<v) == 0 {
						continue
					}
					valueTileIndices[v] |= 1 << uint16(i)
				}
			}

			// 2. Group the values together which have the same candidate tiles.
			// We basically reverse the valueTileIndices list.
			// sets is a map of a set of indices (as a bit mask) to a bit mask of the
			// values in that set.
			// E.G. `0b001000010 => 0b000000101` means that tiles 2 & 7 are both the only
			// candidates for values 1 & 3.
			sets := make(map[uint16]Tile, 9)
			for v, ctiMask := range valueTileIndices {
				sets[ctiMask] |= 1 << uint8(v)
			}

			// 2.1 If the number of grouped values is the same as the number of candidate
			// tiles, that is a hidden subset.
			for ctiMask, valuesMask := range sets {
				// break the tile indicies bitmask out into separate indicies
				tileIndices := MaskBits[ctiMask]

				valuesCount := len(MaskBits[valuesMask])
				if valuesCount != len(tileIndices) {
					// not a hidden subset
					continue
				}

				// 2.2 Remove all other possible values from the candidate tiles.
				for _, cti := range tileIndices {
					if !b.set(colIndices[cti], valuesMask) {
						return false
					}
				}
			}
		}
	}
	return true
}

func (b *Board) ReadFrom(r io.Reader) (int64, error) {
	var ba [9 * 9 * 2]byte
	nr, err := io.ReadAtLeast(r, ba[:], len(ba)-1)
	// io.ReadAtLeast() because we don't care about a trailing newline if it's there
	if err != nil {
		return int64(nr), err
	}
	for i := 0; i < len(ba); i += 2 {
		x := uint8(i / 2 % 9)
		y := uint8(i / 2 / 9)
		ti := xyToIndex(x, y)
		t := byteToTileMap[ba[i]]
		if t == 0 {
			return int64(nr), errors.New("invalid byte")
		}
		if !b.set(ti, t) {
			return int64(nr), fmt.Errorf("invalid board (offset=%d byte=%q)", i, []byte{ba[i]})
		}
		//b.Tiles[ri][ti] = byteToTileMap[ba[i]]
	}

	for b.HasChanges() {
		if !b.evaluateAlgorithms() {
			return int64(nr), fmt.Errorf("invalid board")
		}
	}

	return int64(nr), nil
}

func (b Board) Art() [9 * 9 * 2]byte {
	var ba [9 * 9 * 2]byte
	for y := uint8(0); y < 9; y++ {
		rowStart := y * 9 * 2
		for x, ti := range RowIndices[y][:] {
			t := b.Tiles[ti]
			i := rowStart + uint8(x)*2
			ba[i] = '0' + t.Num()
			if ba[i] == '0' {
				ba[i] = '_'
			}
			ba[i+1] = ' '
		}
		ba[rowStart+8*2+1] = '\n'
	}
	return ba
}

func main() {
	b := NewBoard()
	b.ReadFrom(os.Stdin)
	fmt.Printf("%s\n", b.Art())
}
