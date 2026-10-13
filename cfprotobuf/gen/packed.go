package gen

import "fmt"

// Packed repeated uint64 fields. proto3 writes them as one length-delimited
// field holding the varints of all values, and reads both that and the
// unpacked form, one field per value. Nothing else of packed encoding is
// supported: other numeric types are rejected by structInfo.

// isPackedValue reports whether repeated elements of a value are written
// packed: uint64 is the only numeric type supported for it.
func isPackedValue(v value) bool {
	return v.kind == kVarint && v.src == "uint64"
}

// packed reports whether a field is written packed, see packedField.
func (f field) packed() bool {
	return f.repeated && !f.isMap && isPackedValue(f.val)
}

// packedField checks the packed option of a field, which is only supported
// on repeated uint64, and the types of repeated fields, which are messages,
// strings and repeated uint64. proto3 writes repeated numbers packed, so a
// repeated uint64 without the option is rejected too. Map fields with the
// option are rejected by structInfo.
func packedField(where string, fd field, packedOpt bool) error {
	if packedOpt && (!fd.repeated || !isPackedValue(fd.val)) {
		return fmt.Errorf("%s: protobuf tag option %q is not supported", where, "packed")
	}
	if fd.repeated && fd.val.kind != kMessage && fd.val.kind != kString && !isPackedValue(fd.val) {
		return fmt.Errorf("%s: repeated fields of type %s are not supported", where, fd.val.src)
	}
	if fd.packed() && !packedOpt {
		return fmt.Errorf("%s: repeated uint64 fields must be packed", where)
	}
	return nil
}

// sizePacked emits code adding the size of the packed field f to n.
func (g *generator) sizePacked(f field) {
	x := "m." + f.goName
	g.p("if len(%s) > 0 {", x)
	g.p("packed := 0")
	g.p("for _, e := range %s {", x)
	g.p("packed += cfprotobuf.SizeOfInt(e)")
	g.p("}")
	g.p("n += %d + cfprotobuf.SizeOfLen(packed) + packed", len(tagBytes(tagOf(f.number, 2))))
	g.p("}")
}

// marshalPacked emits code writing the packed field f in front of what has
// been written so far, as the other fields are.
func (g *generator) marshalPacked(f field) {
	x := "m." + f.goName
	g.p("if len(%s) > 0 {", x)
	g.p("end := i")
	g.p("for n := len(%s) - 1; n >= 0; n-- {", x)
	g.p("i = cfprotobuf.PutInt(b, i, %s[n])", x)
	g.p("}")
	g.p("i = cfprotobuf.PutLen(b, i, end-i)")
	g.putTag(tagOf(f.number, 2))
	g.p("}")
}

// unmarshalPacked emits the cases of the decoder for the packed field f: one
// for the packed wire type and one for the unpacked one. single is set if
// the decoder has no switch, see unmarshal.
func (g *generator) unmarshalPacked(f field, single bool) {
	x := "m." + f.goName
	if single {
		g.p("if tag == %d { // %s", tagOf(f.number, 2), f.goName)
	} else {
		g.p("case %d: // %s", f.number, f.goName)
		g.p("if tag&7 == 2 {")
	}
	// The payload is a length followed by varints, each of which is decoded
	// out of the payload.
	g.getLength("b", "i", "")
	g.p("entry := b[from:to]")
	g.p("var nx int")
	g.p("for j := 0; j < len(entry); {")
	g.p("var e uint64")
	g.getValue(value{kind: kVarint, src: "uint64"}, "e", "entry", "j", "nx", "", "", true)
	g.p("%s = append(%s, e)", x, x)
	g.p("}")
	g.p("i = to")
	g.p("continue")
	g.p("}")
	if single {
		g.p("if tag == %d {", tagOf(f.number, 0))
	} else {
		g.p("if tag&7 == 0 {")
	}
	g.p("var e uint64")
	g.getValue(value{kind: kVarint, src: "uint64"}, "e", "b", "i", "next", "", "", true)
	g.p("%s = append(%s, e)", x, x)
	g.p("continue")
	g.p("}")
}
