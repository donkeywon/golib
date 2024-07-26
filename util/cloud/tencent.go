package cloud

import (
	"strconv"
	"time"

	"github.com/donkeywon/golib/util/httpc"
)

func IsTencent() bool {
	data, err := GetTencentCvmBandwidthIngress()
	if len(data) > 0 && err == nil {
		return true
	}
	return false
}

func GetTencentCvmInstanceType() ([]byte, error) {
	body, _, err := httpc.Gtimeout(time.Second, "http://metadata.tencentyun.com/latest/meta-data/instance/instance-type")
	return body, err
}

func GetTencentCvmBandwidthEgress() ([]byte, error) {
	body, _, err := httpc.Gtimeout(time.Second, "http://metadata.tencentyun.com/latest/meta-data/instance/bandwidth-limit-egress")
	return body, err
}

func GetTencentCvmBandwidthIngress() ([]byte, error) {
	body, _, err := httpc.Gtimeout(time.Second, "http://metadata.tencentyun.com/latest/meta-data/instance/bandwidth-limit-ingress")
	return body, err
}

func GetTencentCvmNetworkSpeed() (int, error) {
	egress, err := GetTencentCvmBandwidthEgress()
	if err != nil {
		return 0, err
	}
	speed, err := strconv.Atoi(string(egress))
	if err != nil {
		return 0, err
	}
	speed /= 1024
	return speed, nil
}
