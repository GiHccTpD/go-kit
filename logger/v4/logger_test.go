package v4

import (
	"context"
	"github.com/GiHccTpD/go-kit/known"
	"github.com/google/uuid"
	"testing"
)

func TestInit(t *testing.T) {
	var ctx = context.WithValue(context.Background(), known.XRequestIDKey, uuid.New().String())
	var level = "info"

	Init(&Options{
		DisableCaller:     false,
		DisableStacktrace: false,
		Level:             level,
		Format:            "json",
		OutputPaths:       []string{"stdout", "app.log"},
	})

	std.Debugw("debug")

	std.C(ctx).Infow("update log level", "level", level)

	std.C(ctx).Debugw("不输出")
	std.C(ctx).SetLevel("debug")
	std.C(ctx).Debugw("输出")
}
