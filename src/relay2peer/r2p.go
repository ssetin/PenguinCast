// Package relay2peer - implement p2p connections
package relay2peer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gortc/stun"
)

const (
	defaultSTUNServer   = "stun1.l.google.com:19302"
	msgRequestListener  = "helloRelay"
	msgResponseListener = "niceToMeetYouListener"
	msgRequestRelay     = "helloListener"
	msgResponseRelay    = "niceToMeetYouRelay"
	keepAliveTimeOut    = 500
	searchingTimeOut    = 10000
	udpPackSize         = 512
)

// PeerConnection implement net.Conn for peer connection
type PeerConnection struct {
	*net.UDPConn
	peerAddr  *net.UDPAddr
	DebugMode bool
}

// Write writes packet to peerAddr using udp (WriteToUDP)
func (p *PeerConnection) Write(b []byte) (int, error) {
	if p.peerAddr == nil {
		return -1, errors.New("no peer address")
	}

	end := 0
	bytesLen := len(b)
	totalWrited := 0

	var msg UDPMessage

	// we have to split data into udpPackSize (512 bytes) packets,
	// because larger packages most likely will not be delivered.
	// Also we have to wrap data into UDPMessage
	for i := 0; i < bytesLen; i += udpPackSize - 8 {
		end = i + udpPackSize - 8
		if end > bytesLen {
			end = bytesLen
		}

		msg.FillStreamData(b[i:end])
		msgBytes, err := msg.Marshal()
		if err != nil {
			return totalWrited, err
		}

		n, err := p.WriteToUDP(msgBytes, p.peerAddr)
		if err != nil {
			return totalWrited, err
		}
		totalWrited += n
		if p.DebugMode {
			log.Printf("Writed %d bytes to %s\n", n, p.peerAddr.String())
		}
	}

	return totalWrited, nil
}

// Read reads packet using udp (ReadFromUDP)
func (p *PeerConnection) Read(b []byte) (int, error) {
	if p.peerAddr == nil {
		return -1, errors.New("no peer address")
	}

	n, addr, err := p.ReadFromUDP(b)
	if addr.String() != p.peerAddr.String() {
		return -1, errors.New("packet from unknown peer")
	}

	if p.DebugMode {
		log.Printf("Readed %d bytes from %s\n", n, p.peerAddr.String())
	}
	return n, err
}

// =========================================================================

// UDPMessage wrapped udp packet
type UDPMessage struct {
	data     []byte
	crc      uint32
	isStream bool
}

// IsStreamMessage is that message contain stream data
func (m *UDPMessage) IsStreamMessage() bool {
	return m.isStream
}

// IsSTUNMessage determine is it message from STUN server
func (m *UDPMessage) IsSTUNMessage() bool {
	return stun.IsMessage(m.data)
}

// FillGeneralData fills data, clear crc and mark message as general
func (m *UDPMessage) FillGeneralData(data []byte) {
	m.isStream = false
	m.crc = 0
	m.data = data
}

// FillStreamData fills data, calculate crc and mark message as stream
func (m *UDPMessage) FillStreamData(data []byte) {
	m.isStream = true
	m.crc = crc32.ChecksumIEEE(data)
	m.data = data
}

// String returns data as string
func (m *UDPMessage) String() string {
	return string(m.data)
}

// Marshal returns UDPMessage as []byte
// In case of stream message: -=[crc]=-data
func (m *UDPMessage) Marshal() ([]byte, error) {
	if m.isStream {
		result := make([]byte, 0, len(m.data)+8)

		crcBuf := new(bytes.Buffer)
		binary.Write(crcBuf, binary.LittleEndian, m.crc)

		result = append(result, []byte("-=")...)
		result = append(result, crcBuf.Bytes()...)
		result = append(result, []byte("=-")...)
		result = append(result, m.data...)

		return result, nil
	}
	return m.data, nil
}

// UnMarshal fills UDPMessage from []byte
// In case of stream message: -=[crc]=-data
func (m *UDPMessage) UnMarshal(msg []byte) error {
	l := len(msg)
	if l <= 8 {
		m.FillGeneralData(msg)
		return nil
	}
	header := msg[0:8]

	if string(header[0:2]) == "-=" && string(header[6:8]) == "=-" {
		binary.Read(bytes.NewReader(header[2:6]), binary.LittleEndian, &m.crc)
		realCrc := crc32.ChecksumIEEE(msg[8:l])
		if realCrc == m.crc {
			m.data = msg[8:l]
			m.isStream = true
		} else {
			m.FillGeneralData(msg)
			return errors.New("wrong crc")
		}
	} else {
		m.FillGeneralData(msg)
		return nil
	}

	return nil
}

