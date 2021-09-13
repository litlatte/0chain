package storagesc

import (
	"encoding/json"
	"fmt"

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

func (qt *qualifyingTotals) save(balances cstate.StateContextI) error {
	_, err := balances.InsertTrieNode(QualifyingTotalsKey, qt)
	return err
}

type qualifyingTotalsList map[int64]qualifyingTotals

func (qt *qualifyingTotalsList) Encode() []byte {
	var b, err = json.Marshal(qt)
	if err != nil {
		panic(err)
	}
	return b
}

func (qt *qualifyingTotalsList) Decode(p []byte) error {
	return json.Unmarshal(p, qt)
}

func (qt *qualifyingTotalsList) save(balances cstate.StateContextI) error {
	_, err := balances.InsertTrieNode(QualifyingTotalsPerBlockKey, qt)
	return err
}

func getQualifyingTotalsList(balances cstate.StateContextI) (*qualifyingTotalsList, error) {
	var val util.Serializable
	val, err := balances.GetTrieNode(QualifyingTotalsPerBlockKey)
	if err != nil {
		return nil, err
	}

	qt := new(qualifyingTotalsList)
	err = qt.Decode(val.Encode())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrDecoding, err)
	}
	return qt, nil
}

// Change capacity
// updateBlobberSettings
// addBlobber
//
// Usage
// addBlobbersOffers / newAllocationRequest
// reduceAllocation / updateAllocation
// extendAllocation / updateAllocation
// finishAllocation / CancelAllocation adn FinaliseAllocation
// updateBlobber

func UpdateRewardTotals(roundNumber int64, balances cstate.StateContextI) error {
	qt, err := getQualifyingTotals(balances)
	if err != nil {
		if err != util.ErrValueNotPresent {
			return err
		}
		qt = new(qualifyingTotals)
	}
	var qtl *qualifyingTotalsList
	qtl, err = getQualifyingTotalsList(balances)
	if err != nil {
		if err != util.ErrValueNotPresent {
			return err
		}
		*qtl = qualifyingTotalsList(make(map[int64]qualifyingTotals))
	}
	(*qtl)[roundNumber] = *qt
	return nil
}

func updateBlockRewardTotals(deltaCapacity, deltaUsed int64, balances cstate.StateContextI) error {
	qt, err := getQualifyingTotals(balances)
	if err != nil {
		if err != util.ErrValueNotPresent {
			return err
		}
		if deltaCapacity < 0 {
			return fmt.Errorf("data currption, cannot reduce by negative %d zero capacity", deltaCapacity)
		}
		if deltaUsed < 0 {
			return fmt.Errorf("data currption, cannot reduce by negative %d zero used", deltaUsed)
		}
		qt = new(qualifyingTotals)
	}
	qt.capacity += deltaCapacity
	if qt.capacity < 0 {
		return fmt.Errorf("data curruption, cannot reduce capacity bellow zero. delta %d, existing capacity %d",
			deltaCapacity, qt.capacity)
	}
	qt.used += deltaUsed
	if qt.used < 0 {
		return fmt.Errorf("data curruption, cannot reduce used bellow zero. delta %d, existing used %d",
			deltaUsed, qt.used)
	}
	return nil
}
