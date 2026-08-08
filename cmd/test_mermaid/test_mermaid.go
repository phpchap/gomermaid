package main

import (
	"fmt"
	"github.com/phpchap/gomermaid"
)

func main() {
	src := `graph TD
	Start[Request received] --> Auth{Authenticated?}
	Auth -->|yes| Rate{Rate limit OK?}
	Auth -->|no| R401[401 Unauthorized]
	Rate -->|yes| H(Handle request)
	Rate -->|no| R429[429 Too Many Requests]
	H -.-> Log[Audit log]
	H ==> Resp[200 OK]`

	styles := &mermaid.MermaidStyles{}
	art, err := mermaid.Render(src, styles, nil)
	if err != nil {
		panic(err)
	}
	if art == nil {
		fmt.Println("No diagram rendered")
		return
	}

	for _, line := range art.PlainLines {
		fmt.Println(line)
	}
}
