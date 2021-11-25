//go:build integration_tests
// +build integration_tests

package chain

import (
	"context"
	"encoding/gob"
	"time"

	"0chain.net/chaincore/round"
	"0chain.net/core/logging"
	"0chain.net/core/util"
	"github.com/rcrowley/go-metrics"
	"go.uber.org/zap"
)

/*FinalizeRoundWorker - a worker that handles the finalized blocks */
func (c *Chain) FinalizeRoundWorker(ctx context.Context) {
	for {
		var r round.RoundI
		select {
		case <-ctx.Done():
			return
		case r = <-c.finalizedRoundsChannel:
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
					//if err := crpc.Client().StoreFinalisedRound(r); err != nil {
					//	registerGobTypes()
					//	if err := crpc.Client().StoreFinalisedRound(r); err != nil {
					//		registerGobTypes()
					//		if err := crpc.Client().StoreFinalisedRound(r); err != nil {
					//			log.Printf("Confuctor: storing round failed, err %v,\nround %#v", err, r)
					//		}
					//	}
					//}
				}
			}()
		}
	}
}

func registerGobTypes() {
	gob.Register(util.MerklePatriciaTrie{})
	gob.Register(util.ChangeCollector{})
	gob.Register(util.LeafNode{})
	gob.Register(util.FullNode{})
	gob.Register(util.ExtensionNode{})
	gob.Register(util.ValueNode{})
	gob.Register(util.SecureSerializableValue{})
	gob.Register(util.OriginTracker{})
	gob.Register(metrics.StandardTimer{})
}
