// 工具包 - 深度拷贝
package util

// reference: https://github.com/mohae/deepcopy
import (
	"reflect"
)

// 通过反射深度拷贝反射值
// 引用类型需要递归调用
func deepCopy(dst, src reflect.Value) {
	switch src.Kind() {
	// 字段为接口类型
	case reflect.Interface:
		value := src.Elem()
		if !value.IsValid() {
			return
		}
		newValue := reflect.New(value.Type()).Elem()
		deepCopy(newValue, value)
		dst.Set(newValue)
	// 字段为指针类型
	case reflect.Ptr:
		value := src.Elem()
		if !value.IsValid() {
			return
		}
		dst.Set(reflect.New(value.Type()))
		deepCopy(dst.Elem(), value)
	// 字段为map类型
	case reflect.Map:
		dst.Set(reflect.MakeMap(src.Type()))
		keys := src.MapKeys()
		for _, key := range keys {
			value := src.MapIndex(key)
			newValue := reflect.New(value.Type()).Elem()
			deepCopy(newValue, value)
			dst.SetMapIndex(key, newValue)
		}
	// 字段为切片类型
	case reflect.Slice:
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Cap()))
		for i := 0; i < src.Len(); i++ {
			deepCopy(dst.Index(i), src.Index(i))
		}
	// 字段为接口体类型
	case reflect.Struct:
		typeSrc := src.Type()
		for i := 0; i < src.NumField(); i++ {
			value := src.Field(i)
			tag := typeSrc.Field(i).Tag
			if value.CanSet() && tag.Get("deepcopy") != "-" {
				deepCopy(dst.Field(i), value)
			}
		}
	// 值类型直接进行设置
	default:
		dst.Set(src)
	}
}

// 深度拷贝 - 目标和源变量都必须传地址
func DeepCopy(dst, src interface{}) {
	typeDst := reflect.TypeOf(dst)
	typeSrc := reflect.TypeOf(src)
	if typeDst != typeSrc {
		panic("DeepCopy: " + typeDst.String() + " != " + typeSrc.String())
	}
	if typeSrc.Kind() != reflect.Ptr {
		panic("DeepCopy: pass arguments by address")
	}

	valueDst := reflect.ValueOf(dst).Elem()
	valueSrc := reflect.ValueOf(src).Elem()
	if !valueDst.IsValid() || !valueSrc.IsValid() {
		panic("DeepCopy: invalid arguments")
	}

	deepCopy(valueDst, valueSrc)
}

// 深度克隆 - 通过深度拷贝实现
func DeepClone(v interface{}) interface{} {
	dst := reflect.New(reflect.TypeOf(v)).Elem()
	deepCopy(dst, reflect.ValueOf(v))
	return dst.Interface()
}
