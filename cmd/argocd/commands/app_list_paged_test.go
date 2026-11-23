package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func testApps(n int) []argoappv1.Application {
	apps := make([]argoappv1.Application, n)
	for i := range apps {
		apps[i] = argoappv1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("app-%04d", i)},
		}
	}
	return apps
}

func TestListApplicationsPaged_LegacyServerExactBatchSize(t *testing.T) {
	ctx := t.Context()
	// Non-paginating servers omit Stats and may return exactly the CLI batch size.
	// Without Stats-based detection this used to loop forever, re-appending the same page.
	items := testApps(int(defaultAppListBatchSize))
	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.Anything).
		Return(&argoappv1.ApplicationList{Items: items}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, items, got)
}

func TestListApplicationsPaged_LegacyServerFullListLargerThanBatch(t *testing.T) {
	ctx := t.Context()
	items := testApps(int(defaultAppListBatchSize) + 50)
	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.Anything).
		Return(&argoappv1.ApplicationList{Items: items}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, items, got)
}

func TestListApplicationsPaged_PaginatedServerMultiplePages(t *testing.T) {
	ctx := t.Context()
	page1 := testApps(int(defaultAppListBatchSize))
	page2 := testApps(10)
	for i := range page2 {
		page2[i].Name = fmt.Sprintf("app-%04d", int(defaultAppListBatchSize)+i)
	}
	total := int64(len(page1) + len(page2))
	const listRV = "100"

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Limit != nil && *q.Limit == defaultAppListBatchSize &&
				q.Offset != nil && *q.Offset == 0
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page1,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Limit != nil && *q.Limit == defaultAppListBatchSize &&
				q.Offset != nil && *q.Offset == defaultAppListBatchSize
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page2,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	require.Len(t, got, int(total))
	assert.Equal(t, page1[0].Name, got[0].Name)
	assert.Equal(t, page2[len(page2)-1].Name, got[len(got)-1].Name)
}

func TestListApplicationsPaged_PaginatedServerExactBatchSize(t *testing.T) {
	ctx := t.Context()
	page1 := testApps(int(defaultAppListBatchSize))
	total := defaultAppListBatchSize
	const listRV = "42"

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == 0
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page1,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == defaultAppListBatchSize
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    nil,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, page1, got)
}

func TestListApplicationsPaged_ForwardsSearchOnEachPage(t *testing.T) {
	ctx := t.Context()
	page1 := testApps(int(defaultAppListBatchSize))
	page2 := testApps(2)
	for i := range page2 {
		page2[i].Name = fmt.Sprintf("app-%04d", int(defaultAppListBatchSize)+i)
	}
	total := int64(len(page1) + len(page2))
	const search = "my-app"
	const listRV = "7"

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Search != nil && *q.Search == search &&
				q.Limit != nil && *q.Limit == defaultAppListBatchSize &&
				q.Offset != nil && *q.Offset == 0 &&
				q.Projects == nil && q.Project == nil && q.Repo == nil && q.Clusters == nil
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page1,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Search != nil && *q.Search == search &&
				q.Limit != nil && *q.Limit == defaultAppListBatchSize &&
				q.Offset != nil && *q.Offset == defaultAppListBatchSize &&
				q.Projects == nil && q.Project == nil && q.Repo == nil && q.Clusters == nil
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page2,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "proj=demo", "argocd", search, nil, "", "")
	require.NoError(t, err)
	require.Len(t, got, int(total))
}

func TestListApplicationsPaged_ForwardsSupportedFiltersOnEachPage(t *testing.T) {
	ctx := t.Context()
	page1 := testApps(int(defaultAppListBatchSize))
	page2 := testApps(1)
	page2[0].Name = fmt.Sprintf("app-%04d", int(defaultAppListBatchSize))
	total := int64(len(page1) + len(page2))
	projects := []string{"demo", "other"}
	const (
		repo    = "https://example.com/repo.git"
		cluster = "https://kubernetes.default.svc"
	)

	matchesFilters := func(offset int64) func(q *applicationpkg.ApplicationQuery) bool {
		return func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == offset &&
				q.Limit != nil && *q.Limit == defaultAppListBatchSize &&
				assert.ObjectsAreEqual(projects, q.Projects) &&
				q.Project == nil &&
				q.Repo != nil && *q.Repo == repo &&
				assert.ObjectsAreEqual([]string{cluster}, q.Clusters)
		}
	}

	const listRV = "11"

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(matchesFilters(0))).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page1,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(matchesFilters(defaultAppListBatchSize))).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: listRV},
			Items:    page2,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", projects, repo, cluster)
	require.NoError(t, err)
	require.Len(t, got, int(total))
}

