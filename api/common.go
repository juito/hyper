package api

import (
	"fmt"
	"mime"
	"strings"
)

const (
	APIVERSION = "1.17"
)

func FormGroup(key string, start, last int) string {
	var (
		group     string
		parts     = strings.Split(key, "/")
		groupType = parts[0]
		ip        = ""
	)
	if len(parts) > 1 {
		ip = parts[0]
		groupType = parts[1]
	}
	if start == last {
		group = fmt.Sprintf("%d", start)
	} else {
		group = fmt.Sprintf("%d-%d", start, last)
	}
	if ip != "" {
		group = fmt.Sprintf("%s:%s->%s", ip, group, group)
	}
	return fmt.Sprintf("%s/%s", group, groupType)
}

func MatchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		fmt.Printf("Error parsing media type: %s error: %v", contentType, err)
	}
	return err == nil && mimetype == expectedType
}
