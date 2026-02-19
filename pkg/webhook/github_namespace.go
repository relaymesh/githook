package webhook

import (
	"encoding/json"
	"strconv"
	"strings"
)

func githubNamespaceInfo(raw []byte) (string, string) {
	var payload struct {
		Repository struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			Name     string `json:"name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	namespaceID := ""
	if payload.Repository.ID > 0 {
		namespaceID = strconv.FormatInt(payload.Repository.ID, 10)
	}
	namespaceName := strings.TrimSpace(payload.Repository.FullName)
	if namespaceName == "" && payload.Repository.Owner.Login != "" && payload.Repository.Name != "" {
		namespaceName = payload.Repository.Owner.Login + "/" + payload.Repository.Name
	}
	return namespaceID, namespaceName
}
