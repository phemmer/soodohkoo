package main

import "time"

type Algorithm interface {
	Name() string
	EvaluateChanges(*Board, []uint8) bool
	Stats() *AlgorithmStats
}

type AlgorithmStats struct {
	Calls    uint
	Changes  uint
	Duration time.Duration
}

// algoKnownValueElimination looks for tiles which have a known value. If any
// are found, remove that value as a possibility from its neighbors.
type algoKnownValueElimination struct {
	AlgoStats AlgorithmStats
}

func (a algoKnownValueElimination) Name() string { return "algoKnownValueElimination" }

func (a *algoKnownValueElimination) Stats() *AlgorithmStats { return &a.AlgoStats }

func (a algoKnownValueElimination) EvaluateChanges(b *Board, changes []uint8) bool {
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
			if !b.set(nti, ^t) {
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
			if !b.set(nti, ^t) {
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
			if !b.set(nti, ^t) {
				// invalid board configuration
				return false
			}
		}
	}

	return true
}

// algoOnePossibleTile scans each set of neighbors for any values which have
// only one possible tile.
type algoOnePossibleTile struct {
	AlgoStats AlgorithmStats
}

func (a algoOnePossibleTile) Name() string { return "algoOnePossibleTile" }

func (a *algoOnePossibleTile) Stats() *AlgorithmStats { return &a.AlgoStats }

func (a algoOnePossibleTile) EvaluateChanges(b *Board, changes []uint8) bool {
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
				//TODO this feels like there should be an optimized way to find which bits are set in only one of a set of numbers
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
type algoOnlyRow struct {
	AlgoStats AlgorithmStats
}

func (a algoOnlyRow) Name() string { return "algoOnlyRow" }

func (a *algoOnlyRow) Stats() *AlgorithmStats { return &a.AlgoStats }

func (a algoOnlyRow) EvaluateChanges(b *Board, changes []uint8) bool {
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
				if !b.set(nti, ^v) {
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
				if !b.set(nti, ^v) {
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
type algoNakedSubset struct {
	AlgoStats AlgorithmStats
}

func (a algoNakedSubset) Name() string { return "algoNakedSubset" }

func (a *algoNakedSubset) Stats() *AlgorithmStats { return &a.AlgoStats }

func (a algoNakedSubset) EvaluateChanges(b *Board, changes []uint8) bool {
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
						if !b.set(nti, ^t) {
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
						if !b.set(nti, ^t) {
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
						if !b.set(nti, ^t) {
							return false
						}
					}
				}
			}
		}
	}

	return true
}

// algoHiddenSubset finds all subsets which have only the same number of
// possible tiles as the number of possible values within the set.
// Think of 2 tiles with possiblities [1,4,7] and [1,4,9], where these are the
// only to tiles to contain possibilities for 1 & 4. Because of that, we can
// exempt 7 & 9 from the possibilities of these 2 tiles.
// Likewise for [1,4,7,9],[1,3,4,7],[1,2,4,7], if no other tile has 1, 4, or 7,
// we can set all 3 tiles to [1,4,7].
type algoHiddenSubset struct {
	AlgoStats AlgorithmStats
}

func (a algoHiddenSubset) Name() string { return "algoHiddenSubset" }

func (a *algoHiddenSubset) Stats() *AlgorithmStats { return &a.AlgoStats }

func (a algoHiddenSubset) EvaluateChanges(b *Board, changes []uint8) bool {
	// the algorithm works like this:
	// 1. Iterate over the values 1-9
	// 1.1. Find each tile which can hold that value.
	// 2. Group the values together which have the same candidate tiles.
	// 2.1 If the number of grouped values is the same as the number of candidate
	//     tiles, that is a hidden subset.
	// 2.2 Remove all other possible values from the candidate tiles.

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
