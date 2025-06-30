package commit

import (
	"bytes"
	"fmt"
	"text/template"
)

func generateCommitMessage(commitSubjectTemplate string, metadata hydratorMetadataFile) (string, error) {
	subjectTemplate, err := template.New("subject").Funcs(sprigFuncMap).Parse(commitSubjectTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	err = subjectTemplate.Execute(&subjectBuf, metadata)
	if err != nil {
		return "", fmt.Errorf("failed to execute subject template: %w", err)
	}
	return subjectBuf.String(), nil
}
