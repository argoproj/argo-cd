import {Application} from '../../../shared/models';
import {AppsListPreferences} from '../../../shared/services';

/**
 * Returns true when an empty list means the user has no applications at all, rather than none that
 * pass their filters. The favorites filter is applied server-side, so with it enabled an empty
 * response only means that nothing favorited matched. Getting this wrong hides the filter sidebar,
 * which leaves the user unable to switch the filter back off.
 **/
export function showCreateFirstAppState(apps: Application[], pref: AppsListPreferences): boolean {
    return apps.length === 0 && !pref.showFavorites && (pref.projectsFilter || []).length === 0 && (pref.labelsFilter || []).length === 0;
}
