//go:build linux || darwin
// +build linux darwin

package graceful_shutdown

import (
	"fmt"
	"github.com/GiHccTpD/go-kit/logger"
	"os"
	"os/signal"
	"syscall"
)

const byebye = `  _______   __  __   ______        _______   __  __   ______      
/_______/\ /_/\/_/\ /_____/\     /_______/\ /_/\/_/\ /_____/\     
\::: _  \ \\ \ \ \ \\::::_\/_    \::: _  \ \\ \ \ \ \\::::_\/_    
 \::(_)  \/_\:\_\ \ \\:\/___/\    \::(_)  \/_\:\_\ \ \\:\/___/\   
  \::  _  \ \\::::_\/ \::___\/_    \::  _  \ \\::::_\/ \::___\/_  
   \::(_)  \ \ \::\ \  \:\____/\    \::(_)  \ \ \::\ \  \:\____/\ 
    \_______\/  \__\/   \_____\/     \_______\/  \__\/   \_____\/`

var signalFuncList []func()

func init() {
	signalFuncList = make([]func(), 0, 10)
}

func AddSignalFunc(signalFunc func()) {
	signalFuncList = append(signalFuncList, signalFunc)
}

func WaitSignal() {
	logger.Log.SetLevel(logger.LevelDebug)
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
			fmt.Printf("%v\n", fmt.Sprintf("\x1b[32m%s\x1b[0m", byebye))
			return
		}
	}
}
