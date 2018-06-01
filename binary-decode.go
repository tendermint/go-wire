package amino

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
)

//----------------------------------------
// cdc.decodeReflectBinary

// This is the main entrypoint for decoding all types from binary form.  This
// function calls decodeReflectBinary*, and generally those functions should
// only call this one, for the prefix bytes are consumed here when present.
// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinary(bz []byte, info *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if info.Type.Kind() == reflect.Interface && rv.Kind() == reflect.Ptr {
		panic("should not happen")
	}
	if printLog {
		spew.Printf("(D) decodeReflectBinary(bz: %X, info: %v, rv: %#v (%v), fopts: %v)\n",
			bz, info, rv.Interface(), rv.Type(), fopts)
		defer func() {
			fmt.Printf("(D) -> n: %v, err: %v\n", n, err)
		}()
	}
	var _n int

	// TODO consider the binary equivalent of json.Unmarshaller.

	// Dereference-and-construct pointers all the way.
	// This works for pointer-pointers.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			newPtr := reflect.New(rv.Type().Elem())
			rv.Set(newPtr)
		}
		rv = rv.Elem()
	}

	// Handle override if a pointer to rv implements UnmarshalAmino.
	if info.IsAminoUnmarshaler {
		// First, decode repr instance from bytes.
		rrv, rinfo := reflect.New(info.AminoUnmarshalReprType).Elem(), (*TypeInfo)(nil)
		rinfo, err = cdc.getTypeInfo_wlock(info.AminoUnmarshalReprType)
		if err != nil {
			return
		}
		_n, err = cdc.decodeReflectBinary(bz, rinfo, rrv, fopts)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		// Then, decode from repr instance.
		uwrm := rv.Addr().MethodByName("UnmarshalAmino")
		uwouts := uwrm.Call([]reflect.Value{rrv})
		erri := uwouts[0].Interface()
		if erri != nil {
			err = erri.(error)
		}
		return
	}

	switch info.Type.Kind() {

	//----------------------------------------
	// Complex

	case reflect.Interface:
		_n, err = cdc.decodeReflectBinaryInterface(bz, info, rv, fopts)
		n += _n
		return

	case reflect.Array:
		ert := info.Type.Elem()
		if ert.Kind() == reflect.Uint8 {
			_n, err = cdc.decodeReflectBinaryByteArray(bz, info, rv, fopts)
			n += _n
		} else {
			_n, err = cdc.decodeReflectBinaryArray(bz, info, rv, fopts)
			n += _n
		}
		return

	case reflect.Slice:
		ert := info.Type.Elem()
		if ert.Kind() == reflect.Uint8 {
			_n, err = cdc.decodeReflectBinaryByteSlice(bz, info, rv, fopts)
			n += _n
		} else {
			_n, err = cdc.decodeReflectBinarySlice(bz, info, rv, fopts)
			n += _n
		}
		return

	case reflect.Struct:
		_n, err = cdc.decodeReflectBinaryStruct(bz, info, rv, fopts)
		n += _n
		return

	//----------------------------------------
	// Signed

	case reflect.Int64:
		var num int64
		if fopts.BinVarint {
			num, _n, err = DecodeVarint(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			rv.SetInt(num)
		} else {
			num, _n, err = DecodeInt64(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			rv.SetInt(num)
		}
		return

	case reflect.Int32:
		var num int32
		num, _n, err = DecodeInt32(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetInt(int64(num))
		return

	case reflect.Int16:
		var num int16
		num, _n, err = DecodeInt16(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetInt(int64(num))
		return

	case reflect.Int8:
		var num int8
		num, _n, err = DecodeInt8(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetInt(int64(num))
		return

	case reflect.Int:
		var num int64
		num, _n, err = DecodeVarint(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetInt(num)
		return

	//----------------------------------------
	// Unsigned

	case reflect.Uint64:
		var num uint64
		if fopts.BinVarint {
			num, _n, err = DecodeUvarint(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			rv.SetUint(num)
		} else {
			num, _n, err = DecodeUint64(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			rv.SetUint(num)
		}
		return

	case reflect.Uint32:
		var num uint32
		num, _n, err = DecodeUint32(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetUint(uint64(num))
		return

	case reflect.Uint16:
		var num uint16
		num, _n, err = DecodeUint16(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetUint(uint64(num))
		return

	case reflect.Uint8:
		var num uint8
		num, _n, err = DecodeUint8(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetUint(uint64(num))
		return

	case reflect.Uint:
		var num uint64
		num, _n, err = DecodeUvarint(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetUint(num)
		return

	//----------------------------------------
	// Misc.

	case reflect.Bool:
		var b bool
		b, _n, err = DecodeBool(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetBool(b)
		return

	case reflect.Float64:
		var f float64
		if !fopts.Unsafe {
			err = errors.New("Float support requires `amino:\"unsafe\"`.")
			return
		}
		f, _n, err = DecodeFloat64(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetFloat(f)
		return

	case reflect.Float32:
		var f float32
		if !fopts.Unsafe {
			err = errors.New("Float support requires `amino:\"unsafe\"`.")
			return
		}
		f, _n, err = DecodeFloat32(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetFloat(float64(f))
		return

	case reflect.String:
		var str string
		str, _n, err = DecodeString(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.SetString(str)
		return

	default:
		panic(fmt.Sprintf("unknown field type %v", info.Type.Kind()))
	}

}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinaryInterface(bz []byte, iinfo *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectBinaryInterface")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	if !rv.IsNil() {
		// JAE: Heed this note, this is very tricky.
		// I've forgotten the reason a second time,
		// but I'm pretty sure that reason exists.
		err = errors.New("Decoding to a non-nil interface is not supported yet")
		return
	}

	// Read byte-length prefixed byteslice.
	var buf, _n = []byte(nil), int(0)
	buf, _n, err = DecodeByteSlice(bz)
	if slide(&bz, &n, _n) && err != nil {
		return
	}
	bz = buf.Bytes()

	// Consume disambiguation / prefix bytes.
	disamb, hasDisamb, prefix, hasPrefix, _n, err := DecodeDisambPrefixBytes(bz)
	if slide(&bz, &n, _n) && err != nil {
		return
	}

	// Get concrete type info from disfix/prefix.
	var cinfo *TypeInfo
	if hasDisamb {
		cinfo, err = cdc.getTypeInfoFromDisfix_rlock(toDisfix(disamb, prefix))
	} else if hasPrefix {
		cinfo, err = cdc.getTypeInfoFromPrefix_rlock(iinfo, prefix)
	} else {
		err = errors.New("Expected disambiguation or prefix bytes.")
	}
	if err != nil {
		return
	}

	// Construct the concrete type.
	var crv, irvSet = constructConcreteType(cinfo)

	// Decode into the concrete type.
	_n, err = cdc.decodeReflectBinary(bz, cinfo, crv, fopts)
	if slide(&bz, &n, _n) && err != nil {
		rv.Set(irvSet) // Helps with debugging
		return
	}

	// Earlier, we set bz to the byteslice read from buf.
	// Ensure that all of bz was consumed.
	if len(bz) > 0 {
		err = errors.New("bytes left over after reading interface contents")
		return
	}

	// We need to set here, for when !PointerPreferred and the type
	// is say, an array of bytes (e.g. [32]byte), then we must call
	// rv.Set() *after* the value was acquired.
	// NOTE: rv.Set() should succeed because it was validated
	// already during Register[Interface/Concrete].
	rv.Set(irvSet)
	return
}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinaryByteArray(bz []byte, info *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectBinaryByteArray")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	ert := info.Type.Elem()
	if ert.Kind() != reflect.Uint8 {
		panic("should not happen")
	}
	length := info.Type.Len()
	if len(bz) < length {
		return 0, fmt.Errorf("Insufficient bytes to decode [%v]byte.", length)
	}

	// Read byte-length prefixed byteslice.
	var byteslice, _n = []byte(nil), int(0)
	byteslice, _n, err = DecodeByteSlice(bz)
	if slide(&bz, &n, _n) && err != nil {
		return
	}
	if len(byteslice) != length {
		err = fmt.Errorf("Mismatched byte array length: Expected %v, got %v",
			length, len(byteslice))
		return
	}

	// Copy read byteslice to rv array.
	reflect.Copy(rv, reflect.ValueOf(byteslice))
	return
}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinaryArray(bz []byte, info *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectBinaryArray")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	ert := info.Type.Elem()
	if ert.Kind() == reflect.Uint8 {
		panic("should not happen")
	}
	length := info.Type.Len()
	einfo, err := cdc.getTypeInfo_wlock(ert)
	if err != nil {
		return
	}

	// If elem is not already a ByteLength type, read in packed form.
	// This is a Proto wart due to Proto backwards compatibility issues.
	// Amino2 will probably migrate to use the List typ3.
	typ3 := typeToTyp3(einfo.Type, fopts)
	if typ3 != Typ3_ByteLength {
		// Read elements in packed form.
		// Read byte-length prefixed byteslice.
		var buf, _n = []byte(nil), int(0)
		buf, _n, err = DecodeByteSlice(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		// Read elements from buf.
		for i := 0; i < length; i++ {
			var erv, _n = rv.Index(i), int(0)
			_n, err = cdc.decodeReflectBinary(buf, einfo, erv, fopts)
			if slide(&buf, nil, _n) && err != nil {
				err = fmt.Errorf("error reading array contents: %v", err)
				return
			}
		}
		// Ensure that we read the whole buffer.
		if len(buf) > 0 {
			err = errors.New("bytes left over after reading array contents")
			return
		}
	} else {
		// Read elements in unpacked form.
		for i := 0; i < length; i++ {
			// Read field key (number and type).
			var fieldNum, typ = uint32(0), Typ3(0x00)
			fieldNum, typ, _n, err = decodeFieldNumberAndTyp3(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			// Validate field number and typ3.
			if fieldNum != fopts.BinFieldNum {
				err = errors.New(fmt.Sprintf("expected repeated field number %v, got %v", field.BinFieldNum, fieldNum))
				return
			}
			if typ != Typ3_ByteLength {
				err = errors.New(fmt.Sprintf("expected repeated field type %v, got %v", Typ3_ByteLength, typ))
				return
			}
			// Decode the next ByteLength bytes into erv.
			var erv, _n = rv.Index(i), int(0)
			// Special case if next ByteLength bytes are 0x00, set nil.
			if len(bz) > 0 && bz[0] == 0x00 {
				slide(&bz, &n, 1)
				erv.Set(reflect.Zero(erv.Type()))
				continue
			}
			// Normal case, read next non-nil element from bz.
			_n, err = cdc.decodeReflectBinary(bz, einfo, erv, fopts)
			if slide(&bz, &n, _n) && err != nil {
				err = fmt.Errorf("error reading array contents: %v", err)
				return
			}
		}
		// Ensure that there are no more elements left,
		// and no field number regression either.
		// This is to provide better error messages.
		if len(bz) > 0 {
			var fieldNum, typ = uint32(0), Typ3(0x00)
			fieldNum, _, _, err = decodeFieldNumberAndTyp3(bz)
			if err != nil {
				return
			}
			if fieldNum <= fopts.BinFieldNum {
				err = fmt.Errorf("unexpected field number %v after repeated field number %v", fieldNum, fopts.BinFieldNum)
				return
			}
		}
	}
	return
}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinaryByteSlice(bz []byte, info *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectByteSlice")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	ert := info.Type.Elem()
	if ert.Kind() != reflect.Uint8 {
		panic("should not happen")
	}

	// Read byte-length prefixed byteslice.
	var byteslice, _n = []byte(nil), int(0)
	byteslice, _n, err = DecodeByteSlice(bz)
	if slide(&bz, &n, _n) && err != nil {
		return
	}
	if len(byteslice) == 0 {
		// Special case when length is 0.
		// NOTE: We prefer nil slices.
		rv.Set(info.ZeroValue)
	} else {
		rv.Set(reflect.ValueOf(byteslice))
	}
	return
}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinarySlice(bz []byte, info *TypeInfo, rv reflect.Value, fopts FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectBinarySlice")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	ert := info.Type.Elem()
	if ert.Kind() == reflect.Uint8 {
		panic("should not happen")
	}
	einfo, err := cdc.getTypeInfo_wlock(ert)
	if err != nil {
		return
	}

	// Construct slice to collect decoded items to.
	// NOTE: This is due to Proto3.  How to best optimize?
	esrt := reflect.SliceOf(ert)
	var srv = reflect.MakeSlice(esrt, 1, 1)

	// If elem is not already a ByteLength type, read in packed form.
	// This is a Proto wart due to Proto backwards compatibility issues.
	// Amino2 will probably migrate to use the List typ3.
	typ3 := typeToTyp3(einfo.Type, fopts)
	if typ3 != Typ3_ByteLength {
		// Read elems in packed form.
		// Read byte-length prefixed byteslice.
		var buf, _n = []byte(nil), int(0)
		buf, _n, err = DecodeByteSlice(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		// Read elements from buf.
		for {
			erv := reflect.New(einfo.Type)
			_n, err = cdc.decodeReflectBinary(buf, einfo, erv, fopts)
			if slide(&buf, nil, _n) && err != nil {
				err = fmt.Errorf("error reading array contents: %v", err)
				return
			}
			srv = reflect.Append(srv, erv)
		}
	} else {
		// Read elements in unpacked form.
		for {
			// Read field key (number and type).
			var fieldNum, typ = uint32(0), Typ3(0x00)
			fieldNum, typ, _n, err = decodeFieldNumberAndTyp3(bz)
			if slide(&bz, &n, _n) && err != nil {
				return
			}
			// Validate field number and typ3.
			if fieldNum < fopts.BinFieldNum {
				err = errors.New(fmt.Sprintf("expected repeated field number %v or greater, got %v", field.BinFieldNum, fieldNum))
				return
			}
			if typ != Typ3_ByteLength {
				err = errors.New(fmt.Sprintf("expected repeated field type %v, got %v", Typ3_ByteLength, typ))
				return
			}
			// Decode the next ByteLength bytes into erv.
			erv, _n := reflect.New(einfo.Type), int(0)
			// Special case if next ByteLength bytes are 0x00, set nil.
			if len(bz) > 0 && bz[0] == 0x00 {
				slide(&bz, &n, 1)
				erv.Set(reflect.Zero(erv.Type()))
				continue
			}
			// Normal case, read next non-nil element from bz.
			_n, err = cdc.decodeReflectBinary(bz, einfo, erv, fopts)
			if slide(&bz, &n, _n) && err != nil {
				err = fmt.Errorf("error reading array contents: %v", err)
				return
			}
			srv = reflect.Append(srv, erv)
		}
	}
	rv.Set(srv)
	return
}

// CONTRACT: rv.CanAddr() is true.
func (cdc *Codec) decodeReflectBinaryStruct(bz []byte, info *TypeInfo, rv reflect.Value, _ FieldOptions) (n int, err error) {
	if !rv.CanAddr() {
		panic("rv not addressable")
	}
	if printLog {
		fmt.Println("(d) decodeReflectBinaryStruct")
		defer func() {
			fmt.Printf("(d) -> err: %v\n", err)
		}()
	}
	_n := 0 // nolint: ineffassign

	// Read byte-length prefixed byteslice.
	var buf, _n = []byte(nil), int(0)
	buf, _n, err = DecodeByteSlice(bz)
	if slide(&bz, &n, _n) && err != nil {
		return
	}
	bz = buf.Bytes()

	switch info.Type {

	case timeType:
		// Special case: time.Time
		var t time.Time
		t, _n, err = DecodeTime(bz)
		if slide(&bz, &n, _n) && err != nil {
			return
		}
		rv.Set(reflect.ValueOf(t))

	default:
		// Read each field.
		for _, field := range info.Fields {

			// Get field rv and info.
			var frv = rv.Field(field.Index)
			var finfo *TypeInfo
			finfo, err = cdc.getTypeInfo_wlock(field.Type)
			if err != nil {
				return
			}

			// We're done if we've consumed all the bytes.
			if len(bz) == 0 {
				frv.Set(reflect.Zero(frv.Type()))
				continue
			}

			if field.UnpackedList {
				// This is a list that was encoded unpacked, e.g.
				// with repeated field entries for each list item.
				_n, err = cdc.decodeReflectBinaryList(bz, finfo, frv, field.FieldOptions)
				if slide(&bz, &n, _n) && err != nil {
					return
				}
			} else {
				// Read field key (number and type).
				var fieldNum, typ = uint32(0), Typ3(0x00)
				fieldNum, typ, _n, err = decodeFieldNumberAndTyp3(bz)
				if field.BinFieldNum < fieldNum {
					// Set zero field value.
					frv.Set(reflect.Zero(frv.Type()))
					continue
					// Do not slide, we will read it again.
				}
				if slide(&bz, &n, _n) && err != nil {
					return
				}

				// Validate fieldNum and typ.
				// NOTE: In the future, we'll support upgradeability.
				// So in the future, this may not match,
				// so we will need to remove this sanity check.
				if field.BinFieldNum != fieldNum {
					err = errors.New(fmt.Sprintf("expected field number %v, got %v", field.BinFieldNum, fieldNum))
					return
				}
				typWanted := typeToTyp3(field.Type, field.FieldOptions)
				if typ != typWanted {
					err = errors.New(fmt.Sprintf("expected field type %v, got %v", typWanted, typ))
					return
				}

				// Decode field into frv.
				_n, err = cdc.decodeReflectBinary(bz, finfo, frv, field.FieldOptions)
				if slide(&bz, &n, _n) && err != nil {
					return
				}
			}
		}
	}

	// Earlier, we set bz to the byteslice read from buf.
	// Ensure that all of bz was consumed.
	// XXX: After merge with latest develop, I think this check goes away.
	if len(bz) > 0 {
		err = errors.New("bytes left over after reading struct contents")
		return
	}
	return
}

//----------------------------------------

func DecodeDisambPrefixBytes(bz []byte) (db DisambBytes, hasDb bool, pb PrefixBytes, hasPb bool, n int, err error) {
	// Validate
	if len(bz) < 4 {
		err = errors.New("EOF while reading prefix bytes.")
		return // hasPb = false
	}
	if bz[0] == 0x00 { // Disfix
		// Validate
		if len(bz) < 8 {
			err = errors.New("EOF while reading disamb bytes.")
			return // hasPb = false
		}
		copy(db[0:3], bz[1:4])
		copy(pb[0:4], bz[4:8])
		hasDb = true
		hasPb = true
		n = 8
		return
	} else { // Prefix
		// General case with no disambiguation
		copy(pb[0:4], bz[0:4])
		hasDb = false
		hasPb = true
		n = 4
		return
	}
}

// Read field key.
func decodeFieldNumberAndTyp3(bz []byte) (num uint32, typ Typ3, n int, err error) {

	// Read uvarint value.
	var value64 = uint64(0)
	value64, n, err = DecodeUvarint(bz)
	if err != nil {
		return
	}

	// Decode first typ3 byte.
	typ = Typ3(value64 & 0x07)

	// Decode num.
	var num64 uint64
	num64 = value64 >> 3
	if num64 > (1<<29 - 1) {
		err = errors.New(fmt.Sprintf("invalid field num %v", num64))
		return
	}
	num = uint32(num64)
	return
}

// Error if typ doesn't match rt.
func checkTyp3(rt reflect.Type, typ Typ3, fopts FieldOptions) (err error) {
	typWanted := typeToTyp3(rt, fopts)
	if typ != typWanted {
		err = fmt.Errorf("Typ3 mismatch. Expected %v, got %v", typWanted, typ)
	}
	return
}

// Read typ3 byte.
func decodeTyp3(bz []byte) (typ Typ3, n int, err error) {
	if len(bz) == 0 {
		err = fmt.Errorf("EOF while reading typ3 byte")
		return
	}
	if bz[0]&0xF8 != 0 {
		err = fmt.Errorf("Invalid typ3 byte")
		return
	}
	typ = Typ3(bz[0])
	n = 1
	return
}

// Read a uvarint that encodes the number of nil items to skip.  NOTE:
// Currently does not support any number besides 0 (not nil) and 1 (nil).  All
// other values will error.
func decodeNumNilBytes(bz []byte) (numNil int64, n int, err error) {
	if len(bz) == 0 {
		err = errors.New("EOF while reading nil byte(s)")
		return
	}
	if bz[0] == 0x00 {
		numNil, n = 0, 1
		return
	}
	if bz[0] == 0x01 {
		numNil, n = 1, 1
		return
	}
	n, err = 0, fmt.Errorf("unexpected nil byte, want: either '0x00' or '0x01' got: %X (sparse lists not supported)", bz[0])
	return
}
