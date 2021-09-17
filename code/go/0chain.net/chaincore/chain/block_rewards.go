package chain

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
	QualifyingTotalsKey         = datastore.Key("6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7" + encryption.Hash("qualifying_totals"))
	QualifyingTotalsPerBlockKey = datastore.Key("6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7" + encryption.Hash("qualifying_totals_per_block"))
)

type blockReward struct {
	BlockReward           state.Balance `json:"block_reward"`
	QualifyingStake       state.Balance `json:"qualifying_stake"`
	SharderWeight         float64       `json:"sharder_weight"`
	MinerWeight           float64       `json:"miner_weight"`
	BlobberCapacityWeight float64       `json:"blobber_capacity_weight"`
	BlobberUsageWeight    float64       `json:"blobber_usage_weight"`
}

type qualifyingTotals struct {
	Capacity       int64        `json:"capacity"`
	Used           int64        `json:"used"`
	SettingsChange *blockReward `json:"settings_change"`
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

func updateRewardTotalList(balances cstate.StateContextI) error {
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
