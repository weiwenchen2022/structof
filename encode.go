package structof

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/weiwenchen2022/structtag"
)

var mapType = reflect.TypeOf(map[string]any(nil))

// FillMap fills the given struct into the map[string]any.
// FillMap panics if i's kind not struct or pointer to struct,
// or v not non-nil pointer to map[string]any.
//
// FillMap first establishes a map to use. If the map is nil,
// FillMap allocates a new map. Otherwise FillMap
// reuses the existing map, keeping existing entries.
//
// Each exported struct field becomes a element of the map, using the
// field name as the map key, unless the field is omitted for one of the
// reasons given below.
//
// The encoding of each struct field can be customized by the format string
// stored under the "structof" key in the struct field's tag.
// The format string gives the name of the field, possibly followed by a
// comma-separated list of options. The name may be empty in order to
// specify options without overriding the default field name.
//
// The "omitempty" option specifies that the field should be omitted
// from the encoding if the field has an empty value, defined as
// false, 0, a nil pointer, a nil interface value, and any empty array,
// slice, map, or string.
//
// As a special case, if the field tag is "-", the field is always omitted.
// Note that a field with name "-" can still be generated using the tag "-,".
//
// Examples of struct field tags and their meanings:
//
//	// Field appears in map as key "myName".
//	Field int `structof:"myName"`
//
//	// Field appears in map as key "myName" and
//	// the field is omitted from the object if its value is empty,
//	// as defined above.
//	Field int `structof:"myName,omitempty"`
//
//	// Field appears in map as key "Field" (the default), but
//	// the field is skipped if empty.
//	// Note the leading comma.
//	Field int `structof:",omitempty"`
//
//	// Field is ignored by this package.
//	Field int `structof:"-"`
//
//	// Field appears in map as key "-".
//	Field int `structof:"-,"`
//
// The "string" option signals that a field is stored as map inside a
// quoted string. It applies only to fields of string, floating point,
// integer, or boolean types. This extra level of encoding is sometimes used
// when communicating with JavaScript programs:
//
//	Int64String int64 `structof:",string"`
//
// The "inline" option signals a non-embedded struct field flatten its fields
// in the outside map. Example:
//
//	// The F's fields will be flattened into the output map.
//	F struct {A int; B string} `structof:",inline"`
//
// The key name will be used if it's a non-empty string consisting of
// only Unicode letters, digits, and ASCII punctuation except quotation
// marks, backslash, and comma.
//
// Anonymous struct fields are usually marshaled as if their inner exported fields
// were fields in the outer struct, subject to the usual Go visibility rules amended
// as described in the next paragraph.
// An anonymous struct field with a name given in its structof's tag is treated as
// having that name, rather than being anonymous.
// An anonymous struct field of interface type is treated the same as having
// that type as its name, rather than being anonymous.
//
// The Go visibility rules for struct fields are amended for structof when
// deciding which field to marshal or unmarshal. If there are
// multiple fields at the same level, and that level is the least
// nested (and would therefore be the nesting level selected by the
// usual Go rules), the following extra rules apply:
//
// 1) Of those fields, if any are structof-tagged, only tagged fields are considered,
// even if there are multiple untagged fields that would otherwise conflict.
//
// 2) If there is exactly one field (tagged or not according to the first rule), that is selected.
//
// 3) Otherwise there are multiple fields, and all are ignored; no error occurs.
//
// Map fields encode too. The map's key type must either be a
// string.
//
// Pointer values encode as the value pointed to if it not nil.
// A nil pointer encodes as the self.
//
// Interface values encode as the value contained in the interface if it not nil.
// A nil interface value omitted.
//
// Channel, complex, and function values unsupported.
// Attempting to encode such a value causes FillMap to panics with
// an UnsupportedTypeError.
//
// Passing cyclic structures to FillMap will result in
// panics.
func FillMap(s, i any) {
	rs := reflect.ValueOf(s)
	for reflect.Pointer == rs.Kind() && !rs.IsNil() {
		rs = rs.Elem()
	}
	if reflect.Struct != rs.Kind() {
		panic("not struct or pointer to struct")
	}

	v := reflect.ValueOf(i)
	if reflect.Pointer != v.Kind() || v.IsNil() || mapType != v.Type().Elem() {
		panic("expect non-nil pointer to map[string]any")
	}

	v = v.Elem()
	if v.IsNil() {
		v.Set(reflect.MakeMap(mapType))
	}

	e, put := newEncodeState(v.Interface())
	defer put()
	e.marshal(s, encOpts{})
}

