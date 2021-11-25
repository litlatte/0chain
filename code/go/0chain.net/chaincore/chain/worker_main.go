//go:build !integration_tests
// +build !integration_tests

package chain

import (
	"context"
	"time"

	"0chain.net/core/logging"
	"go.uber.org/zap"
)

/*FinalizeRoundWorker - a worker that handles the finalized blocks */
func (c *Chain) FinalizeRoundWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-c.finalizedRoundsChannel:
			func() {
				// TODO: make the timeout configurable
				cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				doneC := make(chan struct{})
				go func() {
					defer close(doneC)
					c.finalizeRound(cctx, r)
					c.UpdateRoundInfo(r)
				}()

				select {
				case <-cctx.Done():
					logging.Logger.Warn("FinalizeRoundWorker finalize round timeout",
						zap.Int64("round", r.GetRoundNumber()))
				case <-doneC:
				}
			}()
		}
	}
}
