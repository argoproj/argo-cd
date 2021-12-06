package argo

import (
	"context"

	"github.com/prometheus/common/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applicationsv1 "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// FilterByProjects returns applications which belongs to the specified project
func FilterByProjects(apps []argoappv1.Application, projects []string) []argoappv1.Application {
	if len(projects) == 0 {
		return apps
	}
	projectsMap := make(map[string]bool)
	for i := range projects {
		projectsMap[projects[i]] = true
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		a := apps[i]
		if _, ok := projectsMap[a.Spec.GetProject()]; ok {
			items = append(items, a)
		}
	}
	return items

}

// GetAppProjectByName returns a project from an application based on name
func GetAppProjectByName(name string, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB, ctx context.Context) (*argoappv1.AppProject, error) {
	projOrig, err := projLister.AppProjects(ns).Get(name)
	if err != nil {
		return nil, err
	}
	project := projOrig.DeepCopy()
	repos := retrieveScopedRepositories(name, db, ctx)
	for _, repo := range repos {
		project.Spec.SourceRepos = append(project.Spec.SourceRepos, repo.Repo)
	}
	clusters := retrieveScopedClusters(name, db, ctx)
	for _, cluster := range clusters {
		if len(cluster.Namespaces) == 0 {
			project.Spec.Destinations = append(project.Spec.Destinations, argoappv1.ApplicationDestination{Server: cluster.Server, Namespace: "*"})
		} else {
			for _, ns := range cluster.Namespaces {
				project.Spec.Destinations = append(project.Spec.Destinations, argoappv1.ApplicationDestination{Server: cluster.Server, Namespace: ns})
			}
		}
	}
	return GetAppVirtualProject(project, projLister, settingsManager)
}

func GetAppVirtualProject(proj *argoappv1.AppProject, projLister applicationsv1.AppProjectLister, settingsManager *settings.SettingsManager) (*argoappv1.AppProject, error) {
	virtualProj := proj.DeepCopy()
	globalProjects := GetGlobalProjects(proj, projLister, settingsManager)

	for _, gp := range globalProjects {
		virtualProj = mergeVirtualProject(virtualProj, gp)
	}
	return virtualProj, nil
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, projLister applicationsv1.AppProjectLister, ns string, settingsManager *settings.SettingsManager, db db.ArgoDB, ctx context.Context) (*argoappv1.AppProject, error) {
	return GetAppProjectByName(spec.GetProject(), projLister, ns, settingsManager, db, ctx)
}

func GetGlobalProjects(proj *argoappv1.AppProject, projLister applicationsv1.AppProjectLister, settingsManager *settings.SettingsManager) []*argoappv1.AppProject {
	gps, err := settingsManager.GetGlobalProjectsSettings()
	globalProjects := make([]*argoappv1.AppProject, 0)

	if err != nil {
		log.Warnf("Failed to get global project settings: %v", err)
		return globalProjects
	}

	for _, gp := range gps {
		//The project itself is not its own the global project
		if proj.Name == gp.ProjectName {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(&gp.LabelSelector)
		if err != nil {
			break
		}
		//Get projects which match the label selector, then see if proj is a match
		projList, err := projLister.AppProjects(proj.Namespace).List(selector)
		if err != nil {
			break
		}
		var matchMe bool
		for _, item := range projList {
			if item.Name == proj.Name {
				matchMe = true
				break
			}
		}
		if !matchMe {
			continue
		}
		//If proj is a match for this global project setting, then it is its global project
		globalProj, err := projLister.AppProjects(proj.Namespace).Get(gp.ProjectName)
		if err != nil {
			break
		}
		globalProjects = append(globalProjects, globalProj)

	}
	return globalProjects
}

func mergeVirtualProject(proj *argoappv1.AppProject, globalProj *argoappv1.AppProject) *argoappv1.AppProject {
	if globalProj == nil {
		return proj
	}
	proj.Spec.ClusterResourceWhitelist = append(proj.Spec.ClusterResourceWhitelist, globalProj.Spec.ClusterResourceWhitelist...)
	proj.Spec.ClusterResourceBlacklist = append(proj.Spec.ClusterResourceBlacklist, globalProj.Spec.ClusterResourceBlacklist...)

	proj.Spec.NamespaceResourceWhitelist = append(proj.Spec.NamespaceResourceWhitelist, globalProj.Spec.NamespaceResourceWhitelist...)
	proj.Spec.NamespaceResourceBlacklist = append(proj.Spec.NamespaceResourceBlacklist, globalProj.Spec.NamespaceResourceBlacklist...)

	proj.Spec.SyncWindows = append(proj.Spec.SyncWindows, globalProj.Spec.SyncWindows...)

	proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, globalProj.Spec.SourceRepos...)

	proj.Spec.Destinations = append(proj.Spec.Destinations, globalProj.Spec.Destinations...)

	return proj
}
