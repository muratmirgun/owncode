package message

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/muratmirgun/owncode/internal/db"
	"github.com/stretchr/testify/require"
)

type bufferDB struct {
	db.Querier
	mu     sync.Mutex
	writes []db.UpdateMessageParams
	fail   bool
}

func (q *bufferDB) UpdateMessage(_ context.Context, p db.UpdateMessageParams) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.fail {
		return errors.New("database unavailable")
	}
	q.writes = append(q.writes, p)
	return nil
}

func (q *bufferDB) GetMessage(_ context.Context, id string) (db.Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := len(q.writes) - 1; i >= 0; i-- {
		if q.writes[i].ID == id {
			return db.Message{ID: id, Parts: q.writes[i].Parts}, nil
		}
	}
	return db.Message{}, errors.New("missing message")
}

func (q *bufferDB) snapshot() []db.UpdateMessageParams {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.writes)
}

func TestStreamStorageCoalescesBeforeWrites(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := &bufferDB{}
		s := NewService(q).(*service)
		ctx := context.Background()
		for i := range 1000 {
			for agent := range 4 {
				msg := Message{ID: fmt.Sprint(agent), Role: Assistant, SessionID: fmt.Sprint(agent), Parts: []ContentPart{TextContent{Text: fmt.Sprint(i)}}}
				require.NoError(t, s.Update(ctx, msg))
				msg.Parts[0] = TextContent{Text: "mutated caller"}
			}
		}
		require.Empty(t, q.snapshot())
		time.Sleep(updateInterval)
		synctest.Wait()
		require.Len(t, q.snapshot(), 4)
		for _, write := range q.snapshot() {
			require.Contains(t, write.Parts, "999")
		}
		require.NoError(t, s.Close(ctx))
	})
}

func TestStorageFlushBoundariesAndRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := &bufferDB{}
		s := NewService(q).(*service)
		ctx := context.Background()
		msg := Message{ID: "a", SessionID: "session", Role: Assistant, Parts: []ContentPart{TextContent{Text: "first"}}}
		require.NoError(t, s.Update(ctx, msg))
		got, err := s.Get(ctx, msg.ID)
		require.NoError(t, err)
		require.Equal(t, "first", got.Content().Text)
		msg.Parts = append(msg.Parts, ToolCall{ID: "tool", Name: "view"})
		require.NoError(t, s.Update(ctx, msg))
		require.Len(t, q.snapshot(), 2)
		msg.Parts = append(msg.Parts, Finish{Reason: FinishReasonCanceled, Time: 123})
		q.fail = true
		require.Error(t, s.Update(ctx, msg))
		q.fail = false
		require.NoError(t, s.FlushAll(ctx))
		require.Len(t, q.snapshot(), 3)
		require.EqualValues(t, 123, q.snapshot()[2].FinishedAt.Int64)
		time.Sleep(time.Second)
		synctest.Wait()
		require.Len(t, q.snapshot(), 3, "a stale timer must not overwrite completion")
		require.NoError(t, s.Close(ctx))
		require.Error(t, s.Update(ctx, msg))
	})
}

func TestShutdownPersistsCanceledStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := &bufferDB{}
		s := NewService(q).(*service)
		ctx, cancel := context.WithCancel(context.Background())
		require.NoError(t, s.Update(ctx, Message{ID: "a", Role: Assistant, Parts: []ContentPart{TextContent{Text: "last delta"}}}))
		cancel()
		require.NoError(t, s.Close(context.Background()))
		require.Len(t, q.snapshot(), 1)
		require.Contains(t, q.snapshot()[0].Parts, "last delta")
	})
}
