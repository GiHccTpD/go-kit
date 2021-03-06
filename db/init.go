package db

import (
	"gorm.io/gorm/logger"
	"io"
	"log"
	"os"
	"time"
)

type Option func(c *Container)

type Container struct {
	config *config
}

func DefaultContainer() *Container {
	return &Container{
		config: DefaultConfig(),
	}
}

func Load(dbName string) *Container {
	c := DefaultContainer()
	c.config.DbName = dbName
	return c
}

// SetMaxIdleConns 用于设置连接池中空闲连接的最大数量(10)
func WithMaxIdleConns(maxIdleConns int) Option {
	return func(c *Container) {
		c.config.MaxIdleConns = maxIdleConns
	}
}

// SetMaxOpenConns 设置打开数据库连接的最大数量(100)
func WithMaxOpenConnss(maxOpenConns int) Option {
	return func(c *Container) {
		c.config.MaxOpenConns = maxOpenConns
	}
}

func WithMaxLifetime(maxLifetime time.Duration) Option {
	return func(c *Container) {
		c.config.MaxLifetime = maxLifetime
	}
}

func WithDsn(dsn string) Option {
	return func(c *Container) {
		c.config.Dsn = dsn
	}
}

func WithLoggerWriter(loggerWriter io.Writer) Option {
	return func(c *Container) {
		c.config.LoggerWriter = loggerWriter
	}
}

func WithLogLevel(logLevel logger.LogLevel) Option {
	return func(c *Container) {
		c.config.LogLevel = logLevel
	}
}

// Build ...
func (c *Container) Build(options ...Option) *Component {
	for _, option := range options {
		option(c)
	}

	// 设置 Logger
	if c.config.LoggerWriter != nil {
		c.config.Logger = logger.New(
			log.New(c.config.LoggerWriter, "\r\n", log.LstdFlags), // io writer（日志输出的目标，前缀和日志包含的内容——译者注）
			logger.Config{
				SlowThreshold:             time.Second,       // 慢 SQL 阈值
				LogLevel:                  c.config.LogLevel, // 日志级别
				IgnoreRecordNotFoundError: false,             // 忽略ErrRecordNotFound（记录未找到）错误
				Colorful:                  true,              // 禁用彩色打印
			},
		)
	} else {
		c.config.Logger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer（日志输出的目标，前缀和日志包含的内容——译者注）
			logger.Config{
				SlowThreshold:             time.Second,       // 慢 SQL 阈值
				LogLevel:                  c.config.LogLevel, // 日志级别
				IgnoreRecordNotFoundError: false,             // 忽略ErrRecordNotFound（记录未找到）错误
				Colorful:                  true,              // 禁用彩色打印
			},
		)
	}

	return newComponent(c.config)
}
