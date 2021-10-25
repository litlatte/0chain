package storagesc

import (
	"encoding/json"
	"fmt"

	chainstate "0chain.net/chaincore/chain/state"
	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/util"
	"gorm.io/gorm"

	ds "0chain.net/core/datastore"
	"0chain.net/smartcontract/datastore"
)

type BlobberChallenge struct {
	gorm.Model
	BlobberID                string                       `json:"blobber_id"`
	ChallengeIds             []string                     `json:"challenge_ids" gorm:"-"`
	Challenges               []*StorageChallenge          `json:"-" gorm:"ForeignKey:blobber_challenges_id"`
	ChallengeMap             map[string]*StorageChallenge `json:"-" gorm:"-"`
	LatestCompletedChallenge *StorageChallenge            `json:"latest_completed_challenge" gorm:"-"`
}

func (sn *BlobberChallenge) GetKey(globalKey string) ds.Key {
	return ds.Key(globalKey + ":blobberchallenge:" + sn.BlobberID)
}

func (sn *BlobberChallenge) Encode() []byte {
	buff, _ := json.Marshal(sn)
	return buff
}

func (sn *BlobberChallenge) GetHash() string {
	return util.ToHex(sn.GetHashBytes())
}

func (sn *BlobberChallenge) GetHashBytes() []byte {
	return encryption.RawHash(sn.Encode())
}

func (sn *BlobberChallenge) Decode(input []byte) error {
	err := json.Unmarshal(input, sn)
	if err != nil {
		return err
	}
	sn.ChallengeMap = make(map[string]*StorageChallenge)
	for _, challenge := range sn.Challenges {
		sn.ChallengeMap[challenge.ChallengeID] = challenge
	}
	return nil
}

func (sn *BlobberChallenge) addChallenge(challenge *StorageChallenge) bool {
	if sn.Challenges == nil {
		sn.Challenges = make([]*StorageChallenge, 0)
		sn.ChallengeMap = make(map[string]*StorageChallenge)
	}
	if _, ok := sn.ChallengeMap[challenge.ChallengeID]; !ok {
		if len(sn.Challenges) > 0 {
			lastChallenge := sn.Challenges[len(sn.Challenges)-1]
			challenge.PrevID = lastChallenge.ChallengeID
		} else if sn.LatestCompletedChallenge != nil {
			challenge.PrevID = sn.LatestCompletedChallenge.ChallengeID
		}
		sn.Challenges = append(sn.Challenges, challenge)
		sn.ChallengeMap[challenge.ChallengeID] = challenge
		return true
	}
	return false
}

type StorageChallenge struct {
	gorm.Model
	BlobberChallengesId      int                 `json:"blobber_challenges_id"`
	Created                  common.Timestamp    `json:"created"`
	ChallengeID              string              `json:"challenge_id"`
	PrevID                   string              `json:"prev_id"`
	Validators               []*ValidationNodeSC `json:"validators" gorm:"ForeignKey:storage_challenge_id"`
	RandomNumber             int64               `json:"seed"`
	AllocationID             string              `json:"allocation_id"`
	AllocationRoot           string              `json:"allocation_root"`
	Response                 *ChallengeResponse  `json:"challenge_response,omitempty" gorm:"ForeignKey:storage_challenge_id"`
	LatestCompletedChallenge bool                `json:"-"`
}

type ChallengeResponse struct {
	gorm.Model
	StorageChallengeId int                 `json:"storage_challenge_id" gorm:"storage_challenge_id"`
	ID                 string              `json:"challenge_id"`
	ValidationTickets  []*ValidationTicket `json:"validation_tickets" gorm:"ForeignKey:challenge_response_id"`
}

type ValidationNodeSC struct {
	gorm.Model
	StorageChallengeId int    `json:"storage_challenge_id" gorm:"storage_challenge_id"`
	ID                 string `json:"id"`
	BaseURL            string `json:"url"`
}

