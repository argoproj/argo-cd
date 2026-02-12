import Configuration from '../Configuration';
import UiTestUtilities from '../UiTestUtilities';
import assert = require("assert")
import {ApplicationsList} from '../applications-list/applications-list';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {PopupManager} from '../popup/popup-manager';
import {Navigation} from '../navigation';

/**
 * General test that
 * - creates an app based on the environment variables (see .env),
 * - syncs the app
 * - waits for the healthy and sync'ed states
 * - deletes the app.
 *
 * This can be run multiple times for different apps
 *
 */
export async function doTest(navigation: Navigation) {
    await UiTestUtilities.log('------ Test Create App ------');
    const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
    const applicationCreatePanel: ApplicationCreatePanel = await appsList.clickNewAppButton();

    await applicationCreatePanel.createApplication(
        Configuration.APP_NAME,
        Configuration.APP_PROJECT,
        Configuration.GIT_REPO,
        Configuration.SOURCE_REPO_PATH,
        Configuration.DESTINATION_CLUSTER_NAME,
        Configuration.DESTINATION_NAMESPACE
    )

    UiTestUtilities.log('Clicking on sync!');
    const appsSyncPanel: ApplicationSyncPanel = await appsList.clickSyncButtonOnApp(Configuration.APP_NAME);
    await appsSyncPanel.clickSyncButton();

    await appsList.waitForHealthStatusOnApp(Configuration.APP_NAME);
    await appsList.waitForSyncStatusOnApp(Configuration.APP_NAME);
    await appsList.checkNoAdditionalOperations(Configuration.APP_NAME);

    UiTestUtilities.log('Clicking on delete!');
    const popupManager: PopupManager = await appsList.clickDeleteButtonOnApp(Configuration.APP_NAME);
    await popupManager.setPromptFieldName(Configuration.APP_NAME);
    await popupManager.clickPromptOk();

    assert(await appsList.waitForApplicationTileToDisappear(Configuration.APP_NAME), "Application tile still present")
    await UiTestUtilities.log('====== Test Create App passed ======');
}
