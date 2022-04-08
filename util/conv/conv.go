package conv

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/donkeywon/golib/util/jsons"
)

func ToString(v any) (string, error) {
	var (
		vs  string
		err error
	)

	switch vv := v.(type) {
	case string:
		vs = vv
	case *string:
		vs = *vv
	case []byte:
		vs = string(vv)
	case bool:
		vs = strconv.FormatBool(vv)
	case *bool:
		vs = strconv.FormatBool(*vv)
	case complex128:
		vs = strconv.FormatComplex(vv, 'f', 10, 128)
	case *complex128:
		vs = strconv.FormatComplex(*vv, 'f', 10, 128)
	case complex64:
		vs = strconv.FormatComplex(complex128(vv), 'f', 10, 128)
	case *complex64:
		vs = strconv.FormatComplex(complex128(*vv), 'f', 10, 128)
	case float64:
		vs = strconv.FormatFloat(vv, 'f', 10, 64)
	case *float64:
		vs = strconv.FormatFloat(*vv, 'f', 10, 64)
	case float32:
		vs = strconv.FormatFloat(float64(vv), 'f', 10, 64)
	case *float32:
		vs = strconv.FormatFloat(float64(*vv), 'f', 10, 64)
	case int:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int64:
		vs = strconv.FormatInt(vv, 10)
	case *int64:
		vs = strconv.FormatInt(*vv, 10)
	case int32:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int32:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int16:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int16:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int8:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int8:
		vs = strconv.FormatInt(int64(*vv), 10)
	case uint:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint64:
		vs = strconv.FormatUint(vv, 10)
	case *uint64:
		vs = strconv.FormatUint(*vv, 10)
	case uint32:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint32:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint16:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint16:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint8:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint8:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uintptr:
		vs = strconv.FormatInt(int64(vv), 16)
	case *uintptr:
		vs = strconv.FormatInt(int64(*vv), 16)
	case time.Time:
		vs = vv.String()
	case *time.Time:
		vs = vv.String()
	case time.Duration:
		vs = vv.String()
	case *time.Duration:
		vs = vv.String()
	default:
		vs, err = jsons.MarshalString(vv)
	}

	return vs, err
}

func MustToString(any interface{}) string {
	v, err := ToString(any)
	if err != nil {
		panic(err)
	}
	return v
}

func ToBool(v any) (bool, error) {
	var (
		vv  bool
		err error
	)
	switch vt := v.(type) {
	case string:
		vv = vt == "true"
	case *string:
		vv = *vt == "true"
	case bool:
		vv = vt
	case *bool:
		vv = *vt
	default:
		err = fmt.Errorf("unexpected value type, expected: string or bool, actual: %s", reflect.TypeOf(v))
	}
	return vv, err
}

func MustToBool(v any) bool {
	vv, err := ToBool(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func ToInt(v any) (int, error) {
	var (
		vv  int
		err error
	)

	switch vt := v.(type) {
	case string:
		vv, err = strconv.Atoi(vt)
	case *string:
		vv, err = strconv.Atoi(*vt)
	case int8:
		vv = int(vt)
	case *int8:
		vv = int(*vt)
	case int16:
		vv = int(vt)
	case *int16:
		vv = int(*vt)
	case int32:
		vv = int(vt)
	case *int32:
		vv = int(*vt)
	case int64:
		vv = int(vt)
	case *int64:
		vv = int(*vt)
	case int:
		vv = vt
	case *int:
		vv = *vt
	default:
		err = fmt.Errorf("unexpected value type, expected: string or any integer type, actual: %s", reflect.TypeOf(v))
	}
	return vv, err
}

func MustToInt(v any) int {
	vv, err := ToInt(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func ToUint(v any) (uint, error) {
	var (
		vv  uint
		err error
	)
	switch vt := v.(type) {
	case string:
		vvv, er := strconv.ParseUint(vt, 10, 0)
		vv = uint(vvv)
		err = er
	case *string:
		vvv, er := strconv.ParseUint(*vt, 10, 0)
		vv = uint(vvv)
		err = er
	case uint8:
		vv = uint(vt)
	case *uint8:
		vv = uint(*vt)
	case uint16:
		vv = uint(vt)
	case *uint16:
		vv = uint(*vt)
	case uint32:
		vv = uint(vt)
	case *uint32:
		vv = uint(*vt)
	case uint64:
		vv = uint(vt)
	case *uint64:
		vv = uint(*vt)
	case uint:
		vv = vt
	case *uint:
		vv = *vt
	default:
		err = fmt.Errorf("unexpected value type, expected: string or any unsigned integer type, actual: %s", reflect.TypeOf(v))
	}
	return vv, err
}

func MustToUint(v any) uint {
	vv, err := ToUint(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func ToFloat(v any) (float64, error) {
	var (
		vv  float64
		err error
	)
	switch vt := v.(type) {
	case string:
		vv, err = strconv.ParseFloat(vt, 64)
	case *string:
		vv, err = strconv.ParseFloat(*vt, 64)
	case float32:
		vv = float64(vt)
	case *float32:
		vv = float64(*vt)
	case float64:
		vv = vt
	case *float64:
		vv = *vt
	default:
		err = fmt.Errorf("unexpected value type, expected: string or any float type, actual: %s", reflect.TypeOf(v))
	}
	return vv, err
}

func MustToFloat(v any) float64 {
	vv, err := ToFloat(v)
	if err != nil {
		panic(err)
	}
	return vv
}
