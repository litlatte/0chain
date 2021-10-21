package datastore

import (
	"time"

	"gorm.io/gorm"
)

var Db Store

type DbAccess struct {
	Enabled  bool   `json:"enabled"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`

	MaxIdleConns    int           `json:"max_idle_conns"`
	MaxOpenConns    int           `json:"max_open_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

type Store interface {
	GetDB() *gorm.DB
	Open(config DbAccess) error
	Close()
}

func SetupDatabase(config DbAccess) error {
	//return nil
	if Db != nil {
		Db.Close()
	}
	if !config.Enabled {
		Db = nil
		return nil
	}
	//if config.Host != "postgresql" {
	//	return fmt.Errorf("%v host not supported, only postgresql", config.Host)
	//}
	Db = &postgresStore{}
	return Db.Open(config)
}
