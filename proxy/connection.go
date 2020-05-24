// go-httputils / proxy
// MIT License Copyright(c) 2020 Hiroshi Shimamoto
// vim:set sw=4 sts=4:
package proxy

import (
    "bytes"
    "io"
    "log"
    "net"
    "net/url"
    "strings"

    "github.com/hshimamoto/go-iorelay"
    "github.com/hshimamoto/go-session"
)

type BuffConn struct {
    conn net.Conn
    buf []byte
    eof bool
}

func (b *BuffConn)GetLineFromBuf() []byte {
    if len(b.buf) == 0 {
	return nil
    }
    idx := bytes.Index(b.buf, []byte("\r\n"))
    if idx == -1 {
	if b.eof {
	    line := b.buf
	    b.buf = nil
	    return line
	}
	return nil
    }
    line := append([]byte{}, b.buf[:idx]...)
    b.buf = b.buf[idx + 2:]
    if len(b.buf) == 0 {
	b.buf = nil
    }
    return line
}

func (b *BuffConn)ReadLine() []byte {
    if line := b.GetLineFromBuf(); line != nil {
	return line
    }
    if b.eof {
	return nil
    }
    if b.buf == nil {
	b.buf = []byte{}
    }
    buf := make([]byte, 4096)
    r, err := b.conn.Read(buf)
    if err != nil {
	return nil
    }
    if r > 0 {
	b.buf = append(b.buf, buf[:r]...)
    } else {
	b.eof = true
    }
    return b.ReadLine()
}

func (b *BuffConn)Read(p []byte) (int, error) {
    if len(b.buf) > 0 {
	n := copy(p, b.buf)
	b.buf = b.buf[n:]
	if len(b.buf) == 0 {
	    b.buf = nil
	}
	return n, nil
    }
    b.buf = []byte{}
    buf := make([]byte, 4096)
    r, err := b.conn.Read(buf)
    if err != nil {
	return 0, err
    }
    b.buf = append(b.buf, buf[:r]...)
    return b.Read(p)
}

func (b *BuffConn)Write(p []byte) (int, error) {
    return b.conn.Write(p)
}

func (b *BuffConn)Close() error {
    return b.conn.Close()
}

// BuffConn utils
func getHttpHeader(b *BuffConn) (string, []string) {
    status := string(b.ReadLine())
    hdr := []string{}
    line := b.ReadLine()
    for len(line) > 0 {
	hdr = append(hdr, string(line))
	line = b.ReadLine()
    }
    return status, hdr
}

//
type ProxyConnection struct {
    bconn *BuffConn
    p *Proxy
    closed bool
}

func NewProxyConnection(p *Proxy, conn net.Conn) *ProxyConnection {
    bconn := &BuffConn{
	conn: conn,
	buf: nil,
	eof: false,
    }
    c := &ProxyConnection{
	bconn: bconn,
	p: p,
	closed: false,
    }
    return c
}

func (c *ProxyConnection)Close() {
    if c.closed {
	return
    }
    c.bconn.Close()
    c.closed = true
}

func (c *ProxyConnection)getRequest() (string, []string) {
    return getHttpHeader(c.bconn)
}

func (c *ProxyConnection)doConnect(host string) {
    conn, err := session.Dial(host)
    if err != nil {
	log.Printf("doConnect: %v\n", err)
	// disconnect
	c.Close()
	return
    }
    defer conn.Close()

    // ok, we get remote connection
    c.bconn.Write([]byte("HTTP/1.0 200 Established\r\n\r\n"))

    iorelay.Relay(c.bconn, conn)
}

func (c *ProxyConnection)doGet(path string, hdr []string) {
    u, err := url.Parse(path)
    if err != nil {
	log.Printf("doGet: %v\n", err)
	c.Close()
	return
    }
    host := u.Hostname()
    port := u.Port()
    if port == "" {
	port = "80"
    }
    // connect
    conn, err := session.Dial(host + ":" + port)
    if err != nil {
	log.Printf("doGet: %v\n", err)
	c.Close()
	return
    }
    defer conn.Close()

    resphdr := []string{}
    // send request
    conn.Write([]byte("GET " + u.RequestURI() + " HTTP/1.1\r\n"))
    for _, h := range hdr {
	if strings.Index(h, "Proxy-") == 0 {
	    resphdr = append(resphdr, h)
	    continue
	}
	conn.Write([]byte(h + "\r\n"))
	log.Println(h)
    }
    conn.Write([]byte("Connection: close\r\n"))
    conn.Write([]byte("\r\n"))

    bconn := &BuffConn{
	conn: conn,
	buf: nil,
	eof: false,
    }
    // recieve everything
    {
	resp, hdr := getHttpHeader(bconn)
	log.Println(resp)
	cl := ""
	te := ""
	// TODO
	c.bconn.Write([]byte(resp + "\r\n"))
	for _, h := range hdr {
	    log.Println(h)
	    c.bconn.Write([]byte(h + "\r\n"))
	    if strings.Index(h, "Content-Length: ") == 0 {
		cl = h[len("Content-Length: "):]
	    }
	    if strings.Index(h, "Transfer-Encoding: ") == 0 {
		te = h[len("Transfer-Encoding: "):]
	    }
	}
	// restore Proxy-* header
	for _, h := range resphdr {
	    c.bconn.Write([]byte(h + "\r\n"))
	}
	c.bconn.Write([]byte("\r\n"))
	// check type
	if te == "chunked" {
	    // chunked mode
	    log.Println("chunked")
	    // TODO
	}
	if cl != "" {
	    // no need to care about Content-Length now
	    log.Printf("length: %s\n", cl)
	    // TODO
	}
	// Copy is just ok right now
	io.Copy(c.bconn, bconn)
    }
}

func (c *ProxyConnection)process() {
    log.Println("enter process")
    for c.closed == false {
	// reading request
	req, hdr := c.getRequest()
	if req == "" {
	    c.Close()
	    continue
	}
	log.Println(req)
	log.Println(hdr)
	// check req
	// method URI version
	s := strings.Split(req, " ")
	if len(s) < 3 {
	    log.Printf("request error\n")
	    c.Close()
	    return
	}
	method := s[0]
	path := s[1]
	//version := s[2]
	switch method {
	case "CONNECT":
	    c.p.OnConnect(path, hdr)
	    c.doConnect(path)
	case "GET":
	    c.p.OnGet(path, hdr)
	    c.doGet(path, hdr)
	}
    }
    log.Println("exit process")
}
