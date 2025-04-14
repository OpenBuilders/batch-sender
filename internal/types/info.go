package types

import (
	"time"

	"github.com/google/uuid"
)

// ExtMsgInfo contains information from the body of an external message.
type ExtMsgInfo struct {
	// UUID is based on the first 16 byptes of the hash of the message body.
	UUID uuid.UUID
	// Specifc query ID used when the message has been done.
	QueryID uint64
	// Timestamp when the message is created as returned by the MessageBuilder.
	CreatedAt time.Time
	// Expiration of the message (CreatedAt + TTL)
	ExpiredAt time.Time
	// TTL of the message in the highload wallet as defined in the contract.
	TTL uint64
}
