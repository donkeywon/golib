package reflects

import (
	"reflect"
	"runtime"
)

func GetFuncName(v any) string {
	if v == nil {
		return ""
	}
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Func {
		panic("not func")
	}
	return runtime.FuncForPC(vv.Pointer()).Name()
}

func IsStructPointer(v any) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		return false
	}

	return rv.Elem().Kind() == reflect.Struct
}

func IsPointer(v any) bool {
	return reflect.ValueOf(v).Kind() == reflect.Pointer
}

func IsStruct(v any) bool {
	return reflect.ValueOf(v).Kind() == reflect.Struct
}

func IsFunc(v any) bool {
	return reflect.ValueOf(v).Kind() == reflect.Func
}
