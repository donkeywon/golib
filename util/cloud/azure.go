package cloud

import "github.com/donkeywon/golib/util/httpc"

func IsAzure() bool {
	metadata, err := GetAzureVMInstanceMetadata()
	if len(metadata) > 0 && err == nil {
		return true
	}
	return false
}

func GetAzureVMInstanceMetadata() ([]byte, error) {
	body, _, err := httpc.G("http://169.254.169.254/metadata/instance?api-version=2021-02-01", "Metadata", "true")
	return body, err
}
