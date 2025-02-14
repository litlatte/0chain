package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/node"
	"0chain.net/chaincore/state"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/logging"
	"0chain.net/core/util"
	"go.uber.org/zap"
)

/*SetupNodeHandlers - setup the handlers for the chain */
func (c *Chain) SetupNodeHandlers() {
	http.HandleFunc("/_nh/list/m", common.Recover(c.GetMinersHandler))
	http.HandleFunc("/_nh/list/s", common.Recover(c.GetShardersHandler))
}

var (
	// MinerNotarizedBlockRequestor - reuqest a notarized block from a node.
	MinerNotarizedBlockRequestor node.EntityRequestor
	//BlockStateChangeRequestor - request state changes for the block.
	BlockStateChangeRequestor node.EntityRequestor

	// disables (doesn't work, sharders doesn't give changes)
	//
	// ShardersBlockStateChangeRequestor is the same, but from sharders.
	// ShardersBlockStateChangeRequestor node.EntityRequestor

	// PartialStateRequestor - request partial state from a given root.
	PartialStateRequestor node.EntityRequestor
	// StateNodesRequestor - request a set of state nodes given their keys.
	StateNodesRequestor node.EntityRequestor
	// LatestFinalizedMagicBlockRequestor - RequestHandler for latest finalized
	// magic block to a node.
	LatestFinalizedMagicBlockRequestor node.EntityRequestor

	// FBRequestor represents FB from sharders reqeustor.
	FBRequestor node.EntityRequestor
)

/*SetupX2MRequestors - setup requestors */
func SetupX2MRequestors() {
	options := &node.SendOptions{Timeout: node.TimeoutLargeMessage, CODEC: node.CODEC_MSGPACK, Compress: true}

	blockEntityMetadata := datastore.GetEntityMetadata("block")
	MinerNotarizedBlockRequestor = node.RequestEntityHandler("/v1/_x2m/block/notarized_block/get", options, blockEntityMetadata)

	options = &node.SendOptions{Timeout: node.TimeoutLargeMessage, CODEC: node.CODEC_JSON, Compress: true}
	blockStateChangeEntityMetadata := datastore.GetEntityMetadata("block_state_change")
	BlockStateChangeRequestor = node.RequestEntityHandler("/v1/_x2x/block/state_change/get", options, blockStateChangeEntityMetadata)
	// ShardersBlockStateChangeRequestor = node.RequestEntityHandler("/v1/_x2s/block/state_change/get", options, blockStateChangeEntityMetadata)

	partialStateEntityMetadata := datastore.GetEntityMetadata("partial_state")
	PartialStateRequestor = node.RequestEntityHandler("/v1/_x2m/state/get", options, partialStateEntityMetadata)

	stateNodesEntityMetadata := datastore.GetEntityMetadata("state_nodes")
	StateNodesRequestor = node.RequestEntityHandler("/v1/_x2x/state/get_nodes", options, stateNodesEntityMetadata)
}

func SetupX2SRequestors() {
	blockEntityMetadata := datastore.GetEntityMetadata("block")
	options := &node.SendOptions{Timeout: node.TimeoutLargeMessage, MaxRelayLength: 0, CurrentRelayLength: 0, Compress: false}
	LatestFinalizedMagicBlockRequestor = node.RequestEntityHandler("/v1/block/get/latest_finalized_magic_block", options, blockEntityMetadata)

	var opts = node.SendOptions{
		Timeout:  node.TimeoutLargeMessage,
		CODEC:    node.CODEC_MSGPACK,
		Compress: true,
	}
	FBRequestor = node.RequestEntityHandler("/v1/_x2s/block/get", &opts,
		datastore.GetEntityMetadata("block"))
}

func SetupX2XResponders(c *Chain) {
	http.HandleFunc("/v1/_x2x/state/get_nodes", common.N2NRateLimit(node.ToN2NSendEntityHandler(StateNodesHandler)))
	http.HandleFunc("/v1/_x2x/block/state_change/get", common.N2NRateLimit(node.ToN2NSendEntityHandler(c.BlockStateChangeHandler)))
}

//StateNodesHandler - return a list of state nodes
func StateNodesHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	r.ParseForm() // this is needed as we get multiple values for the same key
	nodes := r.Form["nodes"]
	c := GetServerChain()
	keys := make([]util.Key, len(nodes))
	for idx, nd := range nodes {
		key, err := hex.DecodeString(nd)
		if err != nil {
			return nil, err
		}
		keys[idx] = key
	}
	ns, err := c.GetStateNodesFrom(ctx, keys)
	if err != nil {
		if ns != nil {
			logging.Logger.Error("state nodes handler", zap.Int("keys", len(nodes)), zap.Int("found_keys", len(ns.Nodes)), zap.Error(err))
			return ns, nil
		}

		logging.Logger.Error("state nodes handler",
			zap.Int("keys", len(nodes)),
			zap.Int64("current round", c.GetCurrentRound()),
			zap.Error(err))

		return nil, err
	}
	logging.Logger.Info("state nodes handler", zap.Int("keys", len(keys)), zap.Int("nodes", len(ns.Nodes)))
	return ns, nil
}

// BlockStateChangeHandler - provide the state changes associated with a block.
func (c *Chain) BlockStateChangeHandler(ctx context.Context, r *http.Request) (interface{}, error) {
	var b, err = c.getNotarizedBlock(ctx, r)
	if err != nil {
		return nil, err
	}

	if b.GetStateStatus() != block.StateSuccessful {
		return nil, common.NewError("state_not_verified",
			"state is not computed and validated locally")
	}

	var bsc = block.NewBlockStateChange(b)
	if state.Debug() {
		logging.Logger.Info("block state change handler", zap.Int64("round", b.Round),
			zap.String("block", b.Hash),
			zap.Int("state_changes", b.ClientState.GetChangeCount()),
			zap.Int("sc_nodes", len(bsc.Nodes)))
	}

	//if len(bsc.Nodes) == 0 {
	//	logging.Logger.Debug("get state changes - no changes", zap.Int64("round", b.Round))
	if bsc.GetRoot() == nil {
		cr := c.GetCurrentRound()
		logging.Logger.Debug("get state changes - state nil root",
			zap.Int64("round", b.Round),
			zap.Int64("current_round", cr))
	}

	return bsc, nil
}

func (c *Chain) getNotarizedBlock(ctx context.Context, req *http.Request) (*block.Block, error) {

	var (
		r    = req.FormValue("round")
		hash = req.FormValue("block")

		cr = c.GetCurrentRound()
	)

	errBlockNotAvailable := common.NewError("block_not_available",
		fmt.Sprintf("Requested block is not available, current round: %d, request round: %s, request hash: %s",
			cr, r, hash))

	if hash != "" {
		b, err := c.GetBlock(ctx, hash)
		if err != nil {
			return nil, err
		}

		if b.IsBlockNotarized() {
			return b, nil
		}
		return nil, errBlockNotAvailable
	}

	if r == "" {
		return nil, common.NewError("none_round_or_hash_provided",
			"no block hash or round number is provided")
	}

	roundN, err := strconv.ParseInt(r, 10, 64)
	if err != nil {
		return nil, err
	}

	rd := c.GetRound(roundN)
	if rd == nil {
		return nil, errBlockNotAvailable
	}

	b := rd.GetHeaviestNotarizedBlock()
	if b == nil {
		return nil, errBlockNotAvailable
	}

	return b, nil
}
