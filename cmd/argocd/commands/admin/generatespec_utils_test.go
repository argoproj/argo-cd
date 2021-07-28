package admin

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetOutWriter_InlineOff(t *testing.T) {
	out, closer, err := getOutWriter(false, "")
	require.NoError(t, err)
	defer io.Close(closer)

	assert.Equal(t, os.Stdout, out)
}

func TestGetOutWriter_InlineOn(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpFile.Name())
		_ = os.Remove(fmt.Sprintf("%s.back", tmpFile.Name()))
	}()

	out, closer, err := getOutWriter(true, tmpFile.Name())
	require.NoError(t, err)
	defer io.Close(closer)

	assert.Equal(t, tmpFile.Name(), out.(*os.File).Name())
	_, err = os.Stat(fmt.Sprintf("%s.back", tmpFile.Name()))
	assert.NoError(t, err, "Back file must be created")
}

func TestPrintResources_Secret_YAML(t *testing.T) {
	out := bytes.Buffer{}
	err := PrintResources("yaml", &out, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret"},
		Data:       map[string][]byte{"my-secret-key": []byte("my-secret-data")},
	})
	assert.NoError(t, err)

	assert.Equal(t, `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
stringData:
  my-secret-key: my-secret-data
`, out.String())
}
