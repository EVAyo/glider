package vmess

import (
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/nadoo/glider/common/log"
	"github.com/nadoo/glider/proxy"
)

// VMess .
type VMess struct {
	dialer proxy.Dialer
	addr   string

	uuid     string
	alterID  int
	security string

	client *Client
}

func init() {
	proxy.RegisterDialer("vmess", NewVMessDialer)
}

// NewVMess returns a vmess proxy.
func NewVMess(s string, dialer proxy.Dialer) (*VMess, error) {
	u, err := url.Parse(s)
	if err != nil {
		log.F("parse url err: %s", err)
		return nil, err
	}

	addr := u.Host
	security := u.User.Username()
	uuid, ok := u.User.Password()
	if !ok {
		// no security type specified, vmess://uuid@server
		uuid = security
		security = ""
	}

	query := u.Query()
	aid := query.Get("alterID")
	if aid == "" {
		aid = "0"
	}

	alterID, err := strconv.ParseUint(aid, 10, 32)
	if err != nil {
		log.F("parse alterID err: %s", err)
		return nil, err
	}

	client, err := NewClient(uuid, security, int(alterID))
	if err != nil {
		log.F("create vmess client err: %s", err)
		return nil, err
	}

	p := &VMess{
		dialer:   dialer,
		addr:     addr,
		uuid:     uuid,
		alterID:  int(alterID),
		security: security,
		client:   client,
	}

	return p, nil
}

// NewVMessDialer returns a vmess proxy dialer.
func NewVMessDialer(s string, dialer proxy.Dialer) (proxy.Dialer, error) {
	return NewVMess(s, dialer)
}

// Addr returns forwarder's address
func (s *VMess) Addr() string {
	if s.addr == "" {
		return s.dialer.Addr()
	}
	return s.addr
}

// NextDialer returns the next dialer
func (s *VMess) NextDialer(dstAddr string) proxy.Dialer { return s.dialer.NextDialer(dstAddr) }

// Dial connects to the address addr on the network net via the proxy.
func (s *VMess) Dial(network, addr string) (net.Conn, string, error) {
	rc, p, err := s.dialer.Dial("tcp", s.addr)
	if err != nil {
		return nil, p, err
	}

	cc, e := s.client.NewConn(rc, addr)
	return cc, p, e
}

// DialUDP connects to the given address via the proxy.
func (s *VMess) DialUDP(network, addr string) (net.PacketConn, net.Addr, error) {
	return nil, nil, errors.New("vmess client does not support udp now")
}
