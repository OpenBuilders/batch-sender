package sender

import (
	"errors"
)

const BIT_NUMBER_SIZE = 10  // 10 bits
const SHIFT_SIZE = 13       // 13 bits
const MAX_BIT_NUMBER = 1022 // (2^10) - 1
const MAX_SHIFT = 8191      // (2^13) - 1

type HighloadQueryID struct {
	shift     uint64 // [0 .. 8191]
	bitnumber uint64 // [0 .. 1022]
}

func NewHighloadQueryID() *HighloadQueryID {
	return &HighloadQueryID{
		shift:     0,
		bitnumber: 0,
	}
}

func (h *HighloadQueryID) GetNext() (*HighloadQueryID, error) {
	newBitnumber := h.bitnumber + 1
	newShift := h.shift

	if newShift == MAX_SHIFT && newBitnumber > MAX_BIT_NUMBER-1 {
		return nil, errors.New("overload: cannot generate more query_ids")
	}

	if newBitnumber > MAX_BIT_NUMBER {
		newBitnumber = 0
		newShift += 1
		if newShift > MAX_SHIFT {
			return nil, errors.New("overload: cannot generate more query_ids")
		}
	}

	return &HighloadQueryID{
		shift:     newShift,
		bitnumber: newBitnumber,
	}, nil
}

func (h *HighloadQueryID) HasNext() bool {
	return !(h.bitnumber >= MAX_BIT_NUMBER-1 && h.shift == MAX_SHIFT)
}

func (h *HighloadQueryID) GetShift() uint64 {
	return h.shift
}

func (h *HighloadQueryID) GetBitNumber() uint64 {
	return h.bitnumber
}

func (h *HighloadQueryID) GetQueryID() uint64 {
	return (h.shift << BIT_NUMBER_SIZE) + h.bitnumber
}

func FromQueryID(queryID uint64) (*HighloadQueryID, error) {
	shift := queryID >> BIT_NUMBER_SIZE
	bitnumber := queryID & MAX_BIT_NUMBER
	if bitnumber > MAX_BIT_NUMBER {
		return nil, errors.New("invalid queryID format")
	}
	return &HighloadQueryID{
		shift:     shift,
		bitnumber: bitnumber,
	}, nil
}

func FromSeqno(seqno uint64) *HighloadQueryID {
	shift := seqno / MAX_BIT_NUMBER
	bitnumber := seqno % MAX_BIT_NUMBER
	return &HighloadQueryID{
		shift:     shift,
		bitnumber: bitnumber,
	}
}

// FromShiftAndBitNumber safely creates a new HighloadQueryID
func FromShiftAndBitNumber(shift, bitnumber uint64) *HighloadQueryID {
	if shift < 0 || shift > MAX_SHIFT {
		panic(errors.New("invalid shift: must be in [0, 8191]"))
	}
	if bitnumber < 0 || bitnumber > MAX_BIT_NUMBER {
		panic(errors.New("invalid bitnumber: must be in [0, 1022]"))
	}
	return &HighloadQueryID{
		shift:     shift,
		bitnumber: bitnumber,
	}
}

func (h *HighloadQueryID) ToSeqno() uint64 {
	return h.bitnumber + h.shift*MAX_BIT_NUMBER
}
