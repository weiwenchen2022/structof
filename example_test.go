package structof_test

import (
	"fmt"

	"github.com/weiwenchen2022/structof"
)

func ExampleMakeStruct() {
	type T struct {
		A int
		B string
	}

	t := &T{23, "foobar"}
	s := structof.MakeStruct(t)

	fmt.Println("Name:", s.Name())
	fmt.Printf("KeyValues: %#v\n", s.MakeSlice())
	f, _ := s.FieldByName("A")
	fmt.Println("Value of A:", f.Interface())

	// Output:
	// Name: T
	// KeyValues: []interface {}{"A", 23, "B", "foobar"}
	// Value of A: 23
}

func ExampleMakeMap() {
	type T struct {
		A int
		B string
	}
	t := T{23, "foobar"}
	m := structof.MakeMap(t)
	fmt.Println(m)

	// Output:
	// map[A:23 B:foobar]
}

func ExampleMakeMap_customTags() {
	// Custom tags can change the map keys instead of using the fields name
	type T struct {
		A int    `structof:"a"`
		B string `structof:"b"`
	}
	t := T{23, "foobar"}
	m := structof.MakeMap(t)
	fmt.Println(m)

	// Output:
	// map[a:23 b:foobar]
}

func ExampleMakeMap_omitempty() {
	// By default field with struct types of zero values are processed too. We
	// can stop processing them via "omitempty" tag option.
	type T struct {
		A int    `structof:",omitempty"`
		B string `structof:",omitempty"`
	}

	m := structof.MakeMap(T{})
	fmt.Println(m)

	// Output:
	// map[]
}

func ExampleMakeSlice() {
	type T struct {
		A int
		B string
	}
	t := &T{23, "foobar"}

	s := structof.MakeSlice(t)
	fmt.Println(s)

	// Output:
	// [A 23 B foobar]
}

func ExampleMakeSlice_omitempty() {
	// By default field with struct types of zero values are processed too. We
	// can stop processing them via "omitempty" tag option.
	type T struct {
		A int    `structof:",omitempty"`
		B string `structof:",omitempty"`
	}

	s := structof.MakeSlice(&T{})
	fmt.Println(s)

	// Output:
	// []
}

func ExampleMakeSlice_omitted() {
	type T struct {
		A int `structof:"-"`
		B string
	}

	t := &T{23, "foobar"}
	s := structof.MakeSlice(t)
	fmt.Println(s)

	// Output:
	// [B foobar]
}

func ExampleFields() {
	type T struct {
		A int
		B string
	}

	t := &T{23, "foobar"}
	fs := structof.Fields(t)
	for i := range fs {
		f := fs[i]
		fmt.Printf("%d: %s %s = %v\n", i, f.Name(), f.Type(), f.Interface())
	}

	// Output:
	// 0: A int = 23
	// 1: B string = foobar
}

func ExampleField_nested() {
	type T struct {
		A int
		B string
	}
	type S struct {
		T T
	}
	s := &S{T{23, "foobar"}}
	t := structof.MakeStruct(s)
	f, _ := t.FieldByName("T.A")
	fmt.Println("S.T.A:", f.Interface())

	// Output:
	// S.T.A: 23
}

func ExampleField() {
	type Inner struct {
		A int
		B string
	}
	type Outer struct {
		A int
		B string
		*Inner
	}

	t := Outer{23, "skidoo", &Inner{46, "foobar"}}
	s := structof.MakeStruct(&t)

	inner, err := s.FieldByName("Inner")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", inner.Interface())

	innnerB, err := s.FieldByName("Inner.B")
	if err != nil {
		panic(err)
	}
	fmt.Println(innnerB.Interface())

	//Output:
	// &{A:46 B:foobar}
	// foobar
}
