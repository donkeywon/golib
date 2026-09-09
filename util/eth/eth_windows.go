package eth

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/donkeywon/golib/errs"
)

// GetNicSpeed
// get nic speed in Mbps.
func GetNicSpeed(ctx context.Context, nic string) (int, error) {
	c := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; Get-NetAdapter | Where-Object { $_.Name -eq "%s" } | ForEach-Object { "$($_.Name)|$($_.LinkSpeed)" }`, nic))

	stdoutBuf := bytes.NewBuffer(nil)
	c.Stdout = stdoutBuf

	err := c.Run()
	if err != nil {
		return 0, errs.Wrap(err, "exec Get-NetAdapter failed")
	}

	stdout := stdoutBuf.String()

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	nicPrefix := nic + "|"
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, nicPrefix) {
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

	if scanner.Err() != nil {
		return 0, errs.Wrap(scanner.Err(), "scan Get-NetAdapter stdout failed")
	}

	return 0, errs.Errorf("nic speed not found in Get-NetAdapter stdout: %s", stdout)
}
