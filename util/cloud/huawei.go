package cloud

import (
	"bytes"
	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/jsons"
	"net/http"
)

type HuaweiEcsNetworkQosData struct {
	InstanceMinBandwidth int `json:"instance_min_bandwidth"`
	InstanceMaxBandwidth int `json:"instance_max_bandwidth"`
}

type HuaweiEcsNetworkData struct {
	Qos *HuaweiEcsNetworkQosData `json:"qos"`
}

func IsHuawei() bool {
	metadata, err := GetHuaweiEcsMetadata()
	if metadata != nil && metadata.Len() > 0 && err == nil {
		return true
	}
	return false
}

func GetHuaweiEcsMetadata() (*bytes.Buffer, error) {
	resp := bytes.NewBuffer(nil)
	_, err := httpc.Get(nil, cloudMetadataReqTimeout, "http://169.254.169.254/openstack/latest/meta_data.json",
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToBytesBuffer(resp),
	)
	return resp, err
}

func GetHuaweiEcsNetworkData() (*HuaweiEcsNetworkData, error) {
	networkData := &HuaweiEcsNetworkData{}
	_, err := httpc.Get(nil, cloudMetadataReqTimeout, "http://169.254.169.254/openstack/latest/network_data.json",
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToAnyUnmarshal(networkData, jsons.Unmarshal),
	)
	return networkData, err
}

func GetHuaweiEcsNetworkSpeed() (int, error) {
	networkData, err := GetHuaweiEcsNetworkData()
	if err != nil {
		return 0, err
	}

	return networkData.Qos.InstanceMinBandwidth, nil
}
