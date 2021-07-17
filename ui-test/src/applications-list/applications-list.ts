import {By, until, WebDriver} from 'selenium-webdriver';
import UiTestUtilities from '../UiTestUtilities';
import * as Const from '../Constants';
import {Base} from '../base';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationsSyncPanel, SYNC_PANEL_SYNCHRONIZE_BUTTON} from '../applications-sync-panel/applications-sync-panel';
import {PopupManager} from '../popup/popup-manager';

const NEW_APP_BUTTON: By = By.xpath('.//button[@qe-id="applications-list-button-new-app"]');
// Uncomment to use:
// const CREATE_APPLICATION_BUTTON: By = By.xpath('.//button[@qe-id="applications-list-button-create-application"]');

export class ApplicationsList extends Base {
    private applicationCreatePanel: ApplicationCreatePanel;
    private applicationsSyncPanel: ApplicationsSyncPanel;
    private popupManager: PopupManager;

    public constructor(driver: WebDriver) {
        super(driver);
        this.applicationCreatePanel = new ApplicationCreatePanel(driver);
        this.applicationsSyncPanel = new ApplicationsSyncPanel(driver);
        this.popupManager = new PopupManager(driver);
    }

    public async clickTile(appName: string): Promise<void> {
        try {
            const tile = await UiTestUtilities.findUiElement(this.driver, this.getApplicationTileLocator(appName));
            await tile.click();
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     *  Click the Add New Button
     */
    public async clickNewAppButton(): Promise<ApplicationCreatePanel> {
        try {
            const newAppButton = await UiTestUtilities.findUiElement(this.driver, NEW_APP_BUTTON);
            await newAppButton.click();
        } catch (err) {
            throw new Error(err);
        }
        return this.applicationCreatePanel;
    }

    /**
     * Click the Sync button on the App tile
     *
     * @param appName
     */
    public async clickSyncButtonOnApp(appName: string): Promise<ApplicationsSyncPanel> {
        try {
            const syncButton = await UiTestUtilities.findUiElement(this.driver, this.getSyncButtonLocatorForApp(appName));
            await syncButton.click();
            // Wait until the Synchronize sliding panel appears
            const synchronizeButton = await this.driver.wait(until.elementLocated(SYNC_PANEL_SYNCHRONIZE_BUTTON), Const.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(synchronizeButton), Const.TEST_TIMEOUT);
        } catch (err) {
            throw new Error(err);
        }
        return this.applicationsSyncPanel;
    }

    /**
     * Delete an application via the Delete button on the App tile
     *
     * @param appName
     */
    public async clickDeleteButtonOnApp(appName: string): Promise<PopupManager> {
        try {
            const deleteButton = await UiTestUtilities.findUiElement(this.driver, this.getDeleteButtonLocatorForApp(appName));
            await deleteButton.click();
        } catch (err) {
            throw new Error(err);
        }
        return this.popupManager;
    }

    public async waitUntilOperationStatusDisappearsOnApp(appName: string) {
        const opStateElem = await UiTestUtilities.findUiElement(this.driver, this.getApplicationOperationsTitle(appName));
        await this.driver.wait(async () => {
            return UiTestUtilities.untilOperationStatusDisappears(opStateElem);
        }, Const.TEST_TIMEOUT);
    }

    /**
     * Click on the Refresh button on the App tile
     *
     * @param appName
     */
    public async clickRefreshButtonOnApp(appName: string): Promise<void> {
        try {
            const refreshButton = await UiTestUtilities.findUiElement(this.driver, this.getRefreshButtonLocatorForApp(appName));
            await this.driver.wait(until.elementIsVisible(refreshButton), Const.TEST_TIMEOUT);
            await refreshButton.click();
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Use with wait. Wait for the health status of the app to change to Healthy
     *
     * @param appName
     */
    public async waitForHealthStatusOnApp(appName: string): Promise<void> {
        try {
            const healthStatusElement = await UiTestUtilities.findUiElement(this.driver, this.getApplicationHealthTitle(appName));
            await this.driver.wait(async () => {
                return UiTestUtilities.untilAttributeIs(healthStatusElement, 'title', 'Healthy');
            }, Const.TEST_TIMEOUT);
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Use with wait. Wait for the sync status of the app to change to Synced
     *
     * @param appName
     */
    public async waitForSyncStatusOnApp(appName: string): Promise<void> {
        try {
            const statusElement = await UiTestUtilities.findUiElement(this.driver, this.getApplicationSyncTitle(appName));
            await this.driver.wait(async () => {
                return UiTestUtilities.untilAttributeIs(statusElement, 'title', 'Synced');
            }, Const.TEST_TIMEOUT);
        } catch (err) {
            throw new Error(err);
        }
    }

    /**
     * Check that there are no operations associated with the app
     *
     * @param appName
     */
    public async checkNoAdditionalOperations(appName: string): Promise<void> {
        // Check if there are no operations still running
        UiTestUtilities.log('Checking if there are any additional operations');
        let opStateElem;
        let opState;
        try {
            opStateElem = await this.driver.wait(until.elementLocated(this.getApplicationOperationsTitle(appName)), Const.TEST_IS_NOT_VISIBLE_TIMEOUT);
            UiTestUtilities.logError('Unexpected to locate Operation element.');
            opState = await opStateElem.getAttribute('title');
        } catch (e) {
            // ignore since we expect to not have any existing operations
        }
        if (opStateElem) {
            throw new Error('Expecting no other operations. Actual: ' + opState);
        }
    }

    // Locators

    // By.css('#app .applications-tiles .applications-list-" + appName + "'');

    private getApplicationTileLocator(appName: string): By {
        return By.xpath('.//div[contains(@class,"qe-applications-list-"' + appName + ')');
    }

    private getSyncButtonLocatorForApp(appName: string): By {
        return By.xpath('.//div[contains(@class, "qe-applications-list-' + appName + '")]//div[@class="row"]//ancestor::a[@qe-id="applications-tiles-button-sync"]');
    }

    private getDeleteButtonLocatorForApp(appName: string): By {
        return By.xpath('.//div[contains(@class, "qe-applications-list-' + appName + '")]//div[@class="row"]//ancestor::a[@qe-id="applications-tiles-button-delete"]');
    }

    private getRefreshButtonLocatorForApp(appName: string): By {
        return By.xpath('.//div[contains(@class, "qe-applications-list-' + appName + '")]//div[@class="row"]//ancestor::a[@qe-id="applications-tiles-button-refresh"]');
    }

    private getApplicationHealthTitle(appName: string): By {
        return By.xpath(
            './/div[contains(@class, "qe-applications-list-' +
                appName +
                '")]//div[@class="row"]//div[@qe-id="applications-tiles-health-status"]//i[@qe-id="utils-health-status-title"]'
        );
    }

    private getApplicationSyncTitle(appName: string): By {
        return By.xpath(
            './/div[contains(@class, "qe-applications-list-' +
                appName +
                '")]//div[@class="row"]//div[@qe-id="applications-tiles-health-status"]//i[@qe-id="utils-sync-status-title"]'
        );
    }

    private getApplicationOperationsTitle(appName: string): By {
        return By.xpath(
            './/div[contains(@class, "qe-applications-list-' +
                appName +
                '")]//div[@class="row"]//div[@qe-id="applications-tiles-health-status"]//i[@qe-id="utils-operations-status-title"]'
        );
    }
}