// NewUDPMessage return new *UDPMessage
func NewUDPMessage(data []byte) *UDPMessage {
	return &UDPMessage{data: data, crc: 0, isStream: false}
}

// UDPFrame consist of UDPMessage and peer address
type UDPFrame struct {
	data UDPMessage
	addr *net.UDPAddr
}

// =========================================================================

// PeerConnector responsible for establishing p2p connections
type PeerConnector struct {
	// STUN server address
	stunHost string
	// client public address
	publicAddr stun.XORMappedAddress
	// address of the listener or relay candidate to connect
	peerAddr *net.UDPAddr
	// latency in reading data from the server. the smaller the value,
	// the greater the likelihood that the client will become a relay point
	latency int64
	// isRelayPoint
	isRelayPoint bool

	// radio\managing server
	srvHost string
	// mount name
	mountName string
}

// Init ...
func (p *PeerConnector) Init(mountName, srvRadioHost, stunURL string) {
	if stunURL > "" {
		p.stunHost = stunURL
	} else {
		p.stunHost = defaultSTUNServer
	}
	p.srvHost = srvRadioHost
	p.mountName = mountName
	p.peerAddr = nil
}

// SetIsRelayPoint mark peer as relay point
func (p *PeerConnector) SetIsRelayPoint(isRelayPoint bool) {
	p.isRelayPoint = isRelayPoint
}

