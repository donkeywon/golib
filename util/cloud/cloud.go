package cloud

import (
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/eth"
)

var cloudMetadataReqTimeout = 10 * time.Second

type Type string

const (
	TypeUnknown Type = "unknown"
	TypeAws     Type = "aws"
	TypeAzure   Type = "azure"
	TypeAliyun  Type = "aliyun"
	TypeHuawei  Type = "huawei"
	TypeTencent Type = "tencent"
)

func GetNicSpeed() (int, error) {
	typ := Which()
	var (
		speed int
		err   error
	)
	switch typ {
	case TypeAws:
		speed, err = GetAwsEc2NetSpeed()
	case TypeHuawei:
		speed, err = GetHuaweiEcsNetworkSpeed()
	case TypeAliyun:
		speed, err = GetAliEcsNetworkSpeed()
	case TypeTencent:
		speed, err = GetTencentCvmNetworkSpeed()
	case TypeAzure:
		speed, err = eth.GetNicSpeed("eth0")
	default:
		speed, err = 0, errs.Errorf("unknown cloud type: %s", typ)
	}
	return speed, err
}

func Which() Type {
	switch {
	case IsHuawei():
		return TypeHuawei
	case IsAws():
		return TypeAws
	case IsAliyun():
		return TypeAliyun
	case IsAzure():
		return TypeAzure
	case IsTencent():
		return TypeTencent
	default:
		return TypeUnknown
	}
}
