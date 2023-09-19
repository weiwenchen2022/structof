package structof

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMakeMapNonStruct(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Passing a non struct into MakeMap should panic")
		}
	}()
	_ = MakeMap("")
}

func TestMakeMap(t *testing.T) {
	t.Parallel()

	type S struct {
		A int
		B string
	}
	s := S{23, "foobar"}
	m := MakeMap(s)
	want := map[string]any{"A": 23, "B": "foobar"}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapWithTag(t *testing.T) {
	t.Parallel()

	type S struct {
		A int    `structof:"a"`
		B string `structof:"b"`
	}
	s := S{23, "foobar"}
	m := MakeMap(s)

	want := map[string]any{"a": 23, "b": "foobar"}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapOmitEmpty(t *testing.T) {
	t.Parallel()

	type S struct {
		A int
		B string    `structof:",omitempty"`
		C time.Time `structof:",omitempty"`
	}
	s := S{}
	m := MakeMap(s)

	want := map[string]any{"A": 0}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapOmiteNested(t *testing.T) {
	t.Parallel()

	type S struct {
		A int       `structof:"a,omitempty"`
		B string    `structof:"b,omitempty"`
		C time.Time `structof:"c,omitempty"`
	}

	now := time.Now()
	s := S{C: now}
	m := MakeMap(s)
	want := map[string]any{"c": now}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNested(t *testing.T) {
	t.Parallel()

	type Inner struct {
		A string
	}
	type Outer struct {
		I *Inner
	}
	outer := Outer{&Inner{"foobar"}}
	m := MakeMap(outer)
	want := map[string]any{"I": map[string]any{"A": "foobar"}}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedMapWithStructElement(t *testing.T) {
	t.Parallel()

	type S1 struct {
		A int
	}
	type S2 struct {
		M map[string]*S1
	}
	s2 := S2{
		M: map[string]*S1{
			"foobar": {23},
		},
	}
	m := MakeMap(s2)

	want := map[string]any{
		"M": map[string]any{
			"foobar": map[string]any{"A": 23},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedMapWithStringElement(t *testing.T) {
	t.Parallel()

	type S1 struct {
		M map[string]string
	}
	type S2 struct {
		S1 *S1
	}
	s2 := S2{&S1{map[string]string{"foo": "bar"}}}
	m := MakeMap(s2)
	want := map[string]any{
		"S1": map[string]any{
			"M": map[string]string{
				"foo": "bar",
			},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedMapWithInterfaceElement(t *testing.T) {
	t.Parallel()

	type S1 struct {
		M map[string]any
	}
	type S2 struct {
		S1 *S1
	}
	s2 := S2{&S1{map[string]any{"foo": "bar"}}}
	m := MakeMap(s2)
	want := map[string]any{
		"S1": map[string]any{
			"M": map[string]any{
				"foo": "bar",
			},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedMapWithIntSlice(t *testing.T) {
	type S1 struct {
		M map[string][]int
	}
	type S2 struct {
		S1 *S1
	}
	s2 := S2{&S1{map[string][]int{
		"foobar": {23},
	}}}
	m := MakeMap(s2)
	want := map[string]any{
		"S1": map[string]any{
			"M": map[string][]int{
				"foobar": {23},
			},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedMapWithStructSlice(t *testing.T) {
	t.Parallel()

	type Address struct {
		Country string `structof:"country"`
	}
	type S1 struct {
		M map[string][]Address
	}
	type S2 struct {
		S1 *S1
	}

	s2 := S2{&S1{map[string][]Address{
		"example": {{"Turkey"}},
	}}}
	m := MakeMap(s2)

	want := map[string]any{
		"S1": map[string]any{
			"M": map[string]any{
				"example": []any{
					map[string]any{
						"country": "Turkey",
					},
				},
			},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedSliceWithStructElement(t *testing.T) {
	t.Parallel()

	type Address struct {
		Country string `structof:"country"`
	}
	type Person struct {
		Name      string    `structof:"name"`
		Addresses []Address `structof:"addresses"`
	}
	p := Person{
		Name: "foobar",
		Addresses: []Address{
			{"England"},
			{"Italy"},
		},
	}
	m := MakeMap(p)
	want := map[string]any{
		"name": "foobar",
		"addresses": []any{
			map[string]any{"country": "England"},
			map[string]any{"country": "Italy"},
		},
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapNestedIntSlice(t *testing.T) {
	t.Parallel()

	type S struct {
		Ints []int `structof:"ints"`
	}
	s := S{[]int{23}}
	m := MakeMap(s)
	want := map[string]any{"ints": []int{23}}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapAnonymous(t *testing.T) {
	t.Parallel()

	type S1 struct {
		A string
	}
	type S2 struct {
		*S1
	}
	s2 := S2{&S1{"foobar"}}
	m := MakeMap(s2)
	want := map[string]any{
		"A": "foobar",
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapInline(t *testing.T) {
	t.Parallel()

	type S1 struct {
		A string `structof:"a"`
	}
	type S2 struct {
		S1 *S1 `structof:",inline"`
	}
	s2 := S2{&S1{"foobar"}}
	m := MakeMap(s2)
	want := map[string]any{
		"a": "foobar",
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapInlineOverwrite(t *testing.T) {
	t.Parallel()

	type S1 struct {
		A string `structof:"a"`
	}
	type S2 struct {
		S1 *S1    `structof:",inline"`
		A  string `structof:"a"`
	}
	s2 := S2{&S1{"s1"}, "s2"}
	m := MakeMap(s2)
	want := map[string]any{
		"a": "s2",
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapTimeField(t *testing.T) {
	t.Parallel()

	type S struct {
		CreatedAt time.Time `structof:"createdAt,omitempty"`
	}

	now := time.Now()
	tests := []struct {
		s    S
		want map[string]any
	}{
		{S{now}, map[string]any{"createdAt": now}},
		{S{}, map[string]any{}},
	}

	for _, tt := range tests {
		m := MakeMap(tt.s)
		if !cmp.Equal(tt.want, m) {
			t.Error(cmp.Diff(tt.want, m))
		}
	}
}

func TestFillMap(t *testing.T) {
	t.Parallel()

	type T struct {
		A int
		B string
	}
	var m map[string]any
	FillMap(T{23, "foobar"}, &m)

	want := map[string]any{
		"A": 23,
		"B": "foobar",
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestFillMapNil(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("FillMap should panic if a nil passed in")
		} else if s, ok := r.(string); !ok || !strings.Contains(s, "map") {
			t.Error(`FillMap should panic with string "...map..." message`)
		}
	}()
	FillMap(struct{}{}, nil)
}

func TestMakeSlice(t *testing.T) {
	t.Parallel()

	type T struct {
		A int
		B string
	}
	s := MakeSlice(&T{23, "foobar"})
	want := []any{"A", 23, "B", "foobar"}
	if !cmp.Equal(want, s) {
		t.Error(cmp.Diff(want, s))
	}
}

func TestMakeSliceOmitempty(t *testing.T) {
	t.Parallel()

	type S struct {
		A int `structof:"a,omitempty"`
	}
	s := MakeSlice(&S{})
	want := []any(nil)
	if !cmp.Equal(want, s) {
		t.Error(cmp.Diff(want, s))
	}
}

func TestMakeSliceNested(t *testing.T) {
	t.Parallel()

	type S1 struct {
		B string
	}
	type S2 struct {
		A  int
		S1 *S1
	}
	s := MakeSlice(&S2{23, &S1{"foobar"}})
	want := []any{"A", 23, "S1", []any{"B", "foobar"}}
	if !cmp.Equal(want, s) {
		t.Error(cmp.Diff(want, s))
	}
}

func TestMakeSliceAnonymous(t *testing.T) {
	t.Parallel()

	type S1 struct {
		A string
	}
	type S2 struct {
		*S1
	}
	s := MakeSlice(&S2{&S1{"foobar"}})
	want := []any{"A", "foobar"}
	if !cmp.Equal(want, s) {
		t.Error(cmp.Diff(want, s))
	}
}

func TestNestedNilPointer(t *testing.T) {
	t.Parallel()

	type (
		S1 struct {
		}
		S2 struct {
			S1 *S1
		}
		S3 struct {
			S2 *S2
		}
	)

	tests := []struct {
		i           any
		shouldPanic bool
	}{
		{i: S3{}},
		{i: S3{&S2{}}},
		{i: S3{&S2{&S1{}}}},
	}

	for _, tt := range tests {
		didPanic := make(chan bool, 1)
		go func() {
			defer func() { didPanic <- recover() != nil }()
			_ = MakeMap(tt.i)
		}()

		if tt.shouldPanic != <-didPanic {
			t.Errorf("MakeMap(%#v) panic mismatch %t", tt.i, !tt.shouldPanic)
		}
	}
}

type S1 struct {
	a int
	b string
}

func (s S1) String() string {
	return fmt.Sprintf("%s(%d)", s.b, s.a)
}

func TestTagWithStringOption(t *testing.T) {
	t.Parallel()

	type T struct {
		S1 *S1 `structof:"s1,string"`
	}
	m := MakeMap(T{&S1{23, "foobar"}})
	want := map[string]any{
		"s1": `"foobar(23)"`,
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestMakeMapInterfaceValue(t *testing.T) {
	t.Parallel()

	type T struct {
		I any
	}
	m := MakeMap(T{[]byte("hello world")})
	want := map[string]any{
		"I": []byte("hello world"),
	}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}

func TestPointer2Pointer(t *testing.T) {
	t.Parallel()

	type S struct{}
	var s S
	ps := &s
	pps := &ps
	m := MakeMap(&pps)
	want := map[string]any{}
	if !cmp.Equal(want, m) {
		t.Error(cmp.Diff(want, m))
	}
}
