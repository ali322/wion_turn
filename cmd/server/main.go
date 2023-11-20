package main

import (
	"app/lib/config"
	"app/lib/logger"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/pion/stun"
	"github.com/pion/turn/v3"
	"go.uber.org/zap"
)

type stunLogger struct {
	net.PacketConn
}

func (s *stunLogger) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if n, err = s.PacketConn.WriteTo(p, addr); err == nil && stun.IsMessage(p) {
		msg := &stun.Message{Raw: p}
		if err = msg.Decode(); err != nil {
			return
		}
		zap.L().Info("Outbound",
			zap.String("STUN", msg.String()))
		fmt.Printf("Outbound STUN: %s \n", msg.String())
	}

	return
}

func (s *stunLogger) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if n, addr, err = s.PacketConn.ReadFrom(p); err == nil && stun.IsMessage(p) {
		msg := &stun.Message{Raw: p}
		if err = msg.Decode(); err != nil {
			return
		}
		zap.L().Info("Inbound",
			zap.String("STUN", msg.String()))
		fmt.Printf("Inbound STUN: %s \n", msg.String())
	}

	return
}

var version = ""

var printVersion bool

func init() {
	flag.BoolVar(&printVersion, "version", false, "print program build version")
	flag.Parse()
}

func serverConf() turn.ServerConfig {
	realm := config.App.Realm
	users := config.App.Users
	publicIP := config.App.PublicIP
	usersMap := map[string][]byte{}
	for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(users, -1) {
		usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], realm, kv[2])
	}
	if config.App.IsTCP {
		fmt.Println("is tcp")
		tcpListener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%s", config.App.Port))
		if err != nil {
			log.Fatalf("Failed to create TURN server listener: %s", err)
		}
		return turn.ServerConfig{
			Realm: realm,
			AuthHandler: func(username, realm string, srcAddr net.Addr) (key []byte, ok bool) {
				if key, ok := usersMap[username]; ok {
					return key, true
				}
				return nil, false
			},
			ListenerConfigs: []turn.ListenerConfig{
				{
					Listener: tcpListener,
					RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
						RelayAddress: net.ParseIP(publicIP),
						Address:      "0.0.0.0",
					},
				},
			},
		}
	} else {
		fmt.Println("is udp")
		udpListener, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%s", config.App.Port))
		if err != nil {
			log.Fatalf("Failed to create TURN server listener: %s", err)
		}
		return turn.ServerConfig{
			Realm: realm,
			AuthHandler: func(username, realm string, srcAddr net.Addr) (key []byte, ok bool) {
				if key, ok := usersMap[username]; ok {
					return key, true
				}
				return nil, false
			},
			PacketConnConfigs: []turn.PacketConnConfig{
				{
					PacketConn: &stunLogger{udpListener},
					RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
						RelayAddress: net.ParseIP(publicIP),
						Address:      "0.0.0.0",
					},
				},
			},
		}
	}
}

func main() {
	if printVersion {
		println(version)
		os.Exit(0)
	}
	config.ReadAppConf()

	logger := logger.New(filepath.Join(config.App.LogDir, "app.log"))
	defer logger.Sync()
	conf := serverConf()
	s, err := turn.NewServer(conf)
	if err != nil {
		log.Panic(err)
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	if err = s.Close(); err != nil {
		log.Panic(err)
	}
}
