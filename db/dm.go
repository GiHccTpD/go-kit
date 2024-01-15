//go:build dm

package db

import (
	dm "github.com/Leefs/gorm-driver-dm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

func (c *Component) initMysqlDb() *gorm.DB {
	log.Println(packageName, "初始化数据库", c.config.DbName)

	var db *gorm.DB
	var err error

	var vlog = new(log.Logger)
	if c.config.LoggerWriter == nil {
		vlog = log.New(os.Stdout, "\r\n", log.LstdFlags|log.Lshortfile)
	} else {
		vlog = log.New(c.config.LoggerWriter, "", 0)
	}

	newLogger := logger.New(
		vlog, // io writer
		logger.Config{
			SlowThreshold: time.Second,       // Slow SQL threshold
			LogLevel:      c.config.LogLevel, // Log level
			Colorful:      true,
		},
	)

	gconfig := gorm.Config{
		Logger: newLogger,
	}

	for db, err = gorm.Open(dm.Open(c.config.Dsn), &gconfig); err != nil; {
		log.Println(packageName, "❌数据库连接异常", c.config.DbName, err)
		time.Sleep(5 * time.Second)
		db, err = gorm.Open(dm.Open(c.config.Dsn), &gconfig)
	}

	if idb, err := db.DB(); err != nil {
		log.Println(packageName, "❌数据库获取异常", c.config.DbName, err)
		return nil
	} else {
		// ==>  用于设置连接池中空闲连接的最大数量(10)
		idb.SetMaxIdleConns(c.config.MaxIdleConns)

		// ==>  设置打开数据库连接的最大数量(100)
		idb.SetMaxOpenConns(c.config.MaxOpenConns)

		// 最大时间
		idb.SetConnMaxLifetime(c.config.MaxLifetime)

		// 设置 callback
		// otgorm.AddGormCallbacks(db)

		return db
	}
}
