package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	clientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1/mocks"
)

func Test_deepCopyAppProjectClient_Get(t *testing.T) {
	type fields struct {
		AppProjectInterface clientset.AppProjectInterface
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.AppProject
		wantErr assert.ErrorAssertionFunc
	}{
		{name: "Get an app project", fields: fields{AppProjectInterface: setupAppProjects("appproject")}, args: args{
			name: "appproject",
		}, want: &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "appproject", Namespace: "deep-copy-ns"},
		}, wantErr: assert.NoError},
		{
			name: "Error getting an app project",
			fields: fields{
				AppProjectInterface: func() clientset.AppProjectInterface {
					appProject := mocks.AppProjectInterface{}
					appProject.On("Get", t.Context(), "appproject2", metav1.GetOptions{}).Return(nil, errors.New("error"))
					return &appProject
				}(),
			},
			args: args{
				name: "appproject2",
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyAppProjectClient{
				AppProjectInterface: tt.fields.AppProjectInterface,
			}
			got, err := d.Get(t.Context(), tt.args.name, metav1.GetOptions{})
			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v)", tt.args.name)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Get(%v)", tt.args.name)
			if tt.want != nil {
				assert.NotSamef(t, tt.want, got, "%v and %v are the same ptr", tt.want, got)
			}
		})
	}
}

func Test_deepCopyAppProjectClient_List(t *testing.T) {
	type fields struct {
		AppProjectInterface clientset.AppProjectInterface
	}
	tests := []struct {
		name    string
		fields  fields
		want    []v1alpha1.AppProject
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "List app projects", fields: fields{AppProjectInterface: setupAppProjects("proj1", "proj2")},
			want: createAppProject("proj1", "proj2"), wantErr: assert.NoError,
		},
		{name: "Error listing app project", fields: fields{
			AppProjectInterface: func() clientset.AppProjectInterface {
				appProject := mocks.AppProjectInterface{}
				appProject.On("List", t.Context(), metav1.ListOptions{}).Return(nil, errors.New("error"))
				return &appProject
			}(),
		}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyAppProjectClient{
				AppProjectInterface: tt.fields.AppProjectInterface,
			}
			got, err := d.List(t.Context(), metav1.ListOptions{})
			if !tt.wantErr(t, err, "List") {
				return
			}
			if tt.want != nil {
				assert.Equalf(t, tt.want, got.Items, "List")
				for i := range tt.want {
					assert.NotSamef(t, &tt.want[i], &got.Items[i], "%v and %v are the same ptr", tt.want, got)
				}
			}
		})
	}
}

func createAppProject(projects ...string) []v1alpha1.AppProject {
	appProjects := make([]v1alpha1.AppProject, len(projects))
	for i, p := range projects {
		appProjects[i] = v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: "deep-copy-ns"}}
	}
	return appProjects
}

func setupAppProjects(projects ...string) clientset.AppProjectInterface {
	appProjects := createAppProject(projects...)
	ro := make([]runtime.Object, len(appProjects))
	for i := range appProjects {
		ro[i] = &appProjects[i]
	}
	return fake.NewSimpleClientset(ro...).ArgoprojV1alpha1().AppProjects("deep-copy-ns")
}

func Test_deepCopyArgoprojV1alpha1Client_RESTClient(t *testing.T) {
	fclientset := fake.NewSimpleClientset().ArgoprojV1alpha1()
	type fields struct {
		ArgoprojV1alpha1Interface clientset.ArgoprojV1alpha1Interface
	}
	tests := []struct {
		name   string
		fields fields
		want   rest.Interface
	}{
		{name: "RestClientGetter", fields: fields{ArgoprojV1alpha1Interface: fclientset}, want: fclientset.RESTClient()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyArgoprojV1alpha1Client{
				ArgoprojV1alpha1Interface: tt.fields.ArgoprojV1alpha1Interface,
			}
			assert.Equalf(t, tt.want, d.RESTClient(), "RESTClient()")
		})
	}
}
