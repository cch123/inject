// Package inject provides utilities for mapping and injecting dependencies in various ways.
package inject

import (
	"fmt"
	"reflect"
)

// Injector represents an interface for mapping and injecting dependencies into structs
// and function arguments.
// 接口也可以像类一样进行下面这样的组合
// 实现父接口的话，就需要把子接口的方法都实现一遍
type Injector interface {
	Applicator
	//负责进行方法调用
	Invoker
	TypeMapper
	// SetParent sets the parent of the injector. If the injector cannot find a
	// dependency in its Type map it will check its parent before returning an
	// error.
	// 设置父injector，在查找map的时候会一级一级往上找
	SetParent(Injector)
}

// Applicator represents an interface for mapping dependencies to a struct.
type Applicator interface {
	// Maps dependencies in the Type map to each field in the struct
	// that is tagged with 'inject'. Returns an error if the injection
	// fails.
	Apply(interface{}) error
}

// Invoker represents an interface for calling functions via reflection.
type Invoker interface {
	// Invoke attempts to call the interface{} provided as a function,
	// providing dependencies for function arguments based on Type. Returns
	// a slice of reflect.Value representing the returned values of the function.
	// Returns an error if the injection fails.
	Invoke(interface{}) ([]reflect.Value, error)
}

// TypeMapper represents an interface for mapping interface{} values based on type.
// 其实就是一个封装过的map
// key => value的type
// value => 具体值
type TypeMapper interface {
	// Maps the interface{} value based on its immediate type from reflect.TypeOf.
	Map(interface{}) TypeMapper
	// Maps the interface{} value based on the pointer of an Interface provided.
	// This is really only useful for mapping a value as an interface, as interfaces
	// cannot at this time be referenced directly without a pointer.
	MapTo(interface{}, interface{}) TypeMapper
	// Provides a possibility to directly insert a mapping based on type and value.
	// This makes it possible to directly map type arguments not possible to instantiate
	// with reflect like unidirectional channels.
	Set(reflect.Type, reflect.Value) TypeMapper
	// Returns the Value that is mapped to the current type. Returns a zeroed Value if
	// the Type has not been mapped.
	Get(reflect.Type) reflect.Value
}

// 主要的数据结构
type injector struct {
	values map[reflect.Type]reflect.Value
	// 如果在当前的map里没有找到对应的value
	//那么会一级一级向上查找parent这个Injector
	parent Injector
}

// InterfaceOf dereferences a pointer to an Interface type.
// It panics if value is not an pointer to an interface.
func InterfaceOf(value interface{}) reflect.Type {
	t := reflect.TypeOf(value)

	//这里为啥要用for循环呢？
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	//如果指针指向的不是interface{}
	//那么panic
	if t.Kind() != reflect.Interface {
		panic("Called inject.InterfaceOf with a value that is not a pointer to an interface. (*MyInterface)(nil)")
	}

	return t
}

// New returns a new Injector.
// 返回一个Injector接口包装的injector
func New() Injector {
	return &injector{
		values: make(map[reflect.Type]reflect.Value),
	}
}

// Invoke attempts to call the interface{} provided as a function,
// providing dependencies for function arguments based on Type.
// Returns a slice of reflect.Value representing the returned values of the function.
// Returns an error if the injection fails.
// It panics if f is not a function
func (inj *injector) Invoke(f interface{}) ([]reflect.Value, error) {
	t := reflect.TypeOf(f)

	// NumIn返回一个函数的变量的数量
	// 所以这个有点类似php里的user_func_array之类的
	var in = make([]reflect.Value, t.NumIn()) //Panic if t is not kind of Func
	for i := 0; i < t.NumIn(); i++ {
		// 变量类型
		argType := t.In(i)
		// 变量值
		val := inj.Get(argType)
		if !val.IsValid() {
			return nil, fmt.Errorf("Value not found for type %v", argType)
		}

		in[i] = val
	}

	// 类似call_user_func_arr
	return reflect.ValueOf(f).Call(in), nil
}

// Maps dependencies in the Type map to each field in the struct
// that is tagged with 'inject'.
// Returns an error if the injection fails.
// 下面是摘自别人的说明
// Apply方法是用于对struct的字段进行注入
// 参数为指向底层类型为结构体的指针。
// 可注入的前提是：字段必须是导出的(也即字段名以大写字母开头)，并且此字段的tag设置为`inject`。
/*
//代码也是别人写的233
package main

import (
	"fmt"
	"github.com/codegangsta/inject"
)

type SpecialString interface{}
type TestStruct struct {
	Name   string `inject`
	Nick   []byte
	Gender SpecialString `inject`
	uid    int           `inject`
	Age    int           `inject`
}

func main() {
	s := TestStruct{}
	inj := inject.New()
	inj.Map("Xargin")
	inj.MapTo("男", (*SpecialString)(nil))
	inj2 := inject.New()
	inj2.Map(20)
	inj.SetParent(inj2)
	inj.Apply(&s)
	fmt.Println("s.Name =", s.Name)
	fmt.Println("s.Gender =", s.Gender)
	fmt.Println("s.Age =", s.Age)
}
*/
func (inj *injector) Apply(val interface{}) error {
	v := reflect.ValueOf(val)

	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil // Should not panic here ?
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		structField := t.Field(i)
		if f.CanSet() && (structField.Tag == "inject" || structField.Tag.Get("inject") != "") {
			ft := f.Type()
			v := inj.Get(ft)
			if !v.IsValid() {
				return fmt.Errorf("Value not found for type %v", ft)
			}

			f.Set(v)
		}

	}

	return nil
}

// Maps the concrete value of val to its dynamic type using reflect.TypeOf,
// It returns the TypeMapper registered in.
// 比如传i int = 10
// 那么key = int
// value = 10
func (i *injector) Map(val interface{}) TypeMapper {
	i.values[reflect.TypeOf(val)] = reflect.ValueOf(val)
	return i
}

// 需要将某种类型map到特殊的key上
// 比如i.MapTo(100, (*MyInt)(nil))
// key = main.MyInt
// value = 100
func (i *injector) MapTo(val interface{}, ifacePtr interface{}) TypeMapper {
	i.values[InterfaceOf(ifacePtr)] = reflect.ValueOf(val)
	return i
}

// Maps the given reflect.Type to the given reflect.Value and returns
// the Typemapper the mapping has been registered in.
// 普通的set和get方法
func (i *injector) Set(typ reflect.Type, val reflect.Value) TypeMapper {
	i.values[typ] = val
	return i
}

func (i *injector) Get(t reflect.Type) reflect.Value {
	val := i.values[t]

	if val.IsValid() {
		return val
	}

	// no concrete types found, try to find implementors
	// if t is an interface
	// 如果固定类型没有找到
	// 那么如果某个类型实现了一个接口
	// 那么用这个接口名字去找变量
	// 感觉好trick
	if t.Kind() == reflect.Interface {
		for k, v := range i.values {
			if k.Implements(t) {
				val = v
				break
			}
		}
	}

	// Still no type found, try to look it up on the parent
	// 如果还是没找到，去父injector里去找
	if !val.IsValid() && i.parent != nil {
		val = i.parent.Get(t)
	}

	return val

}

// 设置父injector
func (i *injector) SetParent(parent Injector) {
	i.parent = parent
}
