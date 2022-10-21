//go:build linux || darwin
// +build linux darwin

package graceful_shutdown

import (
	"github.com/GiHccTpD/go-kit/logger"
	"os"
	"os/signal"
	"syscall"
)

var signalFuncList []func()

func init() {
	signalFuncList = make([]func(), 0, 10)
}

func AddSignalFunc(signalFunc func()) {
	signalFuncList = append(signalFuncList, signalFunc)
}

func WaitSignal() {
	logger.Log.Debug("wait signal")
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGSTOP, syscall.SIGKILL)
	for {
		select {
		case a := <-c:
			logger.Log.Debug("接受到退出信号: ", a.String())
			//logger.Log.Debug(len(signalFuncList))
			for _, s := range signalFuncList {
				logger.Log.Debug("run")
				s()
			}
			return
		}
	}
}
