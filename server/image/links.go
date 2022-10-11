package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	imagepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/image"
	"os"
	"text/template"
)

func links(image string) (map[string]*imagepkg.Link, error) {
	out := map[string]*imagepkg.Link{}
	text := os.Getenv("IMAGE_LINKS")
	if text != "" {
		tmpl, err := template.New("links").Parse(text)
		if err != nil {
			return nil, fmt.Errorf("failed to parse links: %w", err)
		}
		buf := &bytes.Buffer{}
		if err := tmpl.Execute(buf, map[string]any{
			"Image": image,
		}); err != nil {
			return nil, fmt.Errorf("failed to execute template: %w", err)
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			return nil, fmt.Errorf("failed to unmarshall links: %w", err)
		}
	}
	return out, nil
}
