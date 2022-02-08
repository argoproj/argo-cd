package commands

import (
	"bytes"
	"io"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestFindRevisionHistoryWithoutPassedId(t *testing.T) {

	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, -1)

	if err != nil {
		t.Fatal("Find revision history should fail without errors")
	}

	if history == nil {
		t.Fatal("History should be found")
	}

}

func TestFindRevisionHistoryWithoutPassedIdAndEmptyHistoryList(t *testing.T) {

	histories := v1alpha1.RevisionHistories{}

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, -1)

	if err == nil {
		t.Fatal("Find revision history should fail with errors")
	}

	if history != nil {
		t.Fatal("History should be empty")
	}

	if err.Error() != "Application '' should have at least two successful deployments" {
		t.Fatal("Find revision history should fail with correct error message")
	}

}

func TestFindRevisionHistoryWithPassedId(t *testing.T) {

	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3, Revision: "123"})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, 3)

	if err != nil {
		t.Fatal("Find revision history should fail without errors")
	}

	if history == nil {
		t.Fatal("History should be found")
	}

	if history.Revision != "123" {
		t.Fatal("Failed to find correct history with correct revision")
	}

}

// TestYesNoPrompt Test true/false on yes/no and invalid inputs
func TestYesNoPrompt(t *testing.T) {
	testData := []struct {
		input          string
		yesNo, wantErr bool
	}{
		{input: "Y", yesNo: true, wantErr: false},
		{input: "y", yesNo: true, wantErr: false},
		{input: "N", yesNo: false, wantErr: false},
		{input: "N", yesNo: false, wantErr: false},
		{input: "Yes", yesNo: true, wantErr: false},
		{input: "No", yesNo: false, wantErr: false},
		{input: "yes", yesNo: true, wantErr: false},
		{input: "no", yesNo: false, wantErr: false},
		{input: "others", yesNo: false, wantErr: true},
	}
	for _, tt := range testData {
		var r io.Reader = bytes.NewBufferString(tt.input)
		result, err := yesNoPrompt(r, "Confirm to Sync Application")
		if err != nil && !tt.wantErr {
			t.Errorf("YesNoPrompt() error Got = %v, Expectation %v", err, tt.wantErr)
		}
		if tt.yesNo != result {
			t.Errorf("YesNoPrompt() Expected = %v, Got %v", tt.yesNo, result)
		}
	}
}

func TestFindRevisionHistoryWithPassedIdThatNotExist(t *testing.T) {

	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3, Revision: "123"})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, 4)

	if err == nil {
		t.Fatal("Find revision history should fail with errors")
	}

	if history != nil {
		t.Fatal("History should be not found")
	}

	if err.Error() != "Application '' does not have deployment id '4' in history\n" {
		t.Fatal("Find revision history should fail with correct error message")
	}

}
