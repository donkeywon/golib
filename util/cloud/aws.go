package cloud

import (
	"bytes"
	"net/http"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/httpc"
)

var (
	awsInstanceTypeNetworkSpeedMap = map[string]int{
		"a1.medium":        512,
		"a1.large":         768,
		"a1.xlarge":        1280,
		"a1.2xlarge":       2560,
		"a1.4xlarge":       5120,
		"a1.metal":         5120,
		"m4.10xlarge":      10240,
		"m4.16xlarge":      25600,
		"m5.large":         768,
		"m5.xlarge":        1280,
		"m5.2xlarge":       2560,
		"m5.4xlarge":       5120,
		"m5.8xlarge":       10240,
		"m5.12xlarge":      12288,
		"m5.16xlarge":      20480,
		"m5.24xlarge":      25600,
		"m5.metal":         25600,
		"m5a.large":        768,
		"m5a.xlarge":       1280,
		"m5a.2xlarge":      2560,
		"m5a.4xlarge":      5120,
		"m5a.8xlarge":      7680,
		"m5a.12xlarge":     10240,
		"m5a.16xlarge":     12288,
		"m5a.24xlarge":     20480,
		"m5ad.large":       768,
		"m5ad.xlarge":      1280,
		"m5ad.2xlarge":     2560,
		"m5ad.4xlarge":     5120,
		"m5ad.8xlarge":     7680,
		"m5ad.12xlarge":    10240,
		"m5ad.16xlarge":    12288,
		"m5ad.24xlarge":    20480,
		"m5d.large":        768,
		"m5d.xlarge":       1280,
		"m5d.2xlarge":      2560,
		"m5d.4xlarge":      5120,
		"m5d.8xlarge":      10240,
		"m5d.12xlarge":     12288,
		"m5d.16xlarge":     20480,
		"m5d.24xlarge":     25600,
		"m5d.metal":        25600,
		"m5dn.large":       2150,
		"m5dn.xlarge":      4198,
		"m5dn.2xlarge":     8320,
		"m5dn.4xlarge":     16640,
		"m5dn.8xlarge":     25600,
		"m5dn.12xlarge":    51200,
		"m5dn.16xlarge":    76800,
		"m5dn.24xlarge":    102400,
		"m5dn.metal":       102400,
		"m5n.large":        2150,
		"m5n.xlarge":       4198,
		"m5n.2xlarge":      8320,
		"m5n.4xlarge":      16640,
		"m5n.8xlarge":      25600,
		"m5n.12xlarge":     51200,
		"m5n.16xlarge":     76800,
		"m5n.24xlarge":     102400,
		"m5n.metal":        102400,
		"m5zn.large":       3072,
		"m5zn.xlarge":      5120,
		"m5zn.2xlarge":     10240,
		"m5zn.3xlarge":     15360,
		"m5zn.6xlarge":     51200,
		"m5zn.12xlarge":    102400,
		"m5zn.metal":       102400,
		"m6a.large":        799,
		"m6a.xlarge":       1599,
		"m6a.2xlarge":      3200,
		"m6a.4xlarge":      6400,
		"m6a.8xlarge":      12800,
		"m6a.12xlarge":     19200,
		"m6a.16xlarge":     25600,
		"m6a.24xlarge":     38400,
		"m6a.32xlarge":     51200,
		"m6a.48xlarge":     51200,
		"m6a.metal":        51200,
		"m6g.medium":       512,
		"m6g.large":        768,
		"m6g.xlarge":       1280,
		"m6g.2xlarge":      2560,
		"m6g.4xlarge":      5120,
		"m6g.8xlarge":      12288,
		"m6g.12xlarge":     20480,
		"m6g.16xlarge":     25600,
		"m6g.metal":        25600,
		"m6gd.medium":      512,
		"m6gd.large":       768,
		"m6gd.xlarge":      1280,
		"m6gd.2xlarge":     2560,
		"m6gd.4xlarge":     5120,
		"m6gd.8xlarge":     12288,
		"m6gd.12xlarge":    20480,
		"m6gd.16xlarge":    25600,
		"m6gd.metal":       25600,
		"m6i.large":        799,
		"m6i.xlarge":       1599,
		"m6i.2xlarge":      3200,
		"m6i.4xlarge":      6400,
		"m6i.8xlarge":      12800,
		"m6i.12xlarge":     19200,
		"m6i.16xlarge":     25600,
		"m6i.24xlarge":     38400,
		"m6i.32xlarge":     51200,
		"m6i.metal":        51200,
		"m6id.large":       799,
		"m6id.xlarge":      1599,
		"m6id.2xlarge":     3200,
		"m6id.4xlarge":     6400,
		"m6id.8xlarge":     12800,
		"m6id.12xlarge":    19200,
		"m6id.16xlarge":    25600,
		"m6id.24xlarge":    38400,
		"m6id.32xlarge":    51200,
		"m6id.metal":       51200,
		"m6idn.large":      3200,
		"m6idn.xlarge":     6400,
		"m6idn.2xlarge":    12800,
		"m6idn.4xlarge":    25600,
		"m6idn.8xlarge":    51200,
		"m6idn.12xlarge":   76800,
		"m6idn.16xlarge":   102400,
		"m6idn.24xlarge":   153600,
		"m6idn.32xlarge":   204800,
		"m6idn.metal":      204800,
		"m6in.large":       3200,
		"m6in.xlarge":      6400,
		"m6in.2xlarge":     12800,
		"m6in.4xlarge":     25600,
		"m6in.8xlarge":     51200,
		"m6in.12xlarge":    76800,
		"m6in.16xlarge":    102400,
		"m6in.24xlarge":    153600,
		"m6in.32xlarge":    204800,
		"m6in.metal":       204800,
		"m7a.medium":       399,
		"m7a.large":        799,
		"m7a.xlarge":       1599,
		"m7a.2xlarge":      3200,
		"m7a.4xlarge":      6400,
		"m7a.8xlarge":      12800,
		"m7a.12xlarge":     19200,
		"m7a.16xlarge":     25600,
		"m7a.24xlarge":     38400,
		"m7a.32xlarge":     51200,
		"m7a.48xlarge":     51200,
		"m7a.metal-48xl":   51200,
		"m7g.medium":       532,
		"m7g.large":        959,
		"m7g.xlarge":       1921,
		"m7g.2xlarge":      3840,
		"m7g.4xlarge":      7680,
		"m7g.8xlarge":      15360,
		"m7g.12xlarge":     23040,
		"m7g.16xlarge":     30720,
		"m7g.metal":        30720,
		"m7gd.medium":      532,
		"m7gd.large":       959,
		"m7gd.xlarge":      1921,
		"m7gd.2xlarge":     3840,
		"m7gd.4xlarge":     7680,
		"m7gd.8xlarge":     15360,
		"m7gd.12xlarge":    23040,
		"m7gd.16xlarge":    30720,
		"m7i.large":        799,
		"m7i.xlarge":       1599,
		"m7i.2xlarge":      3200,
		"m7i.4xlarge":      6400,
		"m7i.8xlarge":      12800,
		"m7i.12xlarge":     19200,
		"m7i.16xlarge":     25600,
		"m7i.24xlarge":     38400,
		"m7i.48xlarge":     51200,
		"m7i.metal-24xl":   38400,
		"m7i.metal-48xl":   51200,
		"m7i-flex.large":   399,
		"m7i-flex.xlarge":  799,
		"m7i-flex.2xlarge": 1599,
		"m7i-flex.4xlarge": 3200,
		"m7i-flex.8xlarge": 6400,
		"mac1.metal":       25600,
		"mac2.metal":       10240,
		"mac2-m2.metal":    10240,
		"mac2-m2pro.metal": 10240,
		"t3.nano":          32,
		"t3.micro":         65,
		"t3.small":         131,
		"t3.medium":        262,
		"t3.large":         524,
		"t3.xlarge":        1048,
		"t3.2xlarge":       2097,
		"t3a.nano":         32,
		"t3a.micro":        65,
		"t3a.small":        131,
		"t3a.medium":       262,
		"t3a.large":        524,
		"t3a.xlarge":       1048,
		"t3a.2xlarge":      2097,
		"t4g.nano":         32,
		"t4g.micro":        65,
		"t4g.small":        131,
		"t4g.medium":       262,
		"t4g.large":        524,
		"t4g.xlarge":       1048,
		"t4g.2xlarge":      2097,
	}
)

