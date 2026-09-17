//go:build windows

package machine_id

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

var _ = fmt.Print

func read_machine_id() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer k.Close()
	ans, _, err := k.GetStringValue("MachineGuid")
	return ans, err
}
