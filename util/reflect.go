package util

import (
	"reflect"
	"runtime"
)

func ReflectSet(i interface{}, f interface{}) bool {
	iValue := reflect.ValueOf(i)
	if iValue.Kind() != reflect.Pointer {
		return false
	}

	fType := reflect.TypeOf(f)
	if fType.Kind() != reflect.Pointer {
		return false
	}

	found := false
	idx := 0
	for ; idx < iValue.Elem().NumField(); idx++ {
		f := iValue.Elem().Field(idx)
		if f.CanSet() && f.Type() == fType {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	iValue.Elem().Field(idx).Set(reflect.ValueOf(f))
	return true
}

func ReflectGetFuncName(v interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()
}

func IsStructPointer(v interface{}) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		return false
	}

	return rv.Elem().Kind() == reflect.Struct
}

func IsPointer(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Pointer
}

func IsStruct(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Struct
}

func IsFunc(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Func
}
