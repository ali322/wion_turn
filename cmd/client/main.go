package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/pion/logging"
	"github.com/pion/turn/v3"
)

func clientConf() *turn.ClientConfig {
	host := flag.String("host", "", "TURN Server name.")
	port := flag.Int("port", 3478, "Listening port.")
	user := flag.String("user", "", "A pair of username and password (e.g. \"user=pass\")")
	realm := flag.String("realm", "pion.ly", "Realm (defaults to \"pion.ly\")")
	isTcp := flag.Bool("is-tcp", false, "use tcp instead of udp")
	flag.Parse()

	if len(*host) == 0 {
		log.Fatalf("'host' is required")
	}

	if len(*user) == 0 {
		log.Fatalf("'user' is required")
	}

	cred := strings.SplitN(*user, "=", 2)
	turnServerAddr := fmt.Sprintf("%s:%d", *host, *port)

	if *isTcp {
		tcpConn, err := net.Dial("tcp", turnServerAddr)
		if err != nil {
			log.Panicf("Failed to connect to TURN server: %s", err)
		}
		conn := turn.NewSTUNConn(tcpConn)

		return &turn.ClientConfig{
			STUNServerAddr: turnServerAddr,
			TURNServerAddr: turnServerAddr,
			Conn:           conn,
			Username:       cred[0],
			Password:       cred[1],
			Realm:          *realm,
			LoggerFactory:  logging.NewDefaultLoggerFactory(),
		}
	} else {
		// TURN client won't create a local listening socket by itself.
		conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
		if err != nil {
			log.Panicf("Failed to listen: %s", err)
		}
		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				log.Panicf("Failed to close connection: %s", closeErr)
			}
		}()
		return &turn.ClientConfig{
			STUNServerAddr: turnServerAddr,
			TURNServerAddr: turnServerAddr,
			Conn:           conn,
			Username:       cred[0],
			Password:       cred[1],
			Realm:          *realm,
			LoggerFactory:  logging.NewDefaultLoggerFactory(),
		}
	}
}

func main() {
	cfg := clientConf()
	client, err := turn.NewClient(cfg)
	if err != nil {
		log.Panicf("Failed to create TURN client: %s", err)
	}
	defer client.Close()

	// Start listening on the conn provided.
	err = client.Listen()
	if err != nil {
		log.Panicf("Failed to listen: %s", err)
	}

	testClient(client)
}

func testClient(client *turn.Client) {
	ping := flag.Bool("ping", false, "Run ping test")
	// Allocate a relay socket on the TURN server. On success, it
	// will return a net.PacketConn which represents the remote
	// socket.
	relayConn, err := client.Allocate()
	if err != nil {
		log.Panicf("Failed to allocate: %s", err)
	}
	defer func() {
		if closeErr := relayConn.Close(); closeErr != nil {
			log.Panicf("Failed to close connection: %s", closeErr)
		}
	}()

	// The relayConn's local address is actually the transport
	// address assigned on the TURN server.
	log.Printf("relayed-address=%s", relayConn.LocalAddr().String())

	//If you provided `-ping`, perform a ping test against the
	//	relayConn we have just allocated.
	if *ping {
		err = doPingTest(client, relayConn)
		if err != nil {
			log.Panicf("Failed to ping: %s", err)
		}
	}
}

