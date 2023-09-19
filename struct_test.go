package structof

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestStruct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		i    any
		want bool
	}{
		{struct{}{}, true},
		{&struct{}{}, true},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsStruct(tt.i); tt.want != got {
			t.Errorf("IsStruct(%#v) = %t", tt.i, !tt.want)
		}
	}
}

func TestNames(t *testing.T) {
	t.Parallel()

	type T struct {
		A int
		B string
	}
	s := FieldNames(T{})
	want := []string{"A", "B"}
	if !cmp.Equal(want, s) {
		t.Error(cmp.Diff(want, s))
	}
}

func TestFields(t *testing.T) {
	t.Parallel()

	type T struct {
		A int
		B string
	}

	fields := Fields(&T{})
	names := make([]string, len(fields))
	for i := range fields {
		f := &fields[i]
		names[i] = f.Name()
	}

	want := []string{"A", "B"}
	if !cmp.Equal(want, names) {
		t.Error(cmp.Diff(want, names))
	}
}

func TestFields_omitted(t *testing.T) {
	t.Parallel()

	type T struct {
		A int `structof:"-"`
		B string
	}

	fields := Fields(&T{})
	names := make([]string, len(fields))
	for i := range fields {
		f := &fields[i]
		names[i] = f.Name()
	}

	want := []string{"B"}
	if !cmp.Equal(want, names) {
		t.Error(cmp.Diff(want, names))
	}
}

func TestFields_anonymous(t *testing.T) {
	t.Parallel()

	type S struct {
		A int
		B string
	}
	type T struct {
		S
	}

	fields := Fields(&T{})
	names := make([]string, len(fields))
	for i := range fields {
		f := &fields[i]
		names[i] = f.Name()
	}
	want := []string{"A", "B"}

	if !cmp.Equal(want, names) {
		t.Error(cmp.Diff(want, names))
	}
}

func TestName(t *testing.T) {
	t.Parallel()

	type T struct{}

	tests := []struct {
		i           any
		name        string
		shouldPanic bool
	}{
		{i: T{}, name: "T"},
		{i: struct{}{}, name: ""},
		{i: "", name: "string"},
		{i: nil, shouldPanic: true},
	}

	for _, tt := range tests {
		didPanic := make(chan bool, 1)
		name := make(chan string, 1)
		go func() {
			defer func() {
				r := recover()
				if r != nil {
					name <- ""
				}
				didPanic <- r != nil
			}()
			name <- TypeName(tt.i)
		}()

		if tt.shouldPanic != <-didPanic {
			if !tt.shouldPanic {
				t.Errorf("Name(%#v) unexpected panic", tt.i)
			}
			if tt.shouldPanic {
				t.Errorf("Name(%#v) should panic", tt.i)
			}
		}
		if name := <-name; tt.name != name {
			t.Errorf("Name(%#v) got %s want %s", tt.i, name, tt.name)
		}
	}
}
