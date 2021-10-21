package datastore

import (
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type postgresStore struct {
	db *gorm.DB
}

func (store *postgresStore) Open(config DbAccess) error {
	if !config.Enabled {
		return errors.New("db_open_error, db disabled")
	}

	db, err := gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%v port=%v user=%v dbname=%v password=%v sslmode=disable",
		config.Host,
		config.Port,
		config.User,
		config.Name,
		config.Password)),
		&gorm.Config{
			SkipDefaultTransaction: true,
			PrepareStmt:            true,
		})
	if err != nil {
		return fmt.Errorf("db_open_error, Error opening the DB connection: %v", err)
	}

	sqldb, err := db.DB()
	if err != nil {
		return fmt.Errorf("db_open_error, Error opening the DB connection: %v", err)
	}

	sqldb.SetMaxIdleConns(config.MaxIdleConns)
	sqldb.SetMaxOpenConns(config.MaxOpenConns)
	sqldb.SetConnMaxLifetime(config.ConnMaxLifetime)

	store.db = db
	fmt.Println("piers made sql database ok")
	return nil
}

func (store *postgresStore) Close() {
	if store.db != nil {
		if sqldb, _ := store.db.DB(); sqldb != nil {
			sqldb.Close()
		}
	}
}

func (store *postgresStore) GetDB() *gorm.DB {
	return store.db
}
