package fastcgi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/program"
)

type RecordType string

const (
	RecordTypeBeginRequest    RecordType = "begin_request"
	RecordTypeAbortRequest    RecordType = "abort_request"
	RecordTypeEndRequest      RecordType = "end_request"
	RecordTypeParams          RecordType = "params"
	RecordTypeStdin           RecordType = "stdin"
	RecordTypeStdout          RecordType = "stdout"
	RecordTypeStderr          RecordType = "stderr"
	RecordTypeData            RecordType = "data"
	RecordTypeGetValues       RecordType = "get_values"
	RecordTypeGetValuesResult RecordType = "get_values_result"
	RecordTypeUnknownType     RecordType = "unknown_type"
)

func (t RecordType) Encode() (value uint8) {
	switch t {
	case RecordTypeBeginRequest:
		value = 1
	case RecordTypeAbortRequest:
		value = 2
	case RecordTypeEndRequest:
		value = 3
	case RecordTypeParams:
		value = 4
	case RecordTypeStdin:
		value = 5
	case RecordTypeStdout:
		value = 6
	case RecordTypeStderr:
		value = 7
	case RecordTypeData:
		value = 8
	case RecordTypeGetValues:
		value = 9
	case RecordTypeGetValuesResult:
		value = 10
	case RecordTypeUnknownType:
		value = 11

	default:
		program.Panic("unhandled record type %q", t)
	}

	return
}

func (t *RecordType) Decode(value uint8) error {
	switch value {
	case 1:
		*t = RecordTypeBeginRequest
	case 2:
		*t = RecordTypeAbortRequest
	case 3:
		*t = RecordTypeEndRequest
	case 4:
		*t = RecordTypeParams
	case 5:
		*t = RecordTypeStdin
	case 6:
		*t = RecordTypeStdout
	case 7:
		*t = RecordTypeStderr
	case 8:
		*t = RecordTypeData
	case 9:
		*t = RecordTypeGetValues
	case 10:
		*t = RecordTypeGetValuesResult
	case 11:
		*t = RecordTypeUnknownType

	default:
		return fmt.Errorf("invalid record type %d", value)
	}

	return nil
}

type Role string

const (
	RoleResponder  Role = "responder"
	RoleAuthorizer Role = "authorizer"
	RoleFilter     Role = "filter"
)

func (r Role) Encode() (value uint16) {
	switch r {
	case RoleResponder:
		value = 1
	case RoleAuthorizer:
		value = 2
	case RoleFilter:
		value = 3

	default:
		program.Panic("unhandled role %q", r)
	}

	return
}

func (r *Role) Decode(value uint16) error {
	switch value {
	case 1:
		*r = RoleResponder
	case 2:
		*r = RoleAuthorizer
	case 3:
		*r = RoleFilter

	default:
		return fmt.Errorf("invalid role %d", value)
	}

	return nil
}

type ProtocolStatus string

const (
	ProtocolStatusRequestComplete           ProtocolStatus = "request_complete"
	ProtocolStatusCannotMultiplexConnection ProtocolStatus = "cannot_multiplex_connection"
	ProtocolStatusOverloaded                ProtocolStatus = "overloaded"
	ProtocolStatusUnknownRole               ProtocolStatus = "unknown_role"
)

func (p ProtocolStatus) Encode() (value uint8) {
	switch p {
	case ProtocolStatusRequestComplete:
		value = 1
	case ProtocolStatusCannotMultiplexConnection:
		value = 2
	case ProtocolStatusOverloaded:
		value = 3
	case ProtocolStatusUnknownRole:
		value = 4

	default:
		program.Panic("unhandled protocol status %q", p)
	}

	return
}

func (p *ProtocolStatus) Decode(value uint8) error {
	switch value {
	case 1:
		*p = ProtocolStatusRequestComplete
	case 2:
		*p = ProtocolStatusCannotMultiplexConnection
	case 3:
		*p = ProtocolStatusOverloaded
	case 4:
		*p = ProtocolStatusUnknownRole

	default:
		return fmt.Errorf("invalid protocol status %d", value)
	}

	return nil
}

type NameValuePair struct {
	Name  string
	Value string
}

