package cloud

import (
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/jsonu"
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
	body, _, err := httpc.Gtimeout(time.Second, "http://169.254.169.254/openstack/latest/meta_data.json")
	return body, err
}

func GetHuaweiEcsNetworkData() (*HuaweiEcsNetworkData, error) {
	data, _, err := httpc.Gtimeout(time.Second, "http://169.254.169.254/openstack/latest/network_data.json")
	if err != nil {
		return nil, err
	}

	networkData := &HuaweiEcsNetworkData{}
	err = jsonu.Unmarshal(data, networkData)
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
