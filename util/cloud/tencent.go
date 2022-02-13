package cloud

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/donkeywon/golib/util/httpc"
)

func IsTencent() bool {
	data, err := GetTencentCvmBandwidthIngress()
	if data != nil && data.Len() > 0 && err == nil {
		return true
	}
	return false
}

func GetTencentCvmInstanceType() (*bytes.Buffer, error) {
	return getTencentMetadata("http://metadata.tencentyun.com/latest/meta-data/instance/instance-type")
}

func GetTencentCvmBandwidthEgress() (*bytes.Buffer, error) {
	return getTencentMetadata("http://metadata.tencentyun.com/latest/meta-data/instance/bandwidth-limit-egress")
}

func GetTencentCvmBandwidthIngress() (*bytes.Buffer, error) {
	return getTencentMetadata("http://metadata.tencentyun.com/latest/meta-data/instance/bandwidth-limit-ingress")
}

func GetTencentCvmNetworkSpeed() (int, error) {
	egress, err := GetTencentCvmBandwidthEgress()
	if err != nil {
		return 0, err
	}
	speed, err := strconv.Atoi(egress.String())
	if err != nil {
		return 0, err
	}
	speed /= 1024
	return speed, nil
}

func getTencentMetadata(url string) (*bytes.Buffer, error) {
	resp := bytes.NewBuffer(nil)
	_, err := httpc.Get(nil, cloudMetadataReqTimeout, url,
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToBytesBuffer(resp),
	)
	return resp, err
}
