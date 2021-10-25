package storagesc

import (
	"encoding/json"
	"fmt"

	chainstate "0chain.net/chaincore/chain/state"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
)

type ABChallenges struct {
	BlobberId string             `json:"blobber_id"`
	Created   []common.Timestamp `json:"created"`

	//Belt and braces
	AllocationRoot string `json:"allocation_root"`

	//OpenChallenges          int                `json:"open_challenges"`
	//TotalChallenges         int                `json:"total_challenges"`
	FailedChallenges  int `json:"failed_challenges"`
	SuccessChallenges int `json:"success_challenges"`

	// from blobber terms
	ChallengeCompletionTime common.Timestamp `json:"challenge_completion_time"`
}

func (abc *ABChallenges) Encode() []byte {
	var b, err = json.Marshal(abc)
	if err != nil {
		panic(err)
	}
	return b
}

func (abc *ABChallenges) Decode(p []byte) error {
	return json.Unmarshal(p, abc)
}

type AllocationChallenges struct {
	AllocationId string `json:"allocation_id"`
	DataShards   int    `json:"data_shards"`
	//OpenChallenges    int              `json:"open_challenges"`
	//TotalChallenges   int              `json:"total_challenges"`
	//FailedChallenges  int              `json:"failed_challenges"`
	//SuccessChallenges int              `json:"success_challenges"`
	Blobbers   []*ABChallenges  `json:"blobbers"`
	Expiration common.Timestamp `json:"expiration"`
	HasWrite   bool             `json:"has_write"`
}

func GetAllocationChallengeKey(allocId string) datastore.Key {
	return datastore.Key(ADDRESS + ":allocation_challenges:" + allocId)
}

func GetAllocationChallenges(
	allocId string, balances chainstate.StateContextI,
) (*AllocationChallenges, error) {
	var ac = new(AllocationChallenges)
	serializable, err := balances.GetTrieNode(GetAllocationChallengeKey(allocId))
	if err != nil {
		return nil, err
	}
	if err := ac.Decode(serializable.Encode()); err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrDecoding, err)
	}
	return ac, nil
}

func (ac *AllocationChallenges) save(balances chainstate.StateContextI) error {
	_, err := balances.InsertTrieNode(GetAllocationChallengeKey(ac.AllocationId), ac)
	if err != nil {
		return err
	}
	return nil
}

func (ac *AllocationChallenges) Encode() []byte {
	var b, err = json.Marshal(ac)
	if err != nil {
		panic(err)
	}
	return b
}

func (ac *AllocationChallenges) Decode(p []byte) error {
	return json.Unmarshal(p, ac)
}