// MakeMap is like FillMap. Instead allocates a new map and returns it.
// See FillMap function's documentation for more information.
func MakeMap(i any) map[string]any {
	var m map[string]any
	FillMap(i, &m)
	return m
}

// MakeSlice returns a list of field/value pairs of the struct.
// See FillMap function's documentation for more information.
func MakeSlice(i any) []any {
	v := reflect.ValueOf(i)
	for reflect.Pointer == v.Kind() && !v.IsNil() {
		v = v.Elem()
	}
	if reflect.Struct != v.Kind() {
		panic("not struct or pointer to struct")
	}

	var a []any
	e, put := newEncodeState(a)
	defer put()
	e.marshal(i, encOpts{structConvertToSlice: true})
	return e.Interface().([]any)
}

// An encodeState encodes struct into a map[string]any or []any.
type encodeState struct {
	m   map[string]any
	mOK bool

	s   []any
	sOK bool

	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
	ptrLevel uint
	ptrSeen  map[any]struct{}
}

const startDetectingCyclesAfter = 1000

var encodeStatePool sync.Pool

func newEncodeState(i any) (e *encodeState, put func()) {
	if v := encodeStatePool.Get(); v != nil {
		e = v.(*encodeState)
		if len(e.ptrSeen) > 0 {
			panic("ptrEncoder.encode should have emptied ptrSeen via defers")
		}
		e.ptrLevel = 0
	} else {
		e = &encodeState{ptrSeen: make(map[any]struct{})}
	}

	if e.m, e.mOK = i.(map[string]any); !e.mOK {
		var mm *map[string]any
		if mm, e.mOK = i.(*map[string]any); e.mOK {
			e.m = *mm
		}
	}
	if e.s, e.sOK = i.([]any); !e.sOK {
		var ss *[]any
		if ss, e.sOK = i.(*[]any); e.sOK {
			e.s = *ss
		}
	}
	if !e.mOK && !e.sOK {
		panic(fmt.Sprintf("unexpected value type %T", i))
	}
	put = func() { encodeStatePool.Put(e) }
	return e, put
}

func (e *encodeState) Interface() any {
	switch {
	case e.mOK:
		return e.m
	case e.sOK:
		return e.s
	default:
		panic("unreachable")
	}
}

// structofError is an error wrapper type for internal use only.
// Panics with errors are wrapped in structofError so that the top-level recover
// can distinguish intentional panics from this package.
type structofError struct{ error }

func (e *encodeState) marshal(v any, opts encOpts) {
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(structofError); ok {
				r = se.error
			}
			panic(r)
		}
	}()
	e.reflectValue(reflect.ValueOf(v), opts)
}

// error aborts the encoding by panicking with err wrapped in structofError.
func (e *encodeState) error(err error) {
	panic(structofError{err})
}

func (e *encodeState) setKeyValue(key string, elem any) {
	if elem == nil {
		return
	}
	switch {
	case e.mOK:
		if elem != nil {
			e.m[key] = elem
		}
	case e.sOK:
		if elem != nil {
			e.s = append(e.s, key, elem)
		}
	}
}

// An UnsupportedTypeError is returned by MapTo when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
	Key  string
}

func (e *UnsupportedTypeError) Error() string {
	return "structof: unsupported type: " + e.Type.String() + " for field: " + e.Key
}

// An UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "structof: unsupported value: " + e.Str
}

var hasIsZeroType = reflect.TypeOf((*interface{ IsZero() bool })(nil)).Elem()

