// +build !integration_tests
// todo: it's a legacy ugly approach; refactor later

package storagesc

import (
	"fmt"

	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/transaction"
)

// insert new blobber, filling its stake pool
func (sc *StorageSmartContract) insertBlobber(t *transaction.Transaction,
	conf *scConfig, blobber *StorageNode, blobbers *StorageNodes,
	balances cstate.StateContextI,
) (err error) {
	// check for duplicates
	for _, b := range blobbers.Nodes {
		if b.ID == blobber.ID || b.BaseURL == blobber.BaseURL {
			var sp *stakePool
			sp, err = sc.getOrCreateStakePool(conf, blobber.ID,
				&blobber.StakePoolSettings, balances)
			if err != nil {
				return fmt.Errorf("creating stake pool: %v", err)
			}
			return sc.updateBlobber(t, conf, blobber, blobbers, sp, balances)
		}
	}

	// check params
	if err = blobber.validate(conf); err != nil {
		return fmt.Errorf("invalid blobber params: %v", err)
	}

	blobber.LastHealthCheck = t.CreationDate // set to now

	// create stake pool
	var sp *stakePool
	sp, err = sc.getOrCreateStakePool(conf, blobber.ID,
		&blobber.StakePoolSettings, balances)
	if err != nil {
		return fmt.Errorf("creating stake pool: %v", err)
	}

	if err = sp.save(sc.ID, t.ClientID, balances); err != nil {
		return fmt.Errorf("saving stake pool: %v", err)
	}

	if sp.stake() >= conf.BlockReward.QualifyingStake {
		balances.UpdateBlockRewardTotals(blobber.Capacity, 0)
	}
	qtl, err := getQualifyingTotalsList(balances)
	if err != nil {
		return fmt.Errorf("getting block reward totals: %v", err)
	}
	if err := qtl.payBlobberRewards(blobber, sp, conf, balances); err != nil {
		return fmt.Errorf("paying blobber rewards: %v", err)
	}

	// update the list
	blobbers.Nodes.add(blobber)

	// update statistic
	sc.statIncr(statAddBlobber)
	sc.statIncr(statNumberOfBlobbers)
	return
}
