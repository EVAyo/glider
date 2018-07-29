package dns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
)

// UDPMaxLen is the max size of udp dns request.
// https://tools.ietf.org/html/rfc1035#section-4.2.1
// Messages carried by UDP are restricted to 512 bytes (not counting the IP
// or UDP headers).  Longer messages are truncated and the TC bit is set in
// the header.
const UDPMaxLen = 512

// HeaderLen is the length of dns msg header
const HeaderLen = 12

// QR types
const (
	QRQuery    = 0
	QRResponse = 1
)

// QType .
const (
	QTypeA    uint16 = 1  //ipv4
	QTypeAAAA uint16 = 28 ///ipv6
)

// Message format
// https://tools.ietf.org/html/rfc1035#section-4.1
// All communications inside of the domain protocol are carried in a single
// format called a message.  The top level format of message is divided
// into 5 sections (some of which are empty in certain cases) shown below:
//
//     +---------------------+
//     |        Header       |
//     +---------------------+
//     |       Question      | the question for the name server
//     +---------------------+
//     |        Answer       | RRs answering the question
//     +---------------------+
//     |      Authority      | RRs pointing toward an authority
//     +---------------------+
//     |      Additional     | RRs holding additional information
type Message struct {
	// all dns messages should start with a 12 byte dns header
	*Header
	// most dns implementation only support 1 question
	Question   *Question
	Answers    []*RR
	Authority  []*RR
	Additional []*RR

	// used in UnmarshalMessage
	unMarshaled []byte
}

// NewMessage returns a new message
func NewMessage() *Message {
	return &Message{
		Header: &Header{},
	}
}

// SetQuestion sets a question to dns message,
func (m *Message) SetQuestion(q *Question) error {
	m.Question = q
	m.Header.SetQdcount(1)
	return nil
}

// AddAnswer adds an answer to dns message
func (m *Message) AddAnswer(rr *RR) error {
	m.Answers = append(m.Answers, rr)
	return nil
}

// Marshal marshals message struct to []byte
func (m *Message) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	m.Header.SetQdcount(1)
	m.Header.SetAncount(len(m.Answers))

	b, err := m.Header.Marshal()
	if err != nil {
		return nil, err
	}
	buf.Write(b)

	b, err = m.Question.Marshal()
	if err != nil {
		return nil, err
	}
	buf.Write(b)

	// for _, answer := range m.Answers {
	// 	b, err := answer.Marshal()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	buf.Write(b)
	// }

	return buf.Bytes(), nil
}

// UnmarshalMessage unmarshals []bytes to Message
func UnmarshalMessage(b []byte) (*Message, error) {
	msg := NewMessage()
	msg.unMarshaled = b

	fmt.Printf("msg.unMarshaled:\n%s\n", hex.Dump(msg.unMarshaled))

	err := UnmarshalHeader(b[:HeaderLen], msg.Header)
	if err != nil {
		return nil, err
	}

	q := &Question{}
	qLen, err := msg.UnmarshalQuestion(b[HeaderLen:], q)
	if err != nil {
		return nil, err
	}

	msg.SetQuestion(q)

	// resp answers
	rridx := HeaderLen + qLen
	for i := 0; i < int(msg.Header.ANCOUNT); i++ {
		rr := &RR{}
		rrLen, err := msg.UnmarshalRR(rridx, rr)
		if err != nil {
			return nil, err
		}
		msg.AddAnswer(rr)

		rridx += rrLen
	}

	msg.Header.SetAncount(len(msg.Answers))

	return msg, nil
}

// Header format
// https://tools.ietf.org/html/rfc1035#section-4.1.1
// The header contains the following fields:
//
//                                     1  1  1  1  1  1
//       0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                      ID                       |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                    QDCOUNT                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                    ANCOUNT                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                    NSCOUNT                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                    ARCOUNT                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//
type Header struct {
	ID      uint16
	Bits    uint16
	QDCOUNT uint16
	ANCOUNT uint16
	NSCOUNT uint16
	ARCOUNT uint16
}

// NewHeader returns a new dns header
func NewHeader(id uint16, qr int) *Header {
	if id == 0 {
		id = uint16(rand.Uint32())
	}

	h := &Header{ID: id}
	h.SetQR(qr)
	return h
}

// SetQR .
func (h *Header) SetQR(qr int) {
	h.Bits |= uint16(qr) << 15
}

// SetQdcount sets query count, most dns servers only support 1 query per request
func (h *Header) SetQdcount(qdcount int) {
	h.QDCOUNT = uint16(qdcount)
}

// SetAncount sets answers count
func (h *Header) SetAncount(ancount int) {
	h.ANCOUNT = uint16(ancount)
}

func (h *Header) setFlag(QR uint16, Opcode uint16, AA uint16,
	TC uint16, RD uint16, RA uint16, RCODE uint16) {
	h.Bits = QR<<15 + Opcode<<11 + AA<<10 + TC<<9 + RD<<8 + RA<<7 + RCODE
}

// Marshal marshals header struct to []byte
func (h *Header) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, h)
	return buf.Bytes(), err
}

// UnmarshalHeader unmarshals []bytes to Header
func UnmarshalHeader(b []byte, h *Header) error {
	if h == nil {
		return errors.New("unmarshal header must not be nil")
	}

	if len(b) != HeaderLen {
		return errors.New("unmarshal header bytes has an unexpected size")
	}

	h.ID = binary.BigEndian.Uint16(b[:2])
	h.Bits = binary.BigEndian.Uint16(b[2:4])
	h.QDCOUNT = binary.BigEndian.Uint16(b[4:6])
	h.ANCOUNT = binary.BigEndian.Uint16(b[6:8])
	h.NSCOUNT = binary.BigEndian.Uint16(b[8:10])
	h.ARCOUNT = binary.BigEndian.Uint16(b[10:])

	return nil
}

