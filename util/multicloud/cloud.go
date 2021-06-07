package multicloud

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util"
)

const (
	CloudTypeUnknown = "unknown"
	CloudTypeAws     = "aws"
	CloudTypeAzure   = "azure"
	CloudTypeAliyun  = "aliyun"
	CloudTypeHuawei  = "huawei"
	CloudTypeTencent = "tencent"
)

func GetNicSpeed() (int, error) {
	typ := CloudType()
	var (
		speed int
		err   error
	)
	switch typ {
	case CloudTypeAws:
		speed, err = GetAwsEc2NetSpeed()
	case CloudTypeHuawei:
		speed, err = GetHuaweiEcsNetworkSpeed()
	case CloudTypeAliyun:
		speed, err = GetAliEcsNetworkSpeed()
	case CloudTypeTencent:
		speed, err = GetTencentCvmNetworkSpeed()
	case CloudTypeAzure:
		speed, err = util.GetNicSpeed("eth0")
	default:
		speed, err = 0, errs.Errorf("unknown cloud type: %s", typ)
	}
	return speed, err
}

func CloudType() string {
	switch {
	case IsHuawei():
		return CloudTypeHuawei
	case IsAws():
		return CloudTypeAws
	case IsAliyun():
		return CloudTypeAliyun
	case IsAzure():
		return CloudTypeAzure
	default:
		return CloudTypeUnknown
	}
}
