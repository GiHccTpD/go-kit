//go:build linux || darwin
// +build linux darwin

package graceful_shutdown

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const banner = `  ____  _  _  ____     ____  _  _  ____ 
(  _ \( \/ )(  __)___(  _ \( \/ )(  __)
 ) _ ( )  /  ) _)(___)) _ ( )  /  ) _) 
(____/(__/  (____)   (____/(__/  (____)
`

var signalFuncList []func()

func init() {
	signalFuncList = make([]func(), 0, 10)
}

func AddSignalFunc(signalFunc func()) {
	signalFuncList = append(signalFuncList, signalFunc)
}

func WaitSignal() {
	log.Println("wait signal")
	c := make(chan os.Signal, 1)

	// kill 默认会发送 syscall.SIGTERM 信号
	// kill -2 发送 syscall.SIGINT 信号，我们常用的 CTRL + C 就是触发系统 SIGINT 信号
	// kill -9 发送 syscall.SIGKILL 信号，但是不能被捕获，所以不需要添加它
	signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT, syscall.SIGSTOP, syscall.SIGKILL)
	for {
		select {
		case a := <-c:
			log.Println("接受到退出信号: ", a.String())
			//logger.Log.Debug(len(signalFuncList))
			for _, s := range signalFuncList {
				log.Println("run")
				s()
			}
			fmt.Printf("%v\n", fmt.Sprintf("\x1b[32m%s\x1b[0m", banner))
			return
		}
	}
}
