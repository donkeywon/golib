package util

import (
	"reflect"
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
