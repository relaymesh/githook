package githook

import (
	"encoding/json"
	"fmt"
)

func printJSON(value interface{}) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}