type ValidationTicket struct {
	gorm.Model
	ChallengeResponseId int              `json:"challenge_response_id" gorm:"challenge_response_id"`
	ChallengeID         string           `json:"challenge_id"`
	BlobberID           string           `json:"blobber_id"`
	ValidatorID         string           `json:"validator_id"`
	ValidatorKey        string           `json:"validator_key"`
	Result              bool             `json:"success"`
	Message             string           `json:"message"`
	MessageCode         string           `json:"message_code"`
	Timestamp           common.Timestamp `json:"timestamp"`
	Signature           string           `json:"signature"`
}

func (vt *ValidationTicket) VerifySign(balances chainstate.StateContextI) (bool, error) {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v", vt.ChallengeID, vt.BlobberID,
		vt.ValidatorID, vt.ValidatorKey, vt.Result, vt.Timestamp)
	hash := encryption.Hash(hashData)
	signatureScheme := balances.GetSignatureScheme()
	signatureScheme.SetPublicKey(vt.ValidatorKey)
	verified, err := signatureScheme.Verify(vt.Signature, hash)
	return verified, err
}

type BlobberChallengeId struct {
	ID        int
	BlobberID string
}

func (bci *BlobberChallengeId) getOrCreate(blobberId string) error {
	Db := datastore.Db.Get()

	result := Db.Model(&BlobberChallenge{}).Find(&BlobberChallengeId{}).Where("blobber_id", blobberId).First(&bci)

	if result.RowsAffected == 0 {
		bc := BlobberChallenge{
			BlobberID: blobberId,
		}
		err := bc.addToStatsDb()
		if err != nil {
			return err
		}
		result := Db.Model(&BlobberChallenge{}).Find(&BlobberChallengeId{}).Where("blobber_id", blobberId).First(&bci)
		if result.RowsAffected == 0 {
			return fmt.Errorf("cannot create blobber challenge %v, db error %v",
				blobberId, result.Error)
		}
	}
	return nil
}

func (sc *StorageChallenge) addToStatsDb(blobberId string) error {
	Db := datastore.Db.Get()

	bc := BlobberChallengeId{}
	if err := bc.getOrCreate(blobberId); err != nil {
		return err
	}

	sc.BlobberChallengesId = bc.ID
	Db.Create(sc)

	return nil
}

func statsDbDeleteStorageChallengeForAllocation(blobberId, allocationId string) error {
	Db := datastore.Db.Get()

	bc := BlobberChallengeId{}
	if err := bc.getOrCreate(blobberId); err != nil {
		return err
	}

	result := Db.Delete(
		&StorageChallenge{},
	).Where("blobber_challenges_id = ?", bc.ID).Where("allocation_id = ?", allocationId)

	return result.Error
}

func removeStorageChallenge(challengeId string) error {
	Db := datastore.Db.Get()

	result := Db.Delete(&StorageChallenge{}, "challenge_id", challengeId)
	return result.Error
}

func (bc *BlobberChallenge) addToStatsDb() error {
	result := datastore.Db.Get().Create(bc)
	return result.Error
}

func DropChallengeTable() error {
	err := datastore.Db.Get().Migrator().DropTable(&ValidationTicket{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().Migrator().DropTable(&ChallengeResponse{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().Migrator().DropTable(&ValidationNodeSC{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().Migrator().DropTable(&StorageChallenge{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().Migrator().DropTable(&BlobberChallenge{})
	if err != nil {
		return err
	}
	return nil
}

func MigrateChallengeTable() error {
	err := datastore.Db.Get().AutoMigrate(&ValidationTicket{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().AutoMigrate(&ChallengeResponse{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().AutoMigrate(&ValidationNodeSC{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().AutoMigrate(&StorageChallenge{})
	if err != nil {
		return err
	}
	err = datastore.Db.Get().AutoMigrate(&BlobberChallenge{})
	if err != nil {
		return err
	}
	return nil
}
