package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/centrifugal/protocol/cfjson"
)

// The fields with numbers 1000-1999 are written out by hand here, from the
// protocol buffer encoding rules and from the JSON names of the protocol.
// They are checked against google.golang.org/protobuf, which encodes and
// decodes the same messages.

// forkProtoGolden are the Protobuf encodings of messages with fields from
// the fork range. A field with a number from 1000 to 2047 has a tag of two
// bytes: 1000<<3 is 8000, which is c0 3e for varint fields and c2 3e for
// length delimited ones.
var forkProtoGolden = []struct {
	name string
	msg  message
	want string
}{
	{"error app code", &Error{AppCode: 1}, "c03e01"},
	{"reply time", &Reply{Time: 1}, "c03e01"},
	{"connect request instance", &ConnectRequest{Instance: "i"}, "c23e0169"},
	// msg_id is 1001<<3 = 8008, c8 3e. 2^53-1 takes eight bytes.
	{"rpc max msg id", &RPCRequest{MsgId: 1<<53 - 1}, "c83e" + "ffffffffffffff" + "0f"},
	// ack is 1002<<3|2 = 8018, d2 3e, and is packed.
	{"rpc packed ack", &RPCRequest{Ack: []uint64{1}}, "d23e0101"},
	{"rpc packed ack several", &RPCRequest{Ack: []uint64{1, 300}}, "d23e03" + "01ac02"},
	{"rpc packed ack max", &RPCRequest{Ack: []uint64{1<<64 - 1}}, "d23e0a" + "ffffffffffffffffff" + "01"},
	// cancel is 1006<<3 = 8048, f0 3e.
	{"rpc cancel", &RPCRequest{Cancel: 5}, "f03e05"},
}

// Inputs with the same messages in the unpacked form of repeated uint64,
// which google.golang.org/protobuf reads too.
var forkProtoUnpacked = []struct {
	name string
	data string
	typ  string
	want []uint64
}{
	{"ack unpacked", "d03e01d03e02", "RPCRequest", []uint64{1, 2}},
	{"ack packed then unpacked", "d23e0101" + "d03e03", "RPCRequest", []uint64{1, 3}},
}

func TestForkFieldsProtobufGolden(t *testing.T) {
	for _, tt := range forkProtoGolden {
		t.Run(tt.name, func(t *testing.T) {
			want, err := hex.DecodeString(tt.want)
			require.NoError(t, err)

			got, err := tt.msg.MarshalCF()
			require.NoError(t, err)
			require.Equal(t, want, got)
			require.Equal(t, len(want), tt.msg.SizeCF())

			official, err := proto.MarshalOptions{Deterministic: true}.Marshal(tt.msg)
			require.NoError(t, err)
			require.Equal(t, want, official)
		})
	}
}

// A field of the fork range costs two bytes of tag, as the numbers below
// 2048 do. A third byte would be a number of 2048 or more.
func TestForkFieldsTagSize(t *testing.T) {
	m := &Error{AppCode: 1}
	data, err := m.MarshalCF()
	require.NoError(t, err)
	require.Len(t, data, 3)
	require.Equal(t, []byte{0xc0, 0x3e, 0x01}, data)
	require.Equal(t, 3, m.SizeCF())
}

func TestForkFieldsProtobufUnpackedInput(t *testing.T) {
	for _, tt := range forkProtoUnpacked {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.data)
			require.NoError(t, err)
			compareWithOfficial(t, tt.typ, data)

			got := new(RPCRequest)
			require.NoError(t, got.UnmarshalCF(data))
			require.Equal(t, tt.want, got.Ack)
		})
	}
}

// forkJSONGolden are the JSON encodings of the fork fields, with values
// above 2^32, which a 32-bit integer would not hold, and up to 2^53-1, which
// is the largest integer a JavaScript number holds exactly.
var forkJSONGolden = []struct {
	name string
	msg  message
	want string
}{
	{
		"error",
		&Error{AppCode: 420, AppType: "retry_later", Metadata: map[string]string{"pending_token": "abc"}},
		`{"app_code":420,"app_type":"retry_later","metadata":{"pending_token":"abc"}}`,
	},
	{"reply time", &Reply{Time: 1760000000123}, `{"time":1760000000123}`},
	{
		"rpc request",
		&RPCRequest{
			Method:         "rpc.call",
			Headers:        map[string]string{"x-trace": "t1"},
			MsgId:          1<<32 + 1,
			Ack:            []uint64{1 << 32, 1<<53 - 1},
			Retransmit:     true,
			After:          []uint64{7},
			WithoutUpdates: true,
		},
		`{"method":"rpc.call","headers":{"x-trace":"t1"},"msg_id":4294967297,"ack":[4294967296,9007199254740991],"retransmit":true,"after":[7],"without_updates":true}`,
	},
	{"rpc cancel", &RPCRequest{MsgId: 2, Cancel: 1}, `{"msg_id":2,"cancel":1}`},
	{"rpc result", &RPCResult{Cached: true}, `{"cached":true}`},
	{"connect request", &ConnectRequest{Instance: "install-1"}, `{"instance":"install-1"}`},
	{"connect result", &ConnectResult{User: "user-42", AuthClass: "anonymous"}, `{"user":"user-42","auth_class":"anonymous"}`},
	{"refresh result", &RefreshResult{User: "user-42", AuthClass: "anonymous"}, `{"user":"user-42","auth_class":"anonymous"}`},
}

func TestForkFieldsJSONGolden(t *testing.T) {
	for _, tt := range forkJSONGolden {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, string(tt.msg.AppendJSON(nil)))
			require.Equal(t, tt.want, string(encodeJSON(tt.msg)))

			// encoding/json shares nothing with the code under test, the
			// decoders must make the same message of the golden as it does.
			data := []byte(tt.want)
			fresh := reflect.New(reflect.TypeOf(tt.msg).Elem()).Interface().(message)
			require.NoError(t, json.Unmarshal(data, fresh))
			require.Equal(t, tt.want, string(fresh.AppendJSON(nil)))
			for _, flags := range []cfjson.Flags{0, cfjson.Prescan(data, 0)} {
				decoded := reflect.New(reflect.TypeOf(tt.msg).Elem()).Interface().(message)
				n := decoded.DecodeJSON(data, 0, flags)
				require.Equal(t, len(data), n)
				require.True(t, reflect.DeepEqual(tt.msg, decoded), tt.want)
				require.True(t, bytes.Equal(data, []byte(tt.want)))
			}
		})
	}
}
