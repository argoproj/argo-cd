package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	clientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1/mocks"
	lister "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
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
			ObjectMeta: metav1.ObjectMeta{Name: "appproject", Namespace: "default"},
		}, wantErr: assert.NoError},
		{
			name: "Error getting an app project",
			fields: fields{
				AppProjectInterface: func() clientset.AppProjectInterface {
					appProject := mocks.AppProjectInterface{}
					appProject.On("Get", context.Background(), "appproject2", metav1.GetOptions{}).Return(nil, errors.New("error"))
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
			got, err := d.Get(context.Background(), tt.args.name, metav1.GetOptions{})
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
				appProject.On("List", context.Background(), metav1.ListOptions{}).Return(nil, errors.New("error"))
				return &appProject
			}(),
		}, want: nil, wantErr: assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyAppProjectClient{
				AppProjectInterface: tt.fields.AppProjectInterface,
			}
			got, err := d.List(context.Background(), metav1.ListOptions{})
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
		appProjects[i] = v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: "default"}}
	}
	return appProjects
}

func setupAppProjects(projects ...string) clientset.AppProjectInterface {
	appProjects := createAppProject(projects...)
	ro := make([]runtime.Object, len(appProjects))
	for i := range appProjects {
		ro[i] = &appProjects[i]
	}
	return fake.NewSimpleClientset(ro...).ArgoprojV1alpha1().AppProjects("default")
}

func Test_deepCopyApplicationClient_Get(t *testing.T) {
	type fields struct {
		ApplicationInterface clientset.ApplicationInterface
	}
	type args struct {
		ctx     context.Context
		name    string
		options metav1.GetOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.Application
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationClient{
				ApplicationInterface: tt.fields.ApplicationInterface,
			}
			got, err := d.Get(tt.args.ctx, tt.args.name, tt.args.options)
			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v, %v, %v)", tt.args.ctx, tt.args.name, tt.args.options)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Get(%v, %v, %v)", tt.args.ctx, tt.args.name, tt.args.options)
		})
	}
}

func Test_deepCopyApplicationClient_List(t *testing.T) {
	type fields struct {
		ApplicationInterface clientset.ApplicationInterface
	}
	type args struct {
		ctx  context.Context
		opts metav1.ListOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.ApplicationList
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationClient{
				ApplicationInterface: tt.fields.ApplicationInterface,
			}
			got, err := d.List(tt.args.ctx, tt.args.opts)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v, %v)", tt.args.ctx, tt.args.opts)) {
				return
			}
			assert.Equalf(t, tt.want, got, "List(%v, %v)", tt.args.ctx, tt.args.opts)
		})
	}
}

func Test_deepCopyApplicationLister_List(t *testing.T) {
	type fields struct {
		ApplicationLister lister.ApplicationLister
	}
	type args struct {
		selector labels.Selector
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*v1alpha1.Application
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationLister{
				ApplicationLister: tt.fields.ApplicationLister,
			}
			got, err := d.List(tt.args.selector)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v)", tt.args.selector)) {
				return
			}
			assert.Equalf(t, tt.want, got, "List(%v)", tt.args.selector)
		})
	}
}

func Test_deepCopyApplicationNamespaceLister_Get(t *testing.T) {
	type fields struct {
		ApplicationNamespaceLister lister.ApplicationNamespaceLister
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.Application
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationNamespaceLister{
				ApplicationNamespaceLister: tt.fields.ApplicationNamespaceLister,
			}
			got, err := d.Get(tt.args.name)
			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v)", tt.args.name)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Get(%v)", tt.args.name)
		})
	}
}

func Test_deepCopyApplicationNamespaceLister_List(t *testing.T) {
	type fields struct {
		ApplicationNamespaceLister lister.ApplicationNamespaceLister
	}
	type args struct {
		selector labels.Selector
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*v1alpha1.Application
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationNamespaceLister{
				ApplicationNamespaceLister: tt.fields.ApplicationNamespaceLister,
			}
			got, err := d.List(tt.args.selector)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v)", tt.args.selector)) {
				return
			}
			assert.Equalf(t, tt.want, got, "List(%v)", tt.args.selector)
		})
	}
}

func Test_deepCopyApplicationSetClient_Get(t *testing.T) {
	type fields struct {
		ApplicationSetInterface clientset.ApplicationSetInterface
	}
	type args struct {
		ctx     context.Context
		name    string
		options metav1.GetOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.ApplicationSet
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationSetClient{
				ApplicationSetInterface: tt.fields.ApplicationSetInterface,
			}
			got, err := d.Get(tt.args.ctx, tt.args.name, tt.args.options)
			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v, %v, %v)", tt.args.ctx, tt.args.name, tt.args.options)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Get(%v, %v, %v)", tt.args.ctx, tt.args.name, tt.args.options)
		})
	}
}

func Test_deepCopyApplicationSetClient_List(t *testing.T) {
	type fields struct {
		ApplicationSetInterface clientset.ApplicationSetInterface
	}
	type args struct {
		ctx  context.Context
		opts metav1.ListOptions
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1alpha1.ApplicationSetList
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &deepCopyApplicationSetClient{
				ApplicationSetInterface: tt.fields.ApplicationSetInterface,
			}
			got, err := d.List(tt.args.ctx, tt.args.opts)
			if !tt.wantErr(t, err, fmt.Sprintf("List(%v, %v)", tt.args.ctx, tt.args.opts)) {
				return
			}
			assert.Equalf(t, tt.want, got, "List(%v, %v)", tt.args.ctx, tt.args.opts)
		})
	}
}

func Test_deepCopyArgoprojV1alpha1Client_RESTClient(t *testing.T) {
	type fields struct {
		ArgoprojV1alpha1Interface clientset.ArgoprojV1alpha1Interface
	}
	tests := []struct {
		name   string
		fields fields
		want   rest.Interface
	}{
		// TODO: Add test cases.
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
