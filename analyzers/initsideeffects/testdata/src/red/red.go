package red

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

func init() {
	_ = os.Setenv("A", "b")       // want `init performs os\.Setenv`
	_ = os.MkdirAll("x", 0o700)   // want `init performs os\.MkdirAll`
	_, _ = os.ReadFile("x")       // want `init performs os\.ReadFile`
	_ = exec.Command("ls").Run()  // want `init performs exec\.Command` `init performs Cmd\.Run`
	fmt.Println("loaded")         // want `init performs fmt\.Println`
	go loop()                     // want `init starts a goroutine`
	_, _ = net.Dial("tcp", "x:1") // want `init performs net\.Dial`
	time.Sleep(time.Second)       // want `init performs time\.Sleep`
	func() {
		_ = os.Remove("x") // want `init performs os\.Remove`
	}()
}

func init() {
	f, _ := os.Open("x") // want `init performs os\.Open`
	f.Close()            // want `init performs File\.Close`
}

func loop() {}
