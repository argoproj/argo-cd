import UiTestUtilities from '../UiTestUtilities';
import {ApplicationsList} from '../applications-list/applications-list';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {Navigation} from '../navigation';

/**
 * Test to demo how to visit each page via the navigation bar on the left.
 *
 */
export async function doTest(navigation: Navigation) {
    await UiTestUtilities.log('------ Test Navigate Sidebar ------');
    await navigation.clickDocsNavBarButton();
    await navigation.clickUserInfoNavBarButton();
    await navigation.clickSettingsNavBarButton();
    const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
    const applicationCreatePanel: ApplicationCreatePanel = await appsList.clickNewAppButton();

    // wait slide effect
    await navigation.sleep(500);
    await applicationCreatePanel.clickCancelButton();
    await UiTestUtilities.log('====== Test Navigate Sidebar passed ======');
}

