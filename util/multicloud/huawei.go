package multicloud

import (
	"github.com/bytedance/sonic"
	"github.com/donkeywon/golib/util/httpc"
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
	if len(metadata) > 0 && err == nil {
		return true
	}
	return false
}

func GetHuaweiEcsMetadata() ([]byte, error) {
	body, _, err := httpc.G("http://169.254.169.254/openstack/latest/meta_data.json")
	return body, err
}

func GetHuaweiEcsNetworkData() (*HuaweiEcsNetworkData, error) {
	data, _, err := httpc.G("http://169.254.169.254/openstack/latest/network_data.json")
	if err != nil {
		return nil, err
	}

	networkData := &HuaweiEcsNetworkData{}
	err = sonic.Unmarshal(data, networkData)
	if err != nil {
		return nil, err
	}
	return networkData, nil
}

func GetHuaweiEcsNetworkSpeed() (int, error) {
	networkData, err := GetHuaweiEcsNetworkData()
	if err != nil {
		return 0, err
	}

	return networkData.Qos.InstanceMinBandwidth, nil
}
