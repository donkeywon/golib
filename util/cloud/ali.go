package cloud

import (
	"bytes"
	"strconv"

	"github.com/donkeywon/golib/util/httpc"
)

func IsAliyun() bool {
	metadata, err := getAliEcsInstanceMetadata("")
	if metadata != nil && metadata.Len() > 0 && err == nil {
		return true
	}
	return false
}

func GetAliEcsMetadataToken() (*bytes.Buffer, error) {
	respBody := bytes.NewBuffer(nil)

	_, err := httpc.Put(nil, cloudMetadataReqTimeout, "http://100.100.100.200/latest/api/token",
		httpc.WithHeaders("X-aliyun-ecs-metadata-token-ttl-seconds", "30"),
		httpc.ToBytesBuffer(respBody))

	return respBody, err
}

func GetAliEcsInstanceType() (*bytes.Buffer, error) {
	return getAliEcsInstanceMetadata("instance-type")
}

func GetAliEcsNetbwEgress() (*bytes.Buffer, error) {
	return getAliEcsInstanceMetadata("max-netbw-egress")
}

func GetAliEcsNetbwIngress() (*bytes.Buffer, error) {
	return getAliEcsInstanceMetadata("max-netbw-ingress")
}

func GetAliEcsNetworkSpeed() (int, error) {
	egress, err := GetAliEcsNetbwEgress()
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

func getAliEcsInstanceMetadata(typ string) (*bytes.Buffer, error) {
	token, err := GetAliEcsMetadataToken()
	if err != nil {
		return nil, err
	}

	respBody := bytes.NewBuffer(nil)
	_, err = httpc.Get(nil, cloudMetadataReqTimeout, "http://100.100.100.200/latest/meta-data/instance/"+typ,
		httpc.WithHeaders("X-aliyun-ecs-metadata-token", token.String()),
		httpc.ToBytesBuffer(respBody))

	return respBody, err
}