func IsAws() bool {
	token, err := GetEC2IMDSv2Token()
	if token.Len() > 0 && err == nil {
		return true
	}
	return false
}

func GetEC2InstanceType() (*bytes.Buffer, error) {
	token, err := GetEC2IMDSv2Token()
	if err != nil {
		return nil, err
	}

	body := bytes.NewBuffer(nil)
	_, err = httpc.Get(nil, cloudMetadataReqTimeout, "http://169.254.169.254/latest/meta-data/instance-type",
		httpc.WithHeaders("X-aws-ec2-metadata-token", token.String()),
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToBytesBuffer(nil, body),
	)
	return body, err
}

func GetAwsEc2NetSpeed() (int, error) {
	instanceType, err := GetEC2InstanceType()
	if err != nil {
		return 0, err
	}

	instanceTypeStr := instanceType.String()
	speed, exists := awsInstanceTypeNetworkSpeedMap[instanceTypeStr]
	if !exists {
		return 0, errs.Errorf("instance net speed not found, type: %s", instanceTypeStr)
	}

	return speed, nil
}

func GetEC2IMDSv2Token() (*bytes.Buffer, error) {
	body := bytes.NewBuffer(nil)
	_, err := httpc.Put(nil, cloudMetadataReqTimeout, "http://169.254.169.254/latest/api/token",
		httpc.WithHeaders("X-aws-ec2-metadata-token-ttl-seconds", "30"),
		httpc.CheckStatusCode(http.StatusOK),
		httpc.ToBytesBuffer(nil, body),
	)
	return body, err
}
