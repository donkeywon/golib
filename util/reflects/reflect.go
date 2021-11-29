package reflects

import (
	"reflect"
	"runtime"
)

func SetFirst(s interface{}, f interface{}) bool {
	sValue := reflect.ValueOf(s)
	if reflect.Indirect(sValue).Kind() != reflect.Struct {
		return false
	}

	fValue := reflect.ValueOf(f)

	found := false
	idx := 0
	for ; idx < sValue.Elem().NumField(); idx++ {
		field := sValue.Elem().Field(idx)
		if field.CanSet() &&
			(field.Kind() == fValue.Kind() || field.Kind() == reflect.Interface && fValue.Type().Implements(field.Type())) {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	sValue.Elem().Field(idx).Set(fValue)
	return true
}

func GetFuncName(v interface{}) string {
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