func doPingTest(client *turn.Client, relayConn net.PacketConn) error {
	// Send BindingRequest to learn our external IP
	mappedAddr, err := client.SendBindingRequest()
	if err != nil {
		return err
	}

	// Set up pinger socket (pingerConn)
	pingerConn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		log.Panicf("Failed to listen: %s", err)
	}
	defer func() {
		if closeErr := pingerConn.Close(); closeErr != nil {
			log.Panicf("Failed to close connection: %s", closeErr)
		}
	}()

	// Punch a UDP hole for the relayConn by sending a data to the mappedAddr.
	// This will trigger a TURN client to generate a permission request to the
	// TURN server. After this, packets from the IP address will be accepted by
	// the TURN server.
	_, err = relayConn.WriteTo([]byte("Hello"), mappedAddr)
	if err != nil {
		return err
	}

	// Start read-loop on pingerConn
	go func() {
		buf := make([]byte, 1600)
		for {
			n, from, pingerErr := pingerConn.ReadFrom(buf)
			if pingerErr != nil {
				break
			}

			msg := string(buf[:n])
			if sentAt, pingerErr := time.Parse(time.RFC3339Nano, msg); pingerErr == nil {
				rtt := time.Since(sentAt)
				log.Printf("%d bytes from from %s time=%d ms\n", n, from.String(), int(rtt.Seconds()*1000))
			}
		}
	}()

	// Start read-loop on relayConn
	go func() {
		buf := make([]byte, 1600)
		for {
			n, from, readerErr := relayConn.ReadFrom(buf)
			if readerErr != nil {
				break
			}

			// Echo back
			if _, readerErr = relayConn.WriteTo(buf[:n], from); readerErr != nil {
				break
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// Send 10 packets from relayConn to the echo server
	for i := 0; i < 10; i++ {
		msg := time.Now().Format(time.RFC3339Nano)
		_, err = pingerConn.WriteTo([]byte(msg), relayConn.LocalAddr())
		if err != nil {
			return err
		}

		// For simplicity, this example does not wait for the pong (reply).
		// Instead, sleep 1 second.
		time.Sleep(time.Second)
	}

	return nil
}

func setupSignalingChannel(addrCh chan string, signaling bool, relayAddr string) {
	addr := "127.0.0.1:5000"
	if signaling {
		go func() {
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				log.Panicf("Failed to create signaling server: %s", err)
			}
			defer listener.Close() //nolint:errcheck,gosec
			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Panicf("Failed to accept: %s", err)
				}

				go func() {
					var message string
					message, err = bufio.NewReader(conn).ReadString('\n')
					if err != nil {
						log.Panicf("Failed to read from relayAddr: %s", err)
					}
					addrCh <- message[:len(message)-1]
				}()

				if _, err = conn.Write([]byte(fmt.Sprintf("%s\n", relayAddr))); err != nil {
					log.Panicf("Failed to write relayAddr: %s", err)
				}
			}
		}()
	} else {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Panicf("Error dialing: %s", err)
		}
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Panicf("Failed to read relayAddr: %s", err)
		}
		addrCh <- message[:len(message)-1]
		if _, err = conn.Write([]byte(fmt.Sprintf("%s\n", relayAddr))); err != nil {
			log.Panicf("Failed to write relayAddr: %s", err)
		}
	}
}

func testClientWithSignal(client *turn.Client) {
	signaling := flag.Bool("signaling", false, "Whether to start signaling server otherwise connect")
	// Allocate a relay socket on the TURN server. On success, it
	// will return a client.TCPAllocation which represents the remote
	// socket.
	allocation, err := client.AllocateTCP()
	if err != nil {
		log.Panicf("Failed to allocate: %s", err)
	}
	defer func() {
		if closeErr := allocation.Close(); closeErr != nil {
			log.Panicf("Failed to close connection: %s", closeErr)
		}
	}()

	log.Printf("relayed-address=%s", allocation.Addr())

	addrCh := make(chan string, 5)
	setupSignalingChannel(addrCh, *signaling, allocation.Addr().String())

	// Get peer address
	peerAddrStr := <-addrCh
	peerAddr, err := net.ResolveTCPAddr("tcp", peerAddrStr)
	if err != nil {
		log.Panicf("Failed to resolve peer address: %s", err)
	}

	log.Printf("Received peer address: %s", peerAddrStr)

	buf := make([]byte, 4096)
	var n int
	if *signaling {
		conn, err := allocation.DialTCP("tcp", nil, peerAddr)
		if err != nil {
			log.Panicf("Failed to dial: %s", err)
		}

		if _, err = conn.Write([]byte("hello!")); err != nil {
			log.Panicf("Failed to write: %s", err)
		}

		n, err = conn.Read(buf)
		if err != nil {
			log.Panicf("Failed to read from relay connection: %s", err)
		}

		if err := conn.Close(); err != nil {
			log.Panicf("Failed to close: %s", err)
		}
	} else {
		if err := client.CreatePermission(peerAddr); err != nil {
			log.Panicf("Failed to create permission: %s", err)
		}

		conn, err := allocation.AcceptTCP()
		if err != nil {
			log.Panicf("Failed to accept TCP connection: %s", err)
		}

		log.Printf("Accepted connection from: %s", conn.RemoteAddr())

		n, err = conn.Read(buf)
		if err != nil {
			log.Panicf("Failed to read from relay conn: %s", err)
		}

		if _, err := conn.Write([]byte("hello back!")); err != nil {
			log.Panicf("Failed to write: %s", err)
		}

		if err := conn.Close(); err != nil {
			log.Panicf("Failed to close: %s", err)
		}
	}

	log.Printf("Read message: %s", string(buf[:n]))
}