// Question format
// https://tools.ietf.org/html/rfc1035#section-4.1.2
// The question section is used to carry the "question" in most queries,
// i.e., the parameters that define what is being asked.  The section
// contains QDCOUNT (usually 1) entries, each of the following format:
//
//                                     1  1  1  1  1  1
//       0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                                               |
//     /                     QNAME                     /
//     /                                               /
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                     QTYPE                     |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                     QCLASS                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type Question struct {
	QNAME  string
	QTYPE  uint16
	QCLASS uint16
}

// NewQuestion returns a new dns question
func NewQuestion(qtype uint16, domain string) *Question {
	return &Question{
		QNAME:  domain,
		QTYPE:  qtype,
		QCLASS: 1,
	}
}

// Marshal marshals Question struct to []byte
func (q *Question) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	buf.Write(MarshalDomain(q.QNAME))
	binary.Write(&buf, binary.BigEndian, q.QTYPE)
	binary.Write(&buf, binary.BigEndian, q.QCLASS)

	return buf.Bytes(), nil
}

// UnmarshalQuestion unmarshals []bytes to Question
func (m *Message) UnmarshalQuestion(b []byte, q *Question) (n int, err error) {
	if q == nil {
		return 0, errors.New("unmarshal question must not be nil")
	}

	domain, idx := m.GetDomain(b)
	q.QNAME = domain
	q.QTYPE = binary.BigEndian.Uint16(b[idx : idx+2])
	q.QCLASS = binary.BigEndian.Uint16(b[idx+2 : idx+4])

	return idx + 3 + 1, nil
}

// RR format
// https://tools.ietf.org/html/rfc1035#section-3.2.1
// https://tools.ietf.org/html/rfc1035#section-4.1.3
// The answer, authority, and additional sections all share the same
// format: a variable number of resource records, where the number of
// records is specified in the corresponding count field in the header.
// Each resource record has the following format:
//
//                                     1  1  1  1  1  1
//       0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                                               |
//     /                                               /
//     /                      NAME                     /
//     |                                               |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                      TYPE                     |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                     CLASS                     |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                      TTL                      |
//     |                                               |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//     |                   RDLENGTH                    |
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--|
//     /                     RDATA                     /
//     /                                               /
//     +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
type RR struct {
	NAME     string
	TYPE     uint16
	CLASS    uint16
	TTL      uint32
	RDLENGTH uint16
	RDATA    []byte

	IP string
}

// NewRR returns a new dns rr
func NewRR() *RR {
	rr := &RR{}
	return rr
}

// UnmarshalRR unmarshals []bytes to RR
func (m *Message) UnmarshalRR(start int, rr *RR) (n int, err error) {
	if rr == nil {
		return 0, errors.New("unmarshal question must not be nil")
	}

	p := m.unMarshaled[start:]

	fmt.Printf("rr bytes:\n%s\n", hex.Dump(p[:10]))

	domain, n := m.GetDomain(p)
	rr.NAME = domain

	if len(p) <= n+10 {
		return 0, errors.New("not enough data")
	}

	rr.TYPE = binary.BigEndian.Uint16(p[n:])
	rr.CLASS = binary.BigEndian.Uint16(p[n+2:])
	rr.TTL = binary.BigEndian.Uint32(p[n+4:])
	rr.RDLENGTH = binary.BigEndian.Uint16(p[n+8:])
	rr.RDATA = p[n+10 : n+10+int(rr.RDLENGTH)]

	if rr.TYPE == QTypeA {
		rr.IP = net.IP(rr.RDATA[:net.IPv4len]).String()
	} else if rr.TYPE == QTypeAAAA {
		rr.IP = net.IP(rr.RDATA[:net.IPv6len]).String()
	}

	n = n + 10 + int(rr.RDLENGTH)

	fmt.Printf("rr: %+#v\n", rr)

	return n, nil
}

// MarshalDomain marshals domain string struct to []byte
func MarshalDomain(domain string) []byte {
	var buf bytes.Buffer

	for _, seg := range strings.Split(domain, ".") {
		binary.Write(&buf, binary.BigEndian, byte(len(seg)))
		binary.Write(&buf, binary.BigEndian, []byte(seg))
	}
	binary.Write(&buf, binary.BigEndian, byte(0x00))

	return buf.Bytes()
}

// GetDomain gets domain from bytes
func (m *Message) GetDomain(b []byte) (string, int) {
	var idx, size int
	var labels = []string{}

	for {
		// https://tools.ietf.org/html/rfc1035#section-4.1.4
		// "Message compression",
		// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
		// | 1  1|                OFFSET                   |
		// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
		if b[idx]&0xC0 == 0xC0 {
			offset := binary.BigEndian.Uint16(b[idx : idx+2])
			lable := m.GetDomainByPoint(int(offset & 0x3F))
			labels = append(labels, lable)
			idx += 2
			break
		} else {
			size = int(b[idx])
			if size == 0 {
				idx++
				break
			}
			labels = append(labels, string(b[idx+1:idx+size+1]))
			idx += (size + 1)
		}
	}

	domain := strings.Join(labels, ".")
	return domain, idx
}

// GetDomainByPoint gets domain from
func (m *Message) GetDomainByPoint(offset int) string {
	domain, _ := m.GetDomain(m.unMarshaled[offset:])
	fmt.Printf("GetDomainByPoint: %02x\n", offset)
	return domain
}