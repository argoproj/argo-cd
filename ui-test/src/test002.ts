import UiTestUtilities from './UiTestUtilities';
import {trace} from 'console';
import {ApplicationsList} from './applications-list/applications-list';
import {ApplicationCreatePanel} from './application-create-panel/application-create-panel';

/**
 * Test to demo how to visit each page via the navigation bar on the left.
 *
 */
async function doTest() {
    const navigation = await UiTestUtilities.init();
    try {
        await navigation.clickDocsNavBarButton();
        await navigation.clickUserInfoNavBarButton();
        await navigation.clickSettingsNavBarButton();
        const appsList: ApplicationsList = await navigation.clickApplicationsNavBarButton();
        const applicationCreatePanel: ApplicationCreatePanel = await appsList.clickNewAppButton();
        await applicationCreatePanel.clickCancelButton();
        await UiTestUtilities.log('Test passed');
    } catch (e) {
        trace('Test failed ', e);
    } finally {
        await navigation.quit();
    }
}

doTest();
