package conv

import (
	"strconv"
	"time"

	"github.com/donkeywon/golib/util/jsonu"
)

func AnyToString(v any) string {
	var vs string

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
		vs, _ = jsonu.MarshalString(vv)
	}

	return vs
}
