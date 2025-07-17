package generators

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_getDefaultRequeueAfter(t *testing.T) {
	tests := []struct {
		name            string
		requeueAfterEnv string
		want            time.Duration
	}{
		{name: "Default", requeueAfterEnv: "", want: DefaultRequeueAfterSeconds},
		{name: "Min", requeueAfterEnv: "1s", want: 1 * time.Second},
		{name: "Max", requeueAfterEnv: "8760h", want: 8760 * time.Hour},
		{name: "Override", requeueAfterEnv: "10m", want: 10 * time.Minute},
		{name: "LessThanMin", requeueAfterEnv: "1ms", want: DefaultRequeueAfterSeconds},
		{name: "MoreThanMax", requeueAfterEnv: "8761h", want: DefaultRequeueAfterSeconds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ARGOCD_APPLICATIONSET_CONTROLLER_REQUEUE_AFTER", tt.requeueAfterEnv)
			assert.Equalf(t, tt.want, getDefaultRequeueAfter(), "getDefaultRequeueAfter()")
		})
	}
}
