package server

import (
	"io"
	"net"

	"github.com/gitcfly/tunnet/encrypt"
	"github.com/gitcfly/tunnet/logging"
	"github.com/gitcfly/tunnet/protocol"
	"github.com/gitcfly/tunnet/util"
	"github.com/xitongsys/ethernet-go/header"
)

var USERCHANBUFFERSIZE = 1024
var READBUFFERSIZE = 65535

type User struct {
	Client        string
	Protocol      string
	RemoteTunIp   string
	LocalTunIp    string
	Token         string
	Key           string
	TunToConnChan chan []byte
	ConnToTunChan chan []byte
	Conn          net.Conn
	Logout        func(client string)
}

func NewUser(client string, protocol string, tun string, token string, conn net.Conn, logout func(string)) *User {
	key := string(encrypt.GetAESKey([]byte(token)))
	return &User{
		Client:        client,
		Protocol:      protocol,
		LocalTunIp:    tun,
		RemoteTunIp:   tun,
		Token:         token,
		Key:           key,
		TunToConnChan: make(chan []byte, USERCHANBUFFERSIZE),
		ConnToTunChan: make(chan []byte, USERCHANBUFFERSIZE),
		Conn:          conn,
		Logout:        logout,
	}
}

func (user *User) Start() {
	//encryptKey := encrypt.GetAESKey([]byte(user.Token))
	//read from client, write to channel
	buf := make([]byte, READBUFFERSIZE)
	go func() {
		for {
			var err error
			var data []byte
			var n int

			if user.Protocol == "tcp" {
				data, err = util.ReadPacket(user.Conn)

			} else if user.Protocol == "ptcp" {
				if n, err = user.Conn.Read(buf); err == nil && n > 1 && buf[0] == protocol.PTCP_PACKETTYPE_DATA {

					data = buf[1:n]
				}

			} else {
				if n, err = user.Conn.Read(buf); err == nil && n > 0 {
					data = buf[:n]
				}
			}
			logging.Log.Debugf("From %v read data,len=%v", user.Client, len(data))
			if err != nil {
				if err != io.EOF {
					logging.Log.Errorf("From %v, err:%v", user.Client, err)
					continue
				}
				user.Close()
				logging.Log.Errorf("From %v, err:%v", user.Client, err)
				return
			}

			if ln := len(data); ln > 0 {
				//if data, err = encrypt.DecryptAES(data, encryptKey); err == nil {
				if proto, src, dst, err := header.GetBase(data); err == nil {
					remoteIp, _ := header.ParseAddr(src)
					user.RemoteTunIp = remoteIp
					Snat(data, user.LocalTunIp)
					user.ConnToTunChan <- data
					logging.Log.Debugf("From %v client: client:%v, protocol:%v, len:%v, src:%v, dst:%v", user.Protocol, user.Client, proto, ln, src, dst)
				} else {
					logging.Log.Errorf("From %v, src=%v, dest=%v, err:%v", user.Client, src, dst, err)
				}
				//}
			}
		}
	}()

	//read from channel, write to client
	go func() {
		for {
			datas, ok := <-user.TunToConnChan
			if !ok {
				user.Close()
				return
			}
			data := datas
			if ln := len(data); ln > 0 {
				if proto, src, dst, err := header.GetBase(data); err == nil {
					Dnat(data, user.RemoteTunIp)
					//if endata, err := encrypt.EncryptAES(data, encryptKey); err == nil {
					if user.Protocol == "tcp" {
						_, err = util.WritePacket(user.Conn, data)

					} else if user.Protocol == "ptcp" {
						packet := append([]byte{protocol.PTCP_PACKETTYPE_DATA}, data...)
						_, err = user.Conn.Write(packet)

					} else {
						_, err = user.Conn.Write(data)
					}

					if err != nil {
						user.Close()
						return
					}
					logging.Log.Debugf("To %v client: client:%v, protocol:%v, len:%v, src:%v, dst:%v", user.Protocol, user.Client, proto, ln, src, dst)
					//}
				}
			}
		}
	}()
}

func (user *User) Close() {
	go func() {
		defer func() {
			recover()
		}()
		close(user.TunToConnChan)
	}()

	go func() {
		defer func() {
			recover()
		}()
		close(user.ConnToTunChan)
	}()

	go func() {
		defer func() {
			recover()
		}()
		user.Conn.Close()
	}()

	user.Logout(user.Client)
}
