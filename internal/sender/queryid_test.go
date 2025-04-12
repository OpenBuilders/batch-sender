package sender

import (
	"testing"
)

const (
	MaxQueryID = 8388605
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
	q, err := FromQueryID(MaxQueryID - 1)
	if err != nil {
		t.Fatal("unexpected error in FromQueryID")
	}

	final, err := q.GetNext()
	if err != nil {
		t.Fatalf("unexpected error for GetNext")
	}

	lastQueryID := final.GetQueryID()
	if lastQueryID != MaxQueryID {
		t.Fatalf("unexpected max query ID, want: %d, got: %d", MaxQueryID, lastQueryID)
	}
}

func TestHighloadQueryID_Exhaustion(t *testing.T) {
	// Create a query ID close to the limit: MAX_SHIFT=8191, MAX_BITNUMBER=1022
	nearEnd := FromShiftAndBitNumber(8191, 1019)

	// First GetNext() should succeed, 1020
	next, err := nearEnd.GetNext()
	if err != nil {
		t.Fatalf("unexpected error for GetNext: %v", err)
	}

	if next.shift != 8191 || next.bitnumber != 1020 {
		t.Fatalf("expected shift=8191 and bitnumber=1020, got shift=%v bitnumber=%v", next.shift, next.bitnumber)
	}
	if !next.HasNext() {
		t.Fatal("should still have one last emergency query ID left")
	}

	// Last legal query ID, 1021
	final, err := next.GetNext()
	if err != nil {
		t.Fatalf("unexpected error for GetNext")
	}
	if final.HasNext() {
		t.Fatal("should NOT have more query IDs after exhausting the range")
	}

	lastQueryID := final.GetQueryID()
	if lastQueryID != MaxQueryID {
		t.Fatalf("unexpected max query ID, want: %d, got: %d", MaxQueryID, lastQueryID)
	}

	_, err = final.GetNext()
	if err == nil {
		t.Fatal("expected the last GetNext to throw an error but it didn't happen")
	}
}
