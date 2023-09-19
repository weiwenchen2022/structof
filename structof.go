package structof

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/weiwenchen2022/structtag"
)

// Struct encapsulates a struct type to provide several high level functions around the struct.
type Struct struct {
	v   reflect.Value
	typ reflect.Type
}

// MakeStruct returns a Struct with the struct i.
// It panics if the i's is not non-nil pointer to struct.
func MakeStruct(i any) Struct {
	v := reflect.ValueOf(i)
	if reflect.Pointer != v.Kind() || v.IsNil() || reflect.Struct != v.Type().Elem().Kind() {
		panic("not a non-nil pointer to struct")
	}
	v = v.Elem()
	return Struct{v: v, typ: v.Type()}
}

// FillMap fills into the map[string]any with struct field name as the key, and field value as element.
// If i's Kind not struct or pointer to struct,
// or v's type not map[string]any, or pointer to map[string]any, FillMap panics.
func (s Struct) FillMap(i any) {
	FillMap(s.v.Addr().Interface(), i)
}

// MakeMap converts the struct s to a map[string]any.
// See FillMap function's documentation for more information.
func (s Struct) MakeMap() map[string]any {
	var m map[string]any
	FillMap(s.v.Addr().Interface(), &m)
	return m
}

func (s Struct) MakeSlice() []any {
	return MakeSlice(s.v.Addr().Interface())
}

// Fields returns a slice of StructField.
// See Fields function's documentation for more information.
func (s Struct) Fields() []Field {
	return Fields(s.v.Addr().Interface())
}

func (s Struct) FieldNames() []string {
	fields := cachedTypeFields(s.typ)
	names := make([]string, len(fields.list))
	for i := range fields.list {
		f := &fields.list[i]
		names[i] = f.name
	}
	return names
}

// FieldByName returns a single exported struct field that provides several high level functions
// and a boolean indicating if the field was found.
func (s Struct) FieldByName(name string) (Field, error) {
	ft := s.typ
	var sf reflect.StructField

	names := strings.Split(name, ".")
	index := make([]int, len(names))
	for i, n := range names {
		var ok bool
		sf, ok = ft.FieldByNameFunc(func(s string) bool { return n == s })
		if !ok {
			return Field{}, fmt.Errorf("field %q not found", name)
		}

		if !sf.IsExported() {
			return Field{}, fmt.Errorf("field %q not exported", name)
		}

		index[i] = sf.Index[0]
		if len(names)-1 == i {
			break
		}

		ft = sf.Type
		if reflect.Pointer == sf.Type.Kind() {
			// Follow pointer.
			ft = sf.Type.Elem()
		}
		if reflect.Struct != ft.Kind() {
			return Field{}, fmt.Errorf("field %q not struct or pointer to struct",
				strings.Join(names[:i+1], "."))
		}
	}
	sf.Index = index

	f, err := s.v.FieldByIndexErr(sf.Index)
	if err != nil {
		return Field{}, err
	}
	return Field{v: f, sf: sf}, nil
}

// Name returns the s's type name within its package.
// For non-defined types it returns the empty string.
func (s Struct) Name() string {
	return s.typ.Name()
}

// IsZero reports whether v is the zero value for its type.
// It panics if the argument is nil.
func IsZero(i any) bool {
	return reflect.ValueOf(i).IsZero()
}

// IsStruct reports whether i's kind is a struct or a pointer to struct.
func IsStruct(i any) bool {
	t := reflect.TypeOf(i)
	if reflect.Pointer == t.Kind() {
		t = t.Elem()
	}
	return reflect.Struct == t.Kind()
}

// TypeName returns the dynamic type's name within its package.
// For non-defined types it returns the empty string.
// It panics if i is a nil interface value.
func TypeName(i any) string {
	return reflect.TypeOf(i).Name()
}

// FieldNames returns a list of the struct type's field name.
// It panics if the v's kind is not struct or pointer to struct.
func FieldNames(i any) []string {
	t := reflect.TypeOf(i)
	if reflect.Pointer == t.Kind() {
		t = t.Elem()
	}
	if reflect.Struct != t.Kind() {
		panic("not struct")
	}

	fieldNames := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		fieldNames[i] = t.Field(i).Name
	}
	return fieldNames
}

// Fields returns a list of exported Field.
// It panics if i is not non-nil pointer to struct.
//
// As a special case, if the field tag is "-", the field is always omitted.
func Fields(i any) []Field {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Type().Elem().Kind() != reflect.Struct {
		panic("not non-nil pointer to struct")
	}
	v = v.Elem()

	fields := cachedTypeFields(v.Type())
	fs := make([]Field, len(fields.list))
	j := 0
	typ := v.Type()
	for i := range fields.list {
		f := &fields.list[i]
		fv, err := v.FieldByIndexErr(f.index)
		if err != nil {
			continue
		}
		fs[i] = Field{v: fv, sf: typ.FieldByIndex(f.index)}
		j++
	}
	return fs[:j]
	// return MakeStruct(i).Fields()
}

// Field represents a single struct field that encapsulates high level functions around the field.
type Field struct {
	v  reflect.Value
	sf reflect.StructField
}

// Tag returns the tag associated with key in the tag string.
// If there is no such key in the tag, Tag returns the zero value.
func (f Field) Tag(key string) structtag.Tag {
	tag, _ := structtag.StructTag(f.sf.Tag).Lookup(key)
	return tag
}

// Interface returns f's current underlying value as an any. It is equivalent to:
//
//	var i any = (v's underlying value)
func (f Field) Interface() any {
	return f.v.Interface()
}

// IsEmbedded reports whether the field is an embedded field.
func (f Field) IsEmbedded() bool {
	return f.sf.Anonymous
}

// IsZero reports whether f is the zero value for its type.
func (f Field) IsZero() bool {
	return f.v.IsZero()
}

// Name returns the field name.
func (f Field) Name() string {
	return f.sf.Name
}

// Type returns the field's type.
func (f Field) Type() reflect.Type {
	return f.sf.Type
}

// Kind returns the field's kind.
func (f Field) Kind() reflect.Kind {
	return f.sf.Type.Kind()
}

// Set assigns x to the value v.
// It panics if as in Go, i's value cannot be assignable to f's type.
func (f Field) Set(i any) {
	v := reflect.ValueOf(i)
	if f.v.Kind() != v.Kind() {
		panic(fmt.Sprintf("kind not match %s != %s", f.v.Kind(), v.Kind()))
	}
	f.v.Set(v.Convert(f.v.Type()))
}

// SetZero sets f to be the zero value of f's type.
func (f Field) SetZero() {
	f.v.SetZero()
}
