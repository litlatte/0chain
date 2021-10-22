package storagesc

import (
	"testing"
	"time"

	"0chain.net/smartcontract/datastore"
	"github.com/stretchr/testify/require"
)

func TestSetupDatabase(t *testing.T) {
	defer func() {
		if datastore.Db != nil {
			datastore.Db.Close()
		}
	}()

	config := datastore.DbAccess{
		Enabled:         true,
		Name:            "stats_db",
		User:            "zchain_user",
		Password:        "zchain",
		Host:            "localhost",
		Port:            "5432",
		MaxIdleConns:    100,
		MaxOpenConns:    200,
		ConnMaxLifetime: 20 * time.Second,
	}
	err := datastore.SetupDatabase(config)
	require.NoError(t, err)
	err = DropChallengeTable()
	require.NoError(t, err)
	err = MigrateChallengeTable()
	require.NoError(t, err)
	sc := StorageChallenge{
		ChallengeID:  "my challenge id",
		RandomNumber: 777,
	}
	err = sc.addToStatsDb("my blobber")
	require.NoError(t, err)
	err = MigrateChallengeTable()
	require.NoError(t, err)
	//result := Db.GetDB().Create(sp)
	//fmt.Println("result", result)
}
