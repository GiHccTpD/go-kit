package sugar

import (
	logger "github.com/GiHccTpD/go-kit/logger/v3"
	"go.uber.org/zap"
)

func If(condition bool, trueVal, falseVal interface{}) interface{} {
	logger.Debugw("Sugar If", zap.Bool("condition", condition))
	if condition {
		return trueVal
	}
	return falseVal
}

func IfExpress(condition bool, trueVal func() interface{}, falseVal func() interface{}) interface{} {
	logger.Debugw("Sugar IfExpress", zap.Bool("condition", condition))
	if condition {
		return trueVal()
	}
	return falseVal()
}
