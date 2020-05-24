// go-httputils / proxy
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package proxy

import (
    "net"

    "github.com/hshimamoto/go-session"
)

type Proxy struct {
    // callbacks
    OnOpen func(conn net.Conn) bool
    OnConnect func(path string, hdr []string)
    OnGet func(path string, hdr []string)
    OnClose func()
    //
    server *session.Server
}

// Default callbacks
func defOnOpen(conn net.Conn) bool {
    return true
}

func defOnClose() {
}

func defOnConnect(path string, hdr []string) {
}

func defOnGet(path string, hdr []string) {
}

func NewProxy(addr string) (*Proxy, error) {
    proxy := &Proxy{
	OnOpen: defOnOpen,
	OnClose: defOnClose,
	OnConnect: defOnConnect,
	OnGet: defOnGet,
    }
    server, err := session.NewServer(addr, proxy.handler)
    if err != nil {
	return nil, err
    }
    proxy.server = server
    return proxy, err
}

func (p *Proxy)handler(conn net.Conn) {
    if p.OnOpen(conn) == false {
	conn.Close()
	return
    }
    c := NewProxyConnection(p, conn)
    // start processing
    c.process()
    // end
    p.OnClose()
}

func (p *Proxy)Run() {
    p.server.Run()
}