func isEmptyValue(v reflect.Value) bool {
	var z interface{ IsZero() bool }
	if reflect.Pointer != v.Kind() && v.CanAddr() && reflect.PointerTo(v.Type()).Implements(hasIsZeroType) {
		va := v.Addr()
		z = va.Interface().(interface{ IsZero() bool })
	} else if v.CanInterface() && v.Type().Implements(hasIsZeroType) {
		z = v.Interface().(interface{ IsZero() bool })
	}
	if z != nil {
		return z.IsZero()
	}

	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

func (e *encodeState) reflectValue(v reflect.Value, opts encOpts) {
	valueEncoder(v)(e, "", v, opts)
}

type encOpts struct {
	// quoted causes primitive fields to be encoded inside strings.
	quoted bool
	// convertToSlice causes array to be converted to slice.
	convertToSlice bool
	// inline causes struct fields to be encoded outside map.
	inline bool
	// structConvertToSlice causes struct fields to be encoded inside slice.
	structConvertToSlice bool
}

type encoderFunc func(*encodeState, string, reflect.Value, encOpts)

func valueEncoder(v reflect.Value) encoderFunc {
	if !v.IsValid() {
		return invalidValueEncoder
	}
	return typeEncoder(v.Type())
}

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func typeEncoder(t reflect.Type) encoderFunc {
	if fi, ok := encoderCache.Load(t); ok {
		return fi.(encoderFunc)
	}

	// To deal with recursive types, populate the map with an
	// indirect func before we build it. This type waits on the
	// real func (f) to be ready and then calls it. This indirect
	// func is only used for recursive types.
	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(t, encoderFunc(func(e *encodeState, key string, elem reflect.Value, opts encOpts) {
		wg.Wait()
		f(e, key, elem, opts)
	}))
	if loaded {
		return fi.(encoderFunc)
	}

	// Compute the real encoder and replace the indirect func with it.
	f = newTypeEncoder(t)
	wg.Done()
	encoderCache.Store(t, f)
	return f
}

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type) encoderFunc {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return primitiveEncoder
	case reflect.Interface:
		return interfaceEncoder
	case reflect.Struct:
		return newStructEncoder(t)
	case reflect.Map:
		return newMapEncoder(t)
	case reflect.Slice:
		return newSliceEncoder(t)
	case reflect.Array:
		return newArrayEncoder(t)
	case reflect.Pointer:
		return newPtrEncoder(t)
	default:
		return unsupportedTypeEncoder
	}
}

func invalidValueEncoder(e *encodeState, key string, _ reflect.Value, _ encOpts) {
	e.error(errors.New("invalid value"))
	// e.setKeyValue(key, nil)
}

func primitiveEncoder(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if opts.quoted {
		e.setKeyValue(key, strconv.Quote(fmt.Sprint(v)))
	} else {
		e.setKeyValue(key, v.Interface())
	}
}

func interfaceEncoder(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if !v.IsNil() {
		valueEncoder(v.Elem())(e, key, v.Elem(), opts)
	}
}

func unsupportedTypeEncoder(e *encodeState, key string, elem reflect.Value, _ encOpts) {
	e.error(&UnsupportedTypeError{elem.Type(), key})
}

type structEncoder struct {
	fields structFields
}

type structFields struct {
	list []field
}

func (se structEncoder) encode(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if len(se.fields.list) == 0 {
		if key != "" && !opts.inline {
			if opts.quoted {
				e.setKeyValue(key, strconv.Quote(fmt.Sprint(v)))
			} else {
				e.setKeyValue(key, v.Interface())
			}
		}
		return
	}

	var ne *encodeState
	if key == "" || opts.inline {
		ne = e
	} else {
		var i any
		if opts.structConvertToSlice {
			i = []any(nil)
		} else {
			i = make(map[string]any)
		}
		e, put := newEncodeState(i)
		defer put()
		ne = e
	}

FieldLoop:
	for i := range se.fields.list {
		f := &se.fields.list[i]

		// Find the nested struct field by following f.index.
		fv := v
		for _, i := range f.index {
			if reflect.Pointer == fv.Kind() {
				if fv.IsNil() {
					continue FieldLoop
				}
				fv = fv.Elem()
			}
			fv = fv.Field(i)
		}

		if f.omitEmpty && isEmptyValue(fv) {
			continue
		}

		opts.quoted = f.quoted
		opts.inline = f.inline
		f.encoder(ne, f.name, fv, opts)
	}
	if e != ne {
		e.setKeyValue(key, ne.Interface())
	}
}

func newStructEncoder(t reflect.Type) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t)}
	return se.encode
}

type mapEncoder struct {
	elemEnc encoderFunc
}

