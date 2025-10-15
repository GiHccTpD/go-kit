package signal

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var signalFuncList []func()

const byebye = `
  _______   __  __   ______        _______   __  __   ______      
 /_______/\ /_/\/_/\ /_____/\     /_______/\ /_/\/_/\ /_____/\     
 \::: _  \ \\ \ \ \ \\::::_\/_    \::: _  \ \\ \ \ \ \\::::_\/_    
  \::(_)  \/_\:\_\ \ \\:\/___/\    \::(_)  \/_\:\_\ \ \\:\/___/\   
   \::  _  \ \\::::_\/ \::___\/_    \::  _  \ \\::::_\/ \::___\/_  
    \::(_)  \ \ \::\ \  \:\____/\    \::(_)  \ \ \::\ \  \:\____/\ 
     \_______\/  \__\/   \_____\/     \_______\/  \__\/   \_____\/
`

func init() {
	signalFuncList = make([]func(), 0, 10)
}

func AddSignalFunc(f func()) {
	signalFuncList = append(signalFuncList, f)
}

func WaitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	s := <-c
	fmt.Printf("🧩 received signal: %v", s)

	for _, f := range signalFuncList {
		func() {
			defer func() {
				if r := recover(); r != nil {
				}
			}()
			f()
		}()
	}

	fmt.Printf("\x1b[32m%s\x1b[0m\n", byebye)
}
