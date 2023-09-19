package structof

import (
	"testing"

	"github.com/weiwenchen2022/structtag"
)

func TestField_Set(t *testing.T) {
	t.Parallel()

	type (
		S1 struct {
			A int
		}
		S2 struct {
			B string
		}
		S3 struct {
			A           int
			B           string
			nonexported string // not exported
			S1          *S1
			*S2         // embedded
		}
	)

	s := MakeStruct(&S3{})

	f, err := s.FieldByName("A")
	if err != nil {
		t.Fatal(err)
	}

	a := 23
	f.Set(a)
	if a != f.Interface() {
		t.Errorf("field 'A' set wrong got %d, want %d", f.Interface(), a)
	}

	// pass a different type
	didPanic := make(chan bool, 1)
	go func() {
		defer func() { didPanic <- recover() != nil }()
		f.Set("foobar") // Field A is type int, but we pass string
	}()
	if !<-didPanic {
		t.Error("set field pass with a different type should panic")
	}

	// value keep unchange
	if a != f.Interface() {
		t.Errorf("set field panic value change %d, want %d", f.Interface(), a)
	}

	// obtain unexported field, should returns error
	f, err = s.FieldByName("nonexported")
	if err == nil {
		t.Error("obtain unexported field should returns error")
	}

	// set a pointer to struct
	f, err = s.FieldByName("S1")
	if err != nil {
		t.Fatal(err)
	}

	s1 := &S1{23}
	f.Set(s1)
	if s1 != f.Interface() {
		t.Errorf("could not set S1 got %p want %p", f.Interface(), s1)
	}

	f, err = s.FieldByName("S2")
	if err != nil {
		t.Fatal(err)
	}

	s2 := &S2{"foobar"}
	f.Set(s2)
	if s2 != f.Interface() {
		t.Errorf("could not set S2 got %p want %p", f.Interface(), s2)
	}
}

func TestField_SetZero(t *testing.T) {
	t.Parallel()

	type (
		S1 struct {
			A int
		}
		S2 struct {
			B string
		}
		S3 struct {
			A           int
			B           string
			nonexported string // not exported
			S1          *S1
			*S2         // embedded
		}
	)

	s := MakeStruct(&S3{A: 23})

	f, err := s.FieldByName("A")
	if err != nil {
		t.Fatal(err)
	}

	f.SetZero()
	if f.Interface() != 0 {
		t.Errorf("SetZero got %d want %d", f.Interface(), 0)
	}

	// set an unexported field, which should panic
	didPanic := make(chan bool, 1)
	go func() {
		defer func() { didPanic <- recover() != nil }()
		f, _ := s.FieldByName("nonexported")
		f.SetZero()
	}()
	if !<-didPanic {
		t.Error("should panic")
	}

	f, err = s.FieldByName("S1")
	if err != nil {
		t.Fatal(err)
	}
	f.SetZero()

	if (*S1)(nil) != f.Interface() {
		t.Errorf("got %p", f.Interface())
	}

	f, err = s.FieldByName("S2")
	if err != nil {
		t.Fatal(err)
	}

	f.SetZero()
	if (*S2)(nil) != f.Interface() {
		t.Errorf("got %p", f.Interface())
	}
}

func TestNonExistsField(t *testing.T) {
	t.Parallel()

	_, err := MakeStruct(&struct{}{}).FieldByName("nonexists")
	if err == nil {
		t.Error("Retrieveing a non existing field from the struct should returns error")
	}
}

func TestField_Tag(t *testing.T) {
	t.Parallel()

	type S struct {
		F0 string `species:"gopher"`
		F1 string `species:""`
	}

	s := MakeStruct(&S{})
	f0, err := s.FieldByName("F0")
	if err != nil {
		t.Fatal(err)
	}
	tag := f0.Tag("species")
	if tag.Key != "species" && tag.Name != "gopher" {
		t.Errorf("%+v", tag)
	}

	f1, err := s.FieldByName("F1")
	if err != nil {
		t.Fatal(err)
	}
	tag = f1.Tag("species")
	if tag.Key != "species" && tag.Name != "F1" {
		t.Errorf("%+v", tag)
	}

	tag = f1.Tag("json")
	if tag != (structtag.Tag{}) {
		t.Errorf("%+v", tag)
	}
}

func TestField_Interface(t *testing.T) {
	t.Parallel()

	type S struct {
		A int
		B string
	}
	s := MakeStruct(&S{23, "foobar"})

	f, err := s.FieldByName("A")
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := f.Interface().(int); !ok || a != 23 {
		t.Errorf("Field A got %d want %d", f.Interface(), 23)
	}
}

func TestField_IsEmbedded(t *testing.T) {
	t.Parallel()

	type (
		S1 struct{}
		S2 struct {
			*S1
			A int
		}
	)
	s := MakeStruct(&S2{})
	for _, tt := range []struct {
		name string
		want bool
	}{
		{"S1", true},
		{"A", false},
	} {
		f, err := s.FieldByName(tt.name)
		if err != nil {
			t.Error(err)
			continue
		}
		if tt.want != f.IsEmbedded() {
			t.Errorf("Field %q IsEmbedded %t", tt.name, !tt.want)
		}
	}
}

func TestField_Name(t *testing.T) {
	t.Parallel()

	s := MakeStruct(&struct{ F string }{})
	f, err := s.FieldByName("F")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name() != "F" {
		t.Errorf("Field 'F''s name got %s want F", f.Name())
	}
}

func TestNestField(t *testing.T) {
	t.Parallel()

	type (
		S1 struct {
			A int
		}
		S2 struct {
			B string
		}
		S3 struct {
			*S1
			S2 *S2
		}
	)

	s := MakeStruct(&S3{&S1{}, &S2{}})
	f, err := s.FieldByName("S1.A")
	if err != nil {
		t.Fatal(err)
	}

	f.Set(23)
	f, err = s.FieldByName("S1")
	if err != nil {
		t.Fatal(err)
	}
	if a := f.Interface().(*S1).A; a != 23 {
		t.Errorf("The value of the field 'A' inside 'S1' struct got %d want 23", a)
	}

	f, err = s.FieldByName("S2.B")
	if err != nil {
		t.Fatal(err)
	}
	f.Set("foobar")

	f, err = s.FieldByName("S2")
	if err != nil {
		t.Fatal(err)
	}
	if b := f.Interface().(*S2).B; b != "foobar" {
		t.Errorf(`The value of the field 'B' inside 'S2' struct got %s want "foobar"`, b)
	}
}