func (me mapEncoder) encode(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if v.IsNil() {
		e.setKeyValue(key, v.Interface())
		return
	}

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.UnsafePointer()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}

	// Extract keys and values.
	m := make(map[string]any, v.Len())
	ne, put := newEncodeState(m)
	defer put()

	for mi := v.MapRange(); mi.Next(); {
		me.elemEnc(ne, mi.Key().String(), mi.Value(), opts)
	}

	elemType := v.Type().Elem()
	if elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}
	if elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Array {
		elemType = elemType.Elem()
	}

	if elemType.Kind() == reflect.Struct {
		e.setKeyValue(key, m)
	} else {
		vm := reflect.MakeMapWithSize(v.Type(), v.Len())
		for k, e := range m {
			vm.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(e))
		}
		e.setKeyValue(key, vm.Interface())
	}

	e.ptrLevel--
}

func newMapEncoder(t reflect.Type) encoderFunc {
	switch t.Key().Kind() {
	default:
		return unsupportedTypeEncoder
	case reflect.String:
	}
	me := mapEncoder{typeEncoder(t.Elem())}
	return me.encode
}

// sliceEncoder just wraps an arrayEncoder, checking to make sure the value isn't nil.
type sliceEncoder struct {
	arrayEnc encoderFunc
}

func (se sliceEncoder) encode(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if v.IsNil() {
		e.setKeyValue(key, v.Interface())
		return
	}

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		// Here we use a struct to memorize the pointer to the first element of the slice
		// and its length.
		ptr := struct {
			ptr interface{} // always an unsafe.Pointer, but avoids a dependency on package unsafe
			len int
		}{v.UnsafePointer(), v.Len()}
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	opts.convertToSlice = true
	se.arrayEnc(e, key, v, opts)
	e.ptrLevel--
}

func newSliceEncoder(t reflect.Type) encoderFunc {
	enc := sliceEncoder{newArrayEncoder(t)}
	return enc.encode
}

type arrayEncoder struct {
	elemEnc encoderFunc
}

var anyType = reflect.TypeOf((*any)(nil)).Elem()

func (ae arrayEncoder) encode(e *encodeState, key string, v reflect.Value, opts encOpts) {
	s := make([]any, 0, v.Len()*2)
	ne, put := newEncodeState(s)
	defer put()

	n := v.Len()
	for i := 0; i < n; i++ {
		ae.elemEnc(ne, strconv.Itoa(i), v.Index(i), opts)
	}
	s = ne.s

	elemType := v.Type().Elem()
	if elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}

	var a reflect.Value
	if elemType.Kind() == reflect.Struct {
		a = reflect.New(reflect.ArrayOf(v.Len(), anyType)).Elem()
	} else {
		a = reflect.New(reflect.ArrayOf(v.Len(), elemType)).Elem()
	}
	for i := 0; i < a.Len(); i++ {
		a.Index(i).Set(reflect.ValueOf(s[i*2+1]))
	}

	if opts.convertToSlice {
		e.setKeyValue(key, a.Slice(0, a.Len()).Interface())
	} else {
		e.setKeyValue(key, a.Interface())
	}
}

func newArrayEncoder(t reflect.Type) encoderFunc {
	enc := arrayEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe ptrEncoder) encode(e *encodeState, key string, v reflect.Value, opts encOpts) {
	if v.IsNil() {
		e.setKeyValue(key, v.Interface())
		return
	}
	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		// We're a large number of nested ptrEncoder.encode calls deep;
		// start checking if we've run into a pointer cycle.
		ptr := v.Interface()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encountered a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	pe.elemEnc(e, key, v.Elem(), opts)
	e.ptrLevel--
}

func newPtrEncoder(t reflect.Type) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem())}
	return enc.encode
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

func typeByIndex(t reflect.Type, index []int) reflect.Type {
	for _, i := range index {
		if reflect.Pointer == t.Kind() {
			t = t.Elem()
		}
		t = t.Field(i).Type
	}
	return t
}

// A field represents a single field found in a struct.
type field struct {
	name string

	tag       bool
	index     []int
	typ       reflect.Type
	omitEmpty bool
	quoted    bool
	inline    bool

	encoder encoderFunc
}

// byIndex sorts field by index sequence.
type byIndex []field

func (x byIndex) Len() int { return len(x) }

func (x byIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byIndex) Less(i, j int) bool {
	for k, xik := range x[i].index {
		if k >= len(x[j].index) {
			return false
		}
		if xik != x[j].index[k] {
			return xik < x[j].index[k]
		}
	}
	return len(x[i].index) < len(x[j].index)
}

