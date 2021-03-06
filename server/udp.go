package server

import (
	"fmt"
	"net"
	"time"

	"github.com/gitcfly/tunnet/cache"
	"github.com/gitcfly/tunnet/config"
	"github.com/gitcfly/tunnet/logging"
	"github.com/xitongsys/ethernet-go/header"
)

var UDPCHANBUFFERSIZE = 1024

type UdpServer struct {
	Addr          string
	UdpConn       *net.UDPConn
	LoginManager  *LoginManager
	TunToConnChan chan []byte
	ConnToTunChan chan []byte
	RouteMap      *cache.Cache
}

func NewUdpServer(cfg *config.Config, loginManager *LoginManager) (*UdpServer, error) {
	addr := cfg.ServerAddr
	add, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid address", addr)
	}

	conn, err := net.ListenUDP("udp", add)
	if err != nil {
		return nil, err
	}

	return &UdpServer{
		Addr:          addr,
		UdpConn:       conn,
		LoginManager:  loginManager,
		TunToConnChan: make(chan []byte, UDPCHANBUFFERSIZE),
		ConnToTunChan: make(chan []byte, UDPCHANBUFFERSIZE),
		RouteMap:      cache.NewCache(time.Minute * 10),
	}, nil
}

func (us *UdpServer) Start() error {
	logging.Log.Info("UdpServer started")
	us.LoginManager.TunServer.StartClient("udp", us.ConnToTunChan, us.TunToConnChan)

	//from conn to tun
	go func() {
		defer func() {
			recover()
		}()

		data := make([]byte, us.LoginManager.TunServer.TunConn.GetMtu()*2)
		for {
			if n, caddr, err := us.UdpConn.ReadFromUDP(data); err == nil && n > 0 {
				if protocol, src, dst, err := header.GetBase(data[:n]); err == nil {
					key := protocol + ":" + src + ":" + dst
					us.RouteMap.Put(key, caddr.String())
					us.ConnToTunChan <- data[:n]
					logging.Log.Debugf("UdpFromClient: client:%v, protocol:%v, src:%v, dst:%v", caddr, protocol, src, dst)
				}
			}
		}

	}()

	//from tun to conn
	go func() {
		defer func() {
			recover()
		}()

		for {
			data, ok := <-us.TunToConnChan
			if ok {
				if protocol, src, dst, err := header.GetBase(data); err == nil {
					key := protocol + ":" + dst + ":" + src
					clientAddrI := us.RouteMap.Get(key)
					if clientAddrI != nil {
						clientAddr := clientAddrI.(string)
						if add, err := net.ResolveUDPAddr("udp", clientAddr); err == nil {
							us.UdpConn.WriteToUDP(data, add)
							logging.Log.Debugf("UdpToClient: client:%v, protocol:%v, src:%v, dst:%v", clientAddr, protocol, src, dst)
						}
					}
				}
			}
		}

	}()

	return nil
}

func (us *UdpServer) Stop() error {
	logging.Log.Info("UdpServer stopped")

	go func() {
		defer func() {
			recover()
		}()

		close(us.TunToConnChan)
	}()

	go func() {
		defer func() {
			recover()
		}()

		close(us.ConnToTunChan)
	}()

	go func() {
		defer func() {
			recover()
		}()

		us.UdpConn.Close()
	}()
	return nil
}
