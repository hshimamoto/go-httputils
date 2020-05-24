// go-httputils / examples /proxy
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package main

import (
    "log"
    "os"

    "github.com/hshimamoto/go-httputils/proxy"
)

func main() {
    addr := ":8080"
    if len(os.Args) > 1 {
	addr = os.Args[1]
    }
    proxy, err := proxy.NewProxy(addr)
    if err != nil {
	log.Fatal(err)
	return
    }
    // setup
    proxy.Run()
}
