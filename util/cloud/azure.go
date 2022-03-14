package cloud

import (
	"bytes"

	"github.com/donkeywon/golib/util/httpc"
)

func IsAzure() bool {
	metadata, err := GetAzureVMInstanceMetadata()
	if len(metadata) > 0 && err == nil {
		return true
	}
	return false
}

func GetAzureVMInstanceMetadata() ([]byte, error) {
	resp := bytes.NewBuffer(nil)
	_, err := httpc.Get(nil, cloudMetadataReqTimeout, "http://169.254.169.254/metadata/instance?api-version=2021-02-01",
		httpc.WithHeaders("Metadata", "true"), httpc.ToBytesBuffer(resp))

	return resp.Bytes(), err
}
