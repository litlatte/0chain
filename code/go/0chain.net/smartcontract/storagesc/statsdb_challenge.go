package storagesc

import (
	"fmt"

	"0chain.net/smartcontract/datastore"
)

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
