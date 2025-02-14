// +build integration_tests

package chain

import (
	"0chain.net/chaincore/node"
	"0chain.net/chaincore/round"
	crpc "0chain.net/conductor/conductrpc"
	"0chain.net/conductor/config"
	"0chain.net/core/logging"
	"go.uber.org/zap"
)

var myFailingRound int64 // once set, we ignore all restarts for that round

func (c *Chain) IsRoundGenerator(r round.RoundI, nd *node.Node) bool {

	var (
		rank          = r.GetMinerRank(nd)
		state         = crpc.Client().State()
		comp          bool
		numGenerators = c.GetGeneratorsNumOfRound(r.GetRoundNumber())
		is            = rank != -1 && rank < numGenerators
	)

	if is {
		// test if we have request to skip this round
		if r.GetRoundNumber() == myFailingRound {
			logging.Logger.Info("we're still pretending to be not a generator for round", zap.Int64("round", r.GetRoundNumber()))
			return false
		}
		if config.Round(r.GetRoundNumber()) == state.GeneratorsFailureRoundNumber && r.GetTimeoutCount() == 0 {
			logging.Logger.Info("we're a failing generator for round", zap.Int64("round", r.GetRoundNumber()))
			// remember this round as failing
			myFailingRound = r.GetRoundNumber()
			return false
		}
		return true // regular round generator
	}

	var competingBlock = state.CompetingBlock
	comp = competingBlock.IsCompetingRoundGenerator(state, nd.GetKey(),
		r.GetRoundNumber())

	if comp {
		return true // competing generator
	}

	return false // is not
}
