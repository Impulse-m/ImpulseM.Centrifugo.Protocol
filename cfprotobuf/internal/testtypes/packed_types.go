package testtypes

//go:generate go run ../../cmd/cfprotobuf -out packed_types_cfprotobuf.go packed_types.go

// Packed has repeated uint64 fields, which are written packed. One of them
// has a field number which takes a tag of two bytes.
type Packed struct {
	IDs    []uint64 `protobuf:"varint,1,rep,packed,name=ids,proto3"`
	Values []uint64 `protobuf:"varint,1000,rep,packed,name=values,proto3"`

	unknownFields []byte
}

// PackedOne is a Packed with a single field, which is decoded without a
// switch on the field number.
type PackedOne struct {
	IDs []uint64 `protobuf:"varint,2,rep,packed,name=ids,proto3"`
}