func (p NameValuePair) Encode(buf *bytes.Buffer) error {
	writeLength := func(n int) error {
		if n > math.MaxUint32 {
			return fmt.Errorf("cannot encode length %d (must be lower or "+
				"equal to %d)", n, math.MaxUint32)
		}

		if n < 128 {
			buf.WriteByte(byte(n))
		} else {
			var data [4]byte
			binary.BigEndian.PutUint32(data[:], uint32(n))
			data[0] |= 0x80

			buf.Write(data[:])
		}

		return nil
	}

	if err := writeLength(len(p.Name)); err != nil {
		return err
	}

	if err := writeLength(len(p.Value)); err != nil {
		return err
	}

	buf.WriteString(p.Name)
	buf.WriteString(p.Value)

	return nil
}

func (p *NameValuePair) Decode(data []byte) ([]byte, error) {
	readLength := func() (uint32, error) {
		if len(data) == 0 {
			return 0, fmt.Errorf("truncated length")
		}

		var length uint32

		if data[0]&0x80 == 0 {
			length = uint32(data[0])
			data = data[1:]
		} else {
			if len(data) < 4 {
				return 0, fmt.Errorf("truncated 4-byte length")
			}

			var lengthData [4]byte
			copy(lengthData[:], data[0:4])
			lengthData[0] &= 0x7f

			length = binary.BigEndian.Uint32(lengthData[:])
			data = data[4:]
		}

		return length, nil
	}

	nameLength, err := readLength()
	if err != nil {
		return nil, err
	}

	valueLength, err := readLength()
	if err != nil {
		return nil, err
	}

	if uint32(len(data)) < nameLength {
		return nil, fmt.Errorf("truncated name")
	}

	p.Name = string(data[:nameLength])
	data = data[nameLength:]

	if uint32(len(data)) < valueLength {
		return nil, fmt.Errorf("truncated value")
	}

	p.Value = string(data[:valueLength])
	data = data[valueLength:]

	return data, nil
}

type NameValuePairs []NameValuePair

func (ps NameValuePairs) Encode() ([]byte, error) {
	var buf bytes.Buffer

	for _, p := range ps {
		if err := p.Encode(&buf); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (ps *NameValuePairs) Decode(data []byte) error {
	for len(data) > 0 {
		var p NameValuePair

		var err error
		data, err = p.Decode(data)
		if err != nil {
			return err
		}

		*ps = append(*ps, p)
	}

	return nil
}

type BeginRequestBody struct {
	Role           Role
	KeepConnection bool
}

func (body *BeginRequestBody) Encode() ([]byte, error) {
	var buf [8]byte

	binary.BigEndian.PutUint16(buf[0:], body.Role.Encode())

	var flags byte
	if body.KeepConnection {
		flags |= 1
	}
	buf[2] = flags

	return buf[:], nil
}

func (body *BeginRequestBody) Decode(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("invalid body length")
	}

	role := binary.BigEndian.Uint16(data[0:])
	if err := body.Role.Decode(role); err != nil {
		return err
	}

	if data[2]&1 != 0 {
		body.KeepConnection = true
	}

	return nil
}

type AbortRequestBody struct {
	// TODO
}

func (body *AbortRequestBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *AbortRequestBody) Decode(data []byte) error {
	// TODO
	return nil
}

type EndRequestBody struct {
	AppStatus      uint32
	ProtocolStatus ProtocolStatus
}

func (body *EndRequestBody) Encode() ([]byte, error) {
	var buf [8]byte

	binary.BigEndian.PutUint32(buf[0:], body.AppStatus)
	buf[4] = body.ProtocolStatus.Encode()

	return buf[:], nil
}

func (body *EndRequestBody) Decode(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("invalid body length")
	}

	body.AppStatus = binary.BigEndian.Uint32(data[0:])

	if err := body.ProtocolStatus.Decode(data[4]); err != nil {
		return err
	}

	return nil
}

type ParamsBody struct {
	// TODO
}

func (body *ParamsBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *ParamsBody) Decode(data []byte) error {
	// TODO
	return nil
}

type StdinBody struct {
	// TODO
}

func (body *StdinBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *StdinBody) Decode(data []byte) error {
	// TODO
	return nil
}

type StdoutBody struct {
	// TODO
}

func (body *StdoutBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *StdoutBody) Decode(data []byte) error {
	// TODO
	return nil
}

type StderrBody struct {
	// TODO
}

func (body *StderrBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *StderrBody) Decode(data []byte) error {
	// TODO
	return nil
}

type DataBody struct {
	// TODO
}

