package cloud

import (
	"strconv"
	"time"

	"github.com/donkeywon/golib/util/httpc"
)

func IsAliyun() bool {
	metadatas, err := getAliEcsInstaneMetadata("")
	if len(metadatas) > 0 && err == nil {
		return true
	}
	return false
}

func GetAliEcsMetadataToken() ([]byte, error) {
	body, _, err := httpc.PuTimeout(
		time.Second,
		"http://100.100.100.200/latest/api/token",
		nil,
		"X-aliyun-ecs-metadata-token-ttl-seconds",
		"30")
	return body, err
}

func GetAliEcsInstanceType() ([]byte, error) {
	return getAliEcsInstaneMetadata("instance-type")
}

func GetAliEcsNetbwEgress() ([]byte, error) {
	return getAliEcsInstaneMetadata("max-netbw-egress")
}

func GetAliEcsNetbwIngress() ([]byte, error) {
	return getAliEcsInstaneMetadata("max-netbw-ingress")
}

func GetAliEcsNetworkSpeed() (int, error) {
	egress, err := GetAliEcsNetbwEgress()
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

func getAliEcsInstaneMetadata(typ string) ([]byte, error) {
	token, err := GetAliEcsMetadataToken()
	if err != nil {
		return nil, err
	}

	body, _, err := httpc.Gtimeout(
		time.Second,
		"http://100.100.100.200/latest/meta-data/instance/"+typ,
		"X-aliyun-ecs-metadata-token",
		string(token))
	return body, err
}
