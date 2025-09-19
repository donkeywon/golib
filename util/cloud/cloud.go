package cloud

import (
	"time"

	"github.com/donkeywon/golib/errs"
)

var (
	cloudMetadataReqTimeout = time.Second

	speedGetter = map[Type]func() (int, error){
		TypeAws:     GetAwsEc2NetSpeed,
		TypeHuawei:  GetHuaweiEcsNetworkSpeed,
		TypeAliyun:  GetAliEcsNetworkSpeed,
		TypeTencent: GetTencentCvmNetworkSpeed,
		TypeAzure:   GetAzureVMNicSpeed,
	}

	Which = func() Type {
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
)

type Type string

const (
	TypeUnknown Type = "unknown"
	TypeAws     Type = "aws"
	TypeAzure   Type = "azure"
	TypeAliyun  Type = "aliyun"
	TypeHuawei  Type = "huawei"
	TypeTencent Type = "tencent"
)

func RegCloudNicSpeedGetter(typ Type, getter func() (int, error)) {
	speedGetter[typ] = getter
}

func GetNicSpeed() (int, error) {
	typ := Which()
	getter, ok := speedGetter[typ]
	if !ok {
		return 0, errs.Errorf("unknown cloud type: %s", typ)
	}
	return getter()
}
