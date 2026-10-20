package rpc

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/wire"
)

// The final answer block is kept only when the context asks for it
// (chat_ask save_as_note); a plain ask does not pay for the copy.
func TestStreamAnswerBlockOptIn(t *testing.T) {
	body := liveStream(t)
	for _, keep := range []bool{false, true} {
		f := newFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
			writeChunks(w, body, 3, time.Millisecond)
		})
		tr, _ := newTransport(t, f)
		ctx := context.Background()
		if keep {
			ctx = wire.WithAnswerBlock(ctx)
		}
		final, err := tr.Stream(ctx, "nb1", streamParams, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := final.AnswerBlock != nil; got != keep {
			t.Fatalf("keep %v: answer block present = %v", keep, got)
		}
	}
}
