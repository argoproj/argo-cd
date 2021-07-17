import Configuration from './Configuration';
import UiTestUtilities from './UiTestUtilities';
import {trace} from 'console';
import {ApplicationsList} from './applications-list/applications-list';
import {ApplicationCreatePanel} from './application-create-panel/application-create-panel';
import {ApplicationsSyncPanel} from './applications-sync-panel/applications-sync-panel';
import {PopupManager} from './popup/popup-manager';

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
async function doTest() {
    const navigation = await UiTestUtilities.init();
    try {
        const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
        const applicationCreatePanel: ApplicationCreatePanel = await appsList.clickNewAppButton();

        UiTestUtilities.log('About to create application');
        await applicationCreatePanel.setAppName(Configuration.APP_NAME);
        await applicationCreatePanel.setProjectName(Configuration.APP_PROJECT);
        await applicationCreatePanel.setSourceRepoUrl(Configuration.GIT_REPO);
        await applicationCreatePanel.setSourceRepoPath(Configuration.SOURCE_REPO_PATH);
        await applicationCreatePanel.selectDestinationClusterNameMenu(Configuration.DESTINATION_CLUSTER_NAME);
        await applicationCreatePanel.setDestinationNamespace(Configuration.DESTINATION_NAMESPACE);
        await applicationCreatePanel.clickCreateButton();

        const appsSyncPanel: ApplicationsSyncPanel = await appsList.clickSyncButtonOnApp(Configuration.APP_NAME);
        await appsSyncPanel.clickSyncButton();

        await appsList.waitForHealthStatusOnApp(Configuration.APP_NAME);
        await appsList.waitForSyncStatusOnApp(Configuration.APP_NAME);
        await appsList.checkNoAdditionalOperations(Configuration.APP_NAME);

        const popupManager: PopupManager = await appsList.clickDeleteButtonOnApp(Configuration.APP_NAME);
        await popupManager.setPromptFieldName(Configuration.APP_NAME);
        await popupManager.clickPromptOk();
        // After deleting, wait until the delete operation finishes
        await appsList.waitUntilOperationStatusDisappearsOnApp(Configuration.APP_NAME);

        await UiTestUtilities.log('Test passed');
    } catch (e) {
        trace('Test failed ', e);
    } finally {
        await navigation.quit();
    }
}

doTest();