func (body *DataBody) Encode() ([]byte, error) {
	// TODO
	return nil, nil
}

func (body *DataBody) Decode(data []byte) error {
	// TODO
	return nil
}

type GetValuesBody struct {
	Names []string
}

func (body *GetValuesBody) Encode() ([]byte, error) {
	pairs := make(NameValuePairs, len(body.Names))

	for i, name := range body.Names {
		pairs[i].Name = name
	}

	return pairs.Encode()
}

func (body *GetValuesBody) Decode(data []byte) error {
	var pairs NameValuePairs
	if err := pairs.Decode(data); err != nil {
		return err
	}

	body.Names = make([]string, len(pairs))
	for i, pair := range pairs {
		body.Names[i] = pair.Name
	}

	return nil
}

type GetValuesResultBody struct {
	Pairs NameValuePairs
}

func (body *GetValuesResultBody) Encode() ([]byte, error) {
	return body.Pairs.Encode()
}

func (body *GetValuesResultBody) Decode(data []byte) error {
	return body.Pairs.Decode(data)
}

type UnknownTypeBody struct {
	Type uint8
}

func (body *UnknownTypeBody) Encode() ([]byte, error) {
	var buf [8]byte

	buf[0] = body.Type

	return buf[:], nil
}

func (body *UnknownTypeBody) Decode(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("invalid body length")
	}

	body.Type = data[0]

	return nil
}

type Record struct {
	Version    uint8
	RecordType RecordType
	RequestId  uint16
	Body       Body
}

type Body interface {
	Encode() ([]byte, error)
	Decode([]byte) error
}

func (r *Record) Write(w io.Writer) error {
	var body []byte
	var err error

	if r.Body != nil {
		body, err = r.Body.Encode()
		if err != nil {
			return fmt.Errorf("cannot encode body: %w", err)
		}

		if len(body) > math.MaxUint16 {
			return fmt.Errorf("body too large")
		}
	}

	buf := make([]byte, 8)

	buf[0] = uint8(r.Version)
	buf[1] = r.RecordType.Encode()
	binary.BigEndian.PutUint16(buf[2:], r.RequestId)
	binary.BigEndian.PutUint16(buf[4:], uint16(len(body)))
	buf[6] = 0 // padding length
	buf[7] = 0 // reserved

	buf = append(buf, body...)

	_, err = w.Write(buf)
	return err
}

func (r *Record) Read(reader io.Reader) error {
	// Header
	var header [8]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		err = netutils.UnwrapOpError(err, "read")
		return fmt.Errorf("cannot read header: %w", err)
	}

	r.Version = header[0]
	if r.Version != 1 {
		return fmt.Errorf("unhandled version %d", r.Version)
	}

	if err := r.RecordType.Decode(header[1]); err != nil {
		return err
	}

	r.RequestId = binary.BigEndian.Uint16(header[2:])

	contentLength := binary.BigEndian.Uint16(header[4:])
	paddingLength := header[6]

	// Body
	bodyLength := int(contentLength) + int(paddingLength)
	body := make([]byte, bodyLength)
	if _, err := io.ReadFull(reader, body[:]); err != nil {
		err = netutils.UnwrapOpError(err, "read")
		return fmt.Errorf("cannot read body: %w", err)
	}
	body = body[:contentLength]

	switch r.RecordType {
	case RecordTypeBeginRequest:
		r.Body = &BeginRequestBody{}
	case RecordTypeAbortRequest:
		r.Body = &AbortRequestBody{}
	case RecordTypeEndRequest:
		r.Body = &EndRequestBody{}
	case RecordTypeParams:
		r.Body = &ParamsBody{}
	case RecordTypeStdin:
		r.Body = &StdinBody{}
	case RecordTypeStdout:
		r.Body = &StdoutBody{}
	case RecordTypeStderr:
		r.Body = &StderrBody{}
	case RecordTypeData:
		r.Body = &DataBody{}
	case RecordTypeGetValues:
		r.Body = &GetValuesBody{}
	case RecordTypeGetValuesResult:
		r.Body = &GetValuesResultBody{}
	case RecordTypeUnknownType:
		r.Body = &UnknownTypeBody{}
	}

	if r.Body != nil {
		if err := r.Body.Decode(body); err != nil {
			return fmt.Errorf("cannot decode body: %w", err)
		}
	}

	return nil
}
