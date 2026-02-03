import Configuration from '../Configuration';
import UiTestUtilities from '../UiTestUtilities';
import {By} from 'selenium-webdriver';
import assert = require("assert")
import {ApplicationsList} from '../applications-list/applications-list';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {PopupManager} from '../popup/popup-manager';
import {Navigation} from '../navigation';

async function syncApplication(appsList: ApplicationsList, appName: string) {
    UiTestUtilities.log('About to sync application');
    const appsSyncPanel: ApplicationSyncPanel = await appsList.clickSyncButtonOnApp(appName);
    await appsSyncPanel.clickSyncButton();

    await appsList.waitForHealthStatusOnApp(appName);
    await appsList.waitForSyncStatusOnApp(appName);
}

async function deleteApplication(appsList: ApplicationsList, appName: string) {
    UiTestUtilities.log('About to delete application');
    const popupManager: PopupManager = await appsList.clickDeleteButtonOnApp(appName);
    await popupManager.setPromptFieldName(appName);
    await popupManager.clickPromptOk();
}

async function assertAppOfAppsPresent(navigation: Navigation, appName: string) {
    const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
    const syncAppsPanel: ApplicationsSyncPanel = await appsList.clickSyncAppsButton();
    await UiTestUtilities.sleep(200);
    assert(await UiTestUtilities.existInPage(navigation.getDriver(), `\(App of Apps\) ${appName}`));
    await UiTestUtilities.captureSession(navigation.getDriver(), "assert_app_of_apps.png");
    await syncAppsPanel.clickCancelButton();
}

async function assertExternalLinks(navigation: Navigation) {
    const locator: By = By.xpath('//div[contains(@class, "applications-list__external-link")]//div[contains(@class, "argo-dropdown__anchor")]');
    const externalLinksDropDown = await UiTestUtilities.findUiElement(navigation.getDriver(), locator);
    assert(externalLinksDropDown, "External Links not found");
    externalLinksDropDown.click();
    await UiTestUtilities.sleep(200)
    await UiTestUtilities.captureSession(navigation.getDriver(), "assert_externalLinks.png");
}

async function assertSourcesShown(navigation: Navigation) {
    const locator: By = By.xpath('//div[@title="Repository:"]');
    assert(await UiTestUtilities.findUiElement(navigation.getDriver(), locator), "Repository info not found");
    await UiTestUtilities.captureSession(navigation.getDriver(), "assert_sources_shown.png");
}

export async function doTest(navigation: Navigation) {
    await UiTestUtilities.log('------ Test Applications List ------');
    const appName: string = "apps"
    const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
    const applicationCreatePanel: ApplicationCreatePanel = await appsList.clickNewAppButton();
    await applicationCreatePanel.createApplication(
        appName,
        Configuration.APP_PROJECT,
        // TODO: Replace by Configuration.GIT_REPO once one app contains an external-link annotation
        "https://github.com/aveuiller/argocd-tests.git",
        "applications",
        Configuration.DESTINATION_CLUSTER_NAME,
        Configuration.DESTINATION_NAMESPACE
    );
    await syncApplication(appsList, appName);

    await assertAppOfAppsPresent(navigation, appName);
    await assertSourcesShown(navigation);
    await assertExternalLinks(navigation);

    await deleteApplication(appsList, appName);
    await UiTestUtilities.log('====== Test Applications List passed ======');
}