func TestListApplicationsPaged_RestartsWhenResourceVersionChanges(t *testing.T) {
	ctx := t.Context()
	page1a := testApps(int(defaultAppListBatchSize))
	page1b := testApps(int(defaultAppListBatchSize))
	for i := range page1b {
		page1b[i].Name = fmt.Sprintf("stable-%04d", i)
	}
	page2 := testApps(3)
	for i := range page2 {
		page2[i].Name = fmt.Sprintf("stable-%04d", int(defaultAppListBatchSize)+i)
	}
	total := int64(len(page1b) + len(page2))

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	// First walk: page 0 at RV 1, page 1 at RV 2 → restart.
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == 0
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: "1"},
			Items:    page1a,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == defaultAppListBatchSize
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: "2"},
			Items:    page2,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	// Restarted walk under stable RV 2.
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == 0
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: "2"},
			Items:    page1b,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Offset != nil && *q.Offset == defaultAppListBatchSize
		})).
		Return(&argoappv1.ApplicationList{
			ListMeta: metav1.ListMeta{ResourceVersion: "2"},
			Items:    page2,
			Stats:    &argoappv1.ApplicationListStats{Total: total},
		}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	require.Len(t, got, int(total))
	assert.Equal(t, "stable-0000", got[0].Name)
	assert.Equal(t, page2[len(page2)-1].Name, got[len(got)-1].Name)
}

func TestListApplicationsPaged_FallsBackWhenRestartsExhausted(t *testing.T) {
	ctx := t.Context()
	page := testApps(int(defaultAppListBatchSize))
	full := testApps(int(defaultAppListBatchSize) + 3)
	for i := range full[defaultAppListBatchSize:] {
		full[int(defaultAppListBatchSize)+i].Name = fmt.Sprintf("full-%04d", int(defaultAppListBatchSize)+i)
	}
	total := int64(len(page) + 10)

	mockClient := applicationmocks.NewApplicationServiceClient(t)
	// maxAppListSnapshotRestarts+1 mismatches: each walk gets page0 then a different RV on page1.
	for i := 0; i <= maxAppListSnapshotRestarts; i++ {
		rv0 := fmt.Sprintf("%d-a", i)
		rv1 := fmt.Sprintf("%d-b", i)
		mockClient.EXPECT().
			List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
				return q.Offset != nil && *q.Offset == 0 && q.Limit != nil
			})).
			Return(&argoappv1.ApplicationList{
				ListMeta: metav1.ListMeta{ResourceVersion: rv0},
				Items:    page,
				Stats:    &argoappv1.ApplicationListStats{Total: total},
			}, nil).
			Once()
		mockClient.EXPECT().
			List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
				return q.Offset != nil && *q.Offset == defaultAppListBatchSize && q.Limit != nil
			})).
			Return(&argoappv1.ApplicationList{
				ListMeta: metav1.ListMeta{ResourceVersion: rv1},
				Items:    page[:3],
				Stats:    &argoappv1.ApplicationListStats{Total: total},
			}, nil).
			Once()
	}
	// Unpaginated fallback (no limit/offset).
	mockClient.EXPECT().
		List(ctx, mock.MatchedBy(func(q *applicationpkg.ApplicationQuery) bool {
			return q.Limit == nil && q.Offset == nil
		})).
		Return(&argoappv1.ApplicationList{Items: full}, nil).
		Once()

	got, err := listApplicationsPaged(ctx, mockClient, "", "", "", nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, full, got)
}

func TestPaginateApplicationList(t *testing.T) {
	apps := testApps(5)

	assert.Equal(t, apps[:2], paginateApplicationList(apps, 0, 2))
	assert.Equal(t, apps[2:4], paginateApplicationList(apps, 2, 2))
	assert.Equal(t, apps[4:], paginateApplicationList(apps, 4, 10))
	assert.Empty(t, paginateApplicationList(apps, 5, 1))
	assert.Empty(t, paginateApplicationList(apps, 9, 1))
	// Unlimited (limit <= 0) still honors offset.
	assert.Equal(t, apps[2:], paginateApplicationList(apps, 2, 0))
	assert.Equal(t, apps[2:], paginateApplicationList(apps, 2, -1))
	assert.Empty(t, paginateApplicationList(apps, 9, 0))
	assert.Equal(t, apps[:1], paginateApplicationList(apps, -3, 1))
	// Huge limit is bounded to the remaining items (no overflow past the slice).
	assert.Equal(t, apps[2:], paginateApplicationList(apps, 2, 1<<62))
	assert.Empty(t, paginateApplicationList(apps, 5, 1<<62))
}
