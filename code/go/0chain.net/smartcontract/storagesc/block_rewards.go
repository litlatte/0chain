package storagesc

import (
	"encoding/json"
	"fmt"

	"0chain.net/chaincore/state"

	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/util"
)

var (
	QualifyingTotalsKey         = datastore.Key(ADDRESS + encryption.Hash("qualifying_totals"))
	QualifyingTotalsPerBlockKey = datastore.Key(ADDRESS + encryption.Hash("qualifying_totals_per_block"))
)

type qualifyingTotals struct {
	capacity, used int64
	settingsChange *blockReward
}

func (qt *qualifyingTotals) Encode() []byte {
	var b, err = json.Marshal(qt)
	if err != nil {
		panic(err)
	}
	return b
}

func (qt *qualifyingTotals) Decode(p []byte) error {
	return json.Unmarshal(p, qt)
}

func getQualifyingTotals(balances cstate.StateContextI) (*qualifyingTotals, error) {
	var val util.Serializable
	val, err := balances.GetTrieNode(QualifyingTotalsKey)
	if err != nil {
		return nil, err
	}

	qt := new(qualifyingTotals)
	err = qt.Decode(val.Encode())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrDecoding, err)
	}
	return qt, nil
}

func (qtl *qualifyingTotalsList) payBlobberRewards(
	blobber *StorageNode,
	sp *stakePool,
	conf *scConfig,
	balances cstate.StateContextI,
) error {
	if len(*qtl) == 0 {
		return nil
	}
	var stakes = float64(sp.stake())
	if stakes == 0 {
		return nil
	}
	numRounds := int(balances.GetBlock().Round - blobber.LastBlockRewardPayment)
	if numRounds > len(*qtl) {
		numRounds = len(*qtl) - 1
	}
	var settings blockReward = *conf.BlockReward
	var reward = blobber.BlockRewardCarry
	for i := 0; i < numRounds; i++ {
		index := len(*qtl) - i
		if (*qtl)[index].settingsChange != nil {
			settings = *(*qtl)[index].settingsChange
		}

		var capRatio float64
		if (*qtl)[index].capacity > 0 {
			capRatio = float64(blobber.Capacity) / float64((*qtl)[index].capacity)
		}
		capacityReward := float64(settings.BlockReward) * settings.BlobberCapacityWeight * capRatio

		var usedRatio float64
		if (*qtl)[index].used > 0 {
			usedRatio = float64(blobber.Used) / float64((*qtl)[index].used)
		}
		usedReward := float64(settings.BlockReward) * settings.BlobberUsageWeight * usedRatio

		reward += capacityReward + usedReward
	}

	var totalRewardUsed state.Balance
	for _, pool := range sp.Pools {
		poolReward := state.Balance(reward * float64(pool.Balance) / stakes)
		if err := balances.AddMint(state.NewMint(ADDRESS, pool.DelegateID, poolReward)); err != nil {
			return fmt.Errorf(
				"error miniting block reward, mint: %v\terr: %v",
				state.NewMint(ADDRESS, pool.DelegateID, poolReward), err,
			)
		}
		totalRewardUsed += poolReward
	}
	blobber.BlockRewardCarry = reward - float64(totalRewardUsed)

	return nil
}

type qualifyingTotalsList []qualifyingTotals

func newQualifyingTotalsList() qualifyingTotalsList {
	return make([]qualifyingTotals, 1024)
}

func (qtl *qualifyingTotalsList) Encode() []byte {
	var b, err = json.Marshal(qtl)
	if err != nil {
		panic(err)
	}
	return b
}

func (qtl *qualifyingTotalsList) Decode(p []byte) error {
	return json.Unmarshal(p, qtl)
}

func UpdateRewardTotalList(balances cstate.StateContextI) error {
	qt, err := getQualifyingTotals(balances)
	if err != nil {
		if err != util.ErrValueNotPresent {
			return err
		}
		qt = new(qualifyingTotals)
	}
	var qtl qualifyingTotalsList
	qtl, err = getQualifyingTotalsList(balances)
	if err != nil {
		return err
	}
	qtl[balances.GetBlock().Round] = *qt
	if err := qtl.save(balances); err != nil {
		return err
	}
	return nil
}

func (qtl *qualifyingTotalsList) save(balances cstate.StateContextI) error {
	_, err := balances.InsertTrieNode(QualifyingTotalsPerBlockKey, qtl)
	return err
}

func getQualifyingTotalsList(balances cstate.StateContextI) (qualifyingTotalsList, error) {
	var val util.Serializable
	val, err := balances.GetTrieNode(QualifyingTotalsPerBlockKey)
	if err != nil {
		if err != util.ErrValueNotPresent {
			return nil, err
		}
		return newQualifyingTotalsList(), nil
	}

	qtl := newQualifyingTotalsList()
	err = qtl.Decode(val.Encode())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrDecoding, err)
	}
	return qtl, nil
}
