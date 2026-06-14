package eth

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/cmd"
)

// GetNicSpeed
// get nic speed in Mbps.
func GetNicSpeed(nic string) (int, error) {
	result := cmd.Exec(context.Background(), "powershell", "-Command", fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; Get-NetAdapter | Where-Object { $_.Name -eq "%s" } | ForEach-Object { "$($_.Name)|$($_.LinkSpeed)" }`, nic))
	if result.Err() != nil {
		return 0, errs.Wrap(result.Err(), "exec Get-NetAdapter failed")
	}
	for _, line := range result.StdoutLines() {
		if !strings.HasPrefix(line, nic) {
			continue
		}
		if !strings.HasSuffix(line, "bps") {
			continue
		}

		split := strings.SplitN(line, "|", 2)
		speedSplit := strings.SplitN(split[1], " ", 2)
		speed, err := strconv.ParseInt(speedSplit[0], 10, 64)
		if err != nil {
			return 0, errs.Wrapf(err, "parse nic speed failed")
		}
		switch speedSplit[1] {
		case "Gbps":
			return int(speed * 1000), nil
		case "Mbps":
			return int(speed), nil
		case "Kbps":
			if speed < 1000 {
				return 1, nil
			}
			return int(speed / 1000), nil
		case "bps":
			if speed < 1000000 {
				return 1, nil
			}
			return int(speed / 1000000), nil
		default:
			return 0, errs.Errorf("unknown nic speed unit: %s", line)
		}
	}

	return 0, errs.Errorf("nic speed not found in Get-NetAdapter output: %s", result.String())
}
