package sender

import (
	"testing"
)

const (
	MaxQueryID = 8388606 // this is the max value unreachable through GetNext
)

func TestHighloadQueryID_SequentialGeneration(t *testing.T) {
	var err error
	q := NewHighloadQueryID()
	expectedQueryID := uint64(0)

	for i := 0; i < 10; i++ {
		qid := q.GetQueryID()
		if qid != expectedQueryID {
			t.Errorf("expected query_id %v, got %v", expectedQueryID, qid)
		}

		if !q.HasNext() {
			t.Fatal("unexpectedly ran out of query IDs")
		}

		q, err = q.GetNext()
		if err != nil {
			t.Fatal("unexpected error on next id generation")
		}
		expectedQueryID += 1
	}
}

func TestHighloadQueryID_FromQueryID(t *testing.T) {
	// Create from previous query ID
	q, err := FromQueryID(1)
	if err != nil {
		t.Fatal("unexpected error in FromQueryID")
	}

	if q.shift != 0 || q.bitnumber != 1 {
		t.Fatalf("expected shift=0 and bitnumber=1, got shift=%v bitnumber=%v", q.shift, q.bitnumber)
	}
}

func TestHighloadQueryID_Exhaustion(t *testing.T) {
	// Create a query ID close to the limit: MAX_SHIFT=8191, MAX_BITNUMBER=1022
	nearEnd := FromShiftAndBitNumber(8191, 1020)
	if !nearEnd.HasNext() {
		t.Fatal("should still have next query ID left")
	}

	// First GetNext() should succeed, 1021
	next, err := nearEnd.GetNext()
	if err != nil {
		t.Fatalf("unexpected error for GetNext: %v", err)
	}
	if next.shift != 8191 || next.bitnumber != 1021 {
		t.Fatalf("expected shift=8191 and bitnumber=1021, got shift=%v bitnumber=%v", next.shift, next.bitnumber)
	}
	if next.HasNext() {
		t.Fatal("should not allow next generation to keep one last emergency query ID left")
	}

	if next.GetQueryID() != MaxQueryID-1 {
		t.Fatalf("expected last query id to be %d, got: %d", MaxQueryID-1, next.GetQueryID())
	}

	// Last legal query ID, 1021
	_, err = next.GetNext()
	if err == nil {
		t.Fatal("expected the last GetNext to throw an error but it didn't happen")
	}

}