// reportToServer tells managing server information about peer
func (p *PeerConnector) reportToServer() error {
	manConn, err := net.Dial("tcp", p.srvHost)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(manConn)

	writer.WriteString("GET /Pi HTTP/1.0\r\n")
	if p.peerAddr != nil {
		writer.WriteString("Connected: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + ",")
		writer.WriteString(p.peerAddr.String() + "\r\n\r\n")
		writer.Flush()
		return nil
	}

	writer.WriteString("Mount: " + p.mountName + "\r\n")
	writer.WriteString("MyAddr: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + "\r\n")
	if p.isRelayPoint {
		writer.WriteString("Latency: ")
		lat := int(atomic.LoadInt64(&p.latency))
		writer.WriteString(strconv.Itoa(lat))
		writer.WriteString("\r\nFlag: relay\r\n")
	}
	writer.WriteString("\r\n")
	writer.Flush()

	reader := bufio.NewReader(manConn)
	headers, err := processHTTPHeaders(reader)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	if adr, ok := headers["Address"]; ok {
		p.peerAddr, err = net.ResolveUDPAddr("udp", adr)
	}

	if err != nil {
		return err
	}

	return nil
}

// computeMyIP compute peer public address and report to managing server
func (p *PeerConnector) computeMyIP(msg []byte) error {
	m := new(stun.Message)
	m.Raw = msg
	err := m.Decode()
	if err != nil {
		return err
	}
	var xorAddr stun.XORMappedAddress
	if getErr := xorAddr.GetFrom(m); getErr != nil {
		return err
	}
	if p.publicAddr.String() != xorAddr.String() {
		p.publicAddr = xorAddr
	}
	p.reportToServer()
	return nil
}

// UpdateLatency ...
func (p *PeerConnector) UpdateLatency(newlat int64) {
	atomic.StoreInt64(&p.latency, newlat)
}

// isThatFromPeer returns false if this message not from peer
func (p *PeerConnector) isThatFromPeer(msg *UDPFrame) bool {
	if p.peerAddr == nil || msg == nil {
		return false
	}
	if msg.addr.String() == p.peerAddr.String() {
		return true
	}
	return false
}

// GetConnection returns ready for transmitting peer connection
func (p *PeerConnector) GetConnection() chan *PeerConnection {
	udpConnChan := make(chan *PeerConnection)

	go func(udpConnChan chan *PeerConnection) {
		var timeoutTicker *time.Ticker
		// signal that connection established
		// and listen packets inside connector no more needed
		doneChan := make(chan struct{}, 1)

		stunAddr, err := net.ResolveUDPAddr("udp", p.stunHost)
		if err != nil {
			log.Println(err)
			close(udpConnChan)
			return
		}

		conn, err := net.ListenUDP("udp", nil)
		if err != nil {
			log.Println(err)
			close(udpConnChan)
			return
		}

		// udp messages channel
		messageChan := p.listenUDP(conn, doneChan)
		// keep-alive channel
		keepAliveTicker := time.NewTicker(keepAliveTimeOut * time.Millisecond)
		// searching timeout channel
		timeoutTicker = time.NewTicker(searchingTimeOut * time.Millisecond)
		timeout := false

		if p.isRelayPoint {
			timeoutTicker.Stop()
		}
		defer timeoutTicker.Stop()
		defer keepAliveTicker.Stop()

		done := false
		sentResponse := false
		var msg string
		if p.isRelayPoint {
			msg = msgRequestRelay
		} else {
			msg = msgRequestListener
		}

		for {
			if done {
				// done. pass control to the stream reader
				keepAliveTicker.Stop()
				timeoutTicker.Stop()

				doneChan <- struct{}{}
				udpConnChan <- &PeerConnection{conn, p.peerAddr, false}
				close(udpConnChan)
				return
			}

			select {
			case message, ok := <-messageChan:
				if !ok {
					close(udpConnChan)
					return
				}

				switch {
				case p.isThatFromPeer(&message) && !p.isRelayPoint && message.data.String() == msgResponseListener:
					done = true
					continue

				case p.isThatFromPeer(&message) && p.isRelayPoint && message.data.String() == msgResponseRelay:
					done = true
					continue

				case p.isThatFromPeer(&message) && sentResponse && message.data.IsStreamMessage():
					// if we already sent response to relay and start to receive stream data - it's done
					done = true
					continue

				case p.isThatFromPeer(&message) && p.isRelayPoint && message.data.String() == msgRequestListener:
					msg = msgResponseListener

				case p.isThatFromPeer(&message) && !p.isRelayPoint && message.data.String() == msgRequestRelay:
					msg = msgResponseRelay

				case message.data.IsSTUNMessage():
					err = p.computeMyIP(message.data.data)
					if err != nil {
						log.Println(err)
					}

				default:
					log.Println("Unknown msg: " + message.data.String()[:20])
				}

			case <-timeoutTicker.C:
				if timeout {
					// searching time reached
					doneChan <- struct{}{}
					close(udpConnChan)
					return
				}
				timeout = true

			case <-keepAliveTicker.C:
				if p.peerAddr == nil {
					p.sendBindingRequest(conn, stunAddr)
				} else {
					// since peer knows another peer address, it should say him hello
					err = p.sendStr(msg, conn, p.peerAddr)
					if err != nil {
						log.Println(err)
						close(udpConnChan)
						return
					}
					if msg == msgResponseRelay {
						sentResponse = true
					}
				}
			default:
			}
		}
	}(udpConnChan)

	return udpConnChan
}

// sendBindingRequest send packet to STUN server
func (p *PeerConnector) sendBindingRequest(conn *net.UDPConn, addr *net.UDPAddr) error {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	err := p.send(m.Raw, conn, addr)
	if err != nil {
		return fmt.Errorf("binding: %v", err)
	}

	return nil
}

// send bytes to net.UDPAddr
func (p *PeerConnector) send(msg []byte, conn *net.UDPConn, addr *net.UDPAddr) error {
	_, err := conn.WriteToUDP(msg, addr)
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}

	return nil
}

// send string to net.UDPAddr
func (p *PeerConnector) sendStr(msg string, conn *net.UDPConn, addr *net.UDPAddr) error {
	return p.send([]byte(msg), conn, addr)
}

// listenUDP working, while connector trying to find new connection
func (p *PeerConnector) listenUDP(conn *net.UDPConn, doneChan chan struct{}) <-chan UDPFrame {
	messages := make(chan UDPFrame, 5)

	go func(doneChan chan struct{}) {
		for {
			select {
			case <-doneChan:
				close(messages)
				return
			default:
			}

			buf := make([]byte, 1024)
			n, adr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if te, ok := err.(net.Error); ok && te.Timeout() {
					log.Println(te)
				} else {
					log.Println(err)
					close(messages)
					return
				}
			}
			buf = buf[:n]

			var cl UDPFrame
			err = cl.data.UnMarshal(buf)
			if err != nil {
				log.Println(err)
			}
			cl.addr = adr

			select {
			case <-doneChan:
				close(messages)
				return
			default:
				messages <- cl
			}
		}
	}(doneChan)

	return messages
}
