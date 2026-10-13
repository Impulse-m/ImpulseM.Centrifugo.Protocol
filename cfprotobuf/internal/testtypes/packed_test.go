package testtypes

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/centrifugal/protocol/cfprotobuf"
)

// refVarints returns the varints of values one after another, which is what
// the payload of a packed field holds.
func refVarints(values []uint64) []byte {
	var b []byte
	for _, v := range values {
		b = refVarint(b, v)
	}
	return b
}

// refPacked is the reference encoder of Packed, written the obvious way.
func refPacked(m *Packed) []byte {
	var b []byte
	if len(m.IDs) > 0 {
		b = refBytes(b, 1, refVarints(m.IDs))
	}
	if len(m.Values) > 0 {
		b = refBytes(b, 1000, refVarints(m.Values))
	}
	return append(b, m.unknownFields...)
}

func refPackedOne(m *PackedOne) []byte {
	var b []byte
	if len(m.IDs) > 0 {
		b = refBytes(b, 2, refVarints(m.IDs))
	}
	return b
}

// Golden encodings, written out by hand from the protocol buffer encoding
// rules: a tag, a length and the varints of the values. The official
// google.golang.org/protobuf encoder produces the same, see
// fork_fields_test.go in the root of the module, which checks the same bytes
// for repeated uint64 fields of RPCRequest.
var packedGolden = []struct {
	name string
	msg  *Packed
	want string
}{
	{"empty", &Packed{}, ""},
	{"one element", &Packed{IDs: []uint64{1}}, "0a0101"},
	{"several elements", &Packed{IDs: []uint64{1, 300}}, "0a0301ac02"},
	{"max varint", &Packed{IDs: []uint64{math.MaxUint64}}, "0a0a" + strings.Repeat("ff", 9) + "01"},
	{"zero is written", &Packed{IDs: []uint64{0, 0}}, "0a020000"},
	// Field 1000 has a tag of two bytes: 1000<<3|2 is 8002, which is c2 3e.
	{"field with a two byte tag", &Packed{Values: []uint64{1}}, "c23e0101"},
	{"both fields", &Packed{IDs: []uint64{7}, Values: []uint64{1 << 32}}, "0a0107c23e0580808080" + "10"},
}

func TestPackedGolden(t *testing.T) {
	for _, tt := range packedGolden {
		t.Run(tt.name, func(t *testing.T) {
			want, err := hex.DecodeString(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tt.msg.MarshalCF()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("MarshalCF = %x, want %x", got, want)
			}
			if !bytes.Equal(refPacked(tt.msg), want) {
				t.Fatalf("the reference encoder gives %x, the golden data says %x", refPacked(tt.msg), want)
			}
			if size := tt.msg.SizeCF(); size != len(want) {
				t.Fatalf("SizeCF = %d, want %d", size, len(want))
			}
			decoded := new(Packed)
			if err := decoded.UnmarshalCF(want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, tt.msg) {
				t.Fatalf("decoded %+v, want %+v", decoded, tt.msg)
			}
		})
	}
}

// Packed values are read from both the packed and the unpacked form, and a
// message may have both, one after another.
func TestPackedUnpackedInput(t *testing.T) {
	tests := []struct {
		name string
		data string
		want *Packed
	}{
		{"unpacked", "0801" + "0802", &Packed{IDs: []uint64{1, 2}}},
		{"packed then unpacked", "0a0101" + "0803", &Packed{IDs: []uint64{1, 3}}},
		{"unpacked then packed", "0805" + "0a010a", &Packed{IDs: []uint64{5, 10}}},
		{"unpacked field with a two byte tag", "c03e05", &Packed{Values: []uint64{5}}},
		{"empty packed payload", "0a00", &Packed{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			got := new(Packed)
			if err := got.UnmarshalCF(data); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decoded %+v, want %+v", got, tt.want)
			}
		})
	}

	// A message with a single field decodes both forms too.
	for data, want := range map[string][]uint64{
		"1001" + "1002": {1, 2},
		"1201" + "01":   {1},
		"12020102":      {1, 2},
	} {
		raw, err := hex.DecodeString(data)
		if err != nil {
			t.Fatal(err)
		}
		got := new(PackedOne)
		if err := got.UnmarshalCF(raw); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.IDs, want) {
			t.Fatalf("%s decoded to %v, want %v", data, got.IDs, want)
		}
	}
}

func TestPackedErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want error
	}{
		{"payload past the end", "0a05" + "01", cfprotobuf.ErrTruncated},
		{"varint cut off in the payload", "0a02" + "8080", cfprotobuf.ErrTruncated},
		{"varint of eleven bytes in the payload", "0a0b" + strings.Repeat("80", 10) + "01", cfprotobuf.ErrIntOverflow},
		{"unpacked varint cut off", "08", cfprotobuf.ErrTruncated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if err := new(Packed).UnmarshalCF(data); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

// randomUints returns nil or a few values, which include zero, the limits of
// the varint encoding and random ones.
func randomUints(r *rand.Rand) []uint64 {
	n := r.Intn(4)
	if n == 0 {
		return nil
	}
	values := make([]uint64, n)
	for i := range values {
		values[i] = []uint64{0, 1, 127, 128, 1<<32 - 1, 1<<32 + 1, 1<<53 - 1, math.MaxUint64, r.Uint64()}[r.Intn(9)]
	}
	return values
}

func randomPacked(r *rand.Rand) *Packed {
	return &Packed{IDs: randomUints(r), Values: randomUints(r)}
}

func TestPackedRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	for n := 0; n < 20000; n++ {
		m := randomPacked(r)
		want := refPacked(m)
		got, err := m.MarshalCF()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("MarshalCF of %+v:\n got %x\nwant %x", m, got, want)
		}
		if size := m.SizeCF(); size != len(want) {
			t.Fatalf("SizeCF = %d, the encoding has %d bytes", size, len(want))
		}
		decoded := new(Packed)
		if err = decoded.UnmarshalCF(got); err != nil {
			t.Fatalf("%v: %x", err, got)
		}
		if !reflect.DeepEqual(decoded, m) {
			t.Fatalf("round trip changed the message:\n got %+v\nwant %+v", decoded, m)
		}

		one := &PackedOne{IDs: randomUints(r)}
		data, err := one.MarshalCF()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, refPackedOne(one)) || one.SizeCF() != len(data) {
			t.Fatalf("PackedOne of %+v encodes as %x", one, data)
		}
		decodedOne := new(PackedOne)
		if err := decodedOne.UnmarshalCF(data); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decodedOne, one) {
			t.Fatalf("round trip changed the message: got %+v, want %+v", decodedOne, one)
		}
	}
}

// Decoding of arbitrary bytes into Packed must not panic, and whatever it
// accepts must be encoded the way the reference encoder does it.
func FuzzUnmarshalPacked(f *testing.F) {
	for _, tt := range packedGolden {
		data, _ := hex.DecodeString(tt.want)
		f.Add(data)
	}
	f.Add([]byte{0x0a, 0x02, 0x80, 0x80})
	f.Add([]byte{0x08, 0x01, 0x0a, 0x01, 0x01})
	f.Fuzz(func(t *testing.T, data []byte) {
		input := bytes.Clone(data)
		m := new(Packed)
		if err := m.UnmarshalCF(data); err != nil {
			return
		}
		if !bytes.Equal(input, data) {
			t.Fatal("decoding modified the input")
		}
		encoded, err := m.MarshalCF()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, refPacked(m)) {
			t.Fatalf("MarshalCF of %+v is %x, the reference gives %x", m, encoded, refPacked(m))
		}
		if len(encoded) != m.SizeCF() {
			t.Fatalf("SizeCF = %d, the encoding has %d bytes", m.SizeCF(), len(encoded))
		}
		again := new(Packed)
		if err := again.UnmarshalCF(encoded); err != nil {
			t.Fatalf("own output does not decode: %v", err)
		}
		if !reflect.DeepEqual(again, m) {
			t.Fatalf("round trip of %x changed the message", input)
		}
	})
}