// typeFields returns a list of fields that the package should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
func typeFields(t reflect.Type) structFields {
	// Anonymous fields to explore at the current level and the next.
	current := []field{}
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, make(map[reflect.Type]int)

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			hasExported := false

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					ft := sf.Type
					if reflect.Pointer == ft.Kind() {
						ft = ft.Elem()
					}
					if !sf.IsExported() && reflect.Struct != ft.Kind() {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}

					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if !sf.IsExported() {
					// Ignore unexported non-embedded fields.
					continue
				}
				hasExported = true

				tag, _ := structtag.StructTag(sf.Tag).Lookup("structof")
				if tag.String() == `structof:"-"` {
					continue
				}

				name, opts := tag.Name, tag.Options
				if !isValidTag(name) {
					name = ""
				}

				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i

				ft := sf.Type
				if ft.Name() == "" && reflect.Pointer == ft.Kind() {
					// Follow pointer.
					ft = ft.Elem()
				}

				// Only strings, floats, integers, and booleans can be quoted.
				quoted := false
				if opts.Contains("string") {
					switch ft.Kind() {
					case reflect.Bool,
						reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
						reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
						reflect.Float32, reflect.Float64,
						reflect.String,
						reflect.Struct:
						quoted = true
					}
				}

				// Only structs can be inline.
				inline := false
				if opts.Contains("inline") {
					switch ft.Kind() {
					case reflect.Struct:
						inline = true
					}
				}

				// Record found field and index sequence.
				if name != "" || !sf.Anonymous || reflect.Struct != ft.Kind() {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}

					field := field{
						name:      name,
						tag:       tagged,
						index:     index,
						typ:       ft,
						omitEmpty: opts.Contains("omitempty"),
						quoted:    quoted,
						inline:    inline,
					}

					fields = append(fields, field)
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 or 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					continue
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}

					next = append(next, field{
						name:      name,
						tag:       tagged,
						index:     index,
						typ:       ft,
						omitEmpty: opts.Contains("omitempty"),
						quoted:    quoted,
						inline:    inline,
					})
				}
			}

			if !hasExported && f.name != "" && reflect.Struct == f.typ.Kind() {
				field := f
				fields = append(fields, field)
				if count[f.typ] > 1 {
					// If there were multiple instances, add a second,
					// so that the annihilation code will see a duplicate.
					// It only cares about the distinction between 1 or 2,
					// so don't bother generating any more copies.
					fields = append(fields, fields[len(fields)-1])
				}
			}
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		x := fields
		// sort field by name, breaking ties with depth, then
		// breaking ties with "name came from structof tag", then
		// breaking ties with index sequence.
		if x[i].name != x[j].name {
			return x[i].name < x[j].name
		}
		if len(x[i].index) != len(x[j].index) {
			return len(x[i].index) < len(x[j].index)
		}
		if x[i].tag != x[j].tag {
			return x[i].tag
		}
		return byIndex(x).Less(i, j)
	})

	// Delete all fields that are hidden by the Go rules for embedded fields,
	// except that fields with structof tags are promoted.

	// The fields are sorted in primary order of name, secondary order
	// of field index length. Loop over names; for each name, delete
	// hidden fields by choosing the one dominant field that survives.
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name.
		// Find the sequence of fields with the name of this first field.
		fi := fields[i]
		name := fi.name
		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}
		if advance == 1 { // Only one field with this name
			out = append(out, fi)
			continue
		}
		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	sort.Sort(byIndex(fields))

	for i := range fields {
		f := &fields[i]
		f.encoder = typeEncoder(typeByIndex(t, f.index))
	}
	return structFields{fields}
}

// dominantField looks through the fields, all of which are known to
// have the same name, to find the single field that dominates the
// others using Go's embedding rules, modified by the presence of
// structof tags. If there are multiple top-level fields, the boolean
// will be false: This condition is an error in Go and we skip all
// the fields.
func dominantField(fields []field) (field, bool) {
	// The fields are sorted in increasing index-length order, then by presence of tag.
	// That means that the first field is the dominant one. We need only check
	// for error cases: two fields at top level, either both tagged or neither tagged.
	if len(fields) > 1 && len(fields[0].index) == len(fields[1].index) && fields[0].tag == fields[1].tag {
		return field{}, false
	}
	return fields[0], true
}

var fieldCache sync.Map // map[reflect.Type]structFields

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type) structFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(structFields)
	}
	f, _ := fieldCache.LoadOrStore(t, typeFields(t))
	return f.(structFields)
}
