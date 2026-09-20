package tui

import (
	"github.com/muratmirgun/owncode/internal/message"
	"github.com/muratmirgun/owncode/internal/pubsub"
)

// MessageBatchMsg applies stream snapshots before Bubble Tea renders one frame.
type MessageBatchMsg []pubsub.Event[message.Message]
