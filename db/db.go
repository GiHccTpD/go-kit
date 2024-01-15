package db

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
)

const PackageNameMysql = "go-kit.mysql"

// 获取 gorm.DB 对象
func GetGormMysql(dbName string) (*gorm.DB, error) {
	if v, ok := gormPool.Load(dbName); ok {
		return v.(*gorm.DB), nil
	} else {
		return nil, errors.New(packageName + " 获取失败:" + dbName + " 未初始化")
	}
}

// InitMysql 初始化
func (c *Component) InitMysql() *Component {
	// 配置必须信息
	if len(c.config.Dsn) == 0 || len(c.config.DbName) == 0 {
		panic(fmt.Sprintf("❌数据库配置不正确 dbName=%s dsn=%s", c.config.DbName, c.config.Dsn))
	}
	// 初始化 db
	if _, ok := gormPool.Load(c.config.DbName); !ok {
		gormPool.Store(c.config.DbName, c.initMysqlDb())
	}

	// 初始化日志
	log.Println(fmt.Sprintf("[%s] Name:%s 初始化",
		PackageNameMysql,
		c.config.DbName,
	))

	return c
}
