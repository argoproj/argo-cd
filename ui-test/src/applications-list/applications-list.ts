import {By, until, WebDriver} from 'selenium-webdriver';
import UiTestUtilities from '../UiTestUtilities';
import {Base} from '../base';
import {ApplicationCreatePanel} from '../application-create-panel/application-create-panel';
import {ApplicationSyncPanel} from '../application-sync-panel/application-sync-panel';
import {ApplicationsSyncPanel} from '../applications-sync-panel/applications-sync-panel';
import {PopupManager} from '../popup/popup-manager';
import Configuration from '../Configuration';


export class ApplicationsList extends Base {
    public static readonly FIRST_APP_BUTTON: By = By.xpath('.//button[@qe-id="applications-list-noapp-button-create-application"]');
    public static readonly NEW_APP_BUTTON: By = By.xpath('.//button[@qe-id="applications-list-button-new-app"]');
    public static readonly SYNC_APPS_BUTTON: By = By.xpath('.//button[@qe-id="applications-list-button-sync-apps"]');

    private applicationCreatePanel: ApplicationCreatePanel;
    private applicationsSyncPanel: ApplicationsSyncPanel;
    private applicationSyncPanel: ApplicationSyncPanel;
    private popupManager: PopupManager;

    public constructor(driver: WebDriver) {
        super(driver);
        this.applicationCreatePanel = new ApplicationCreatePanel(driver);
        this.applicationsSyncPanel = new ApplicationsSyncPanel(driver);
        this.applicationSyncPanel = new ApplicationSyncPanel(driver);
        this.popupManager = new PopupManager(driver);
    }

    public async clickTile(appName: string): Promise<void> {
        try {
            const tile = await UiTestUtilities.findUiElement(this.driver, this.getApplicationTileLocator(appName));
            await tile.click();
        } catch (err: any) {
            throw new Error(err);
        }
    }

    /**
     *  Click the Add New Button
     */
    public async clickNewAppButton(): Promise<ApplicationCreatePanel> {
        try {
            const newAppButton = await UiTestUtilities.findUiElement(this.driver, ApplicationsList.NEW_APP_BUTTON);
            await newAppButton.click();

            const createButton = await this.driver.wait(until.elementLocated(ApplicationCreatePanel.BUTTON_CREATE), Configuration.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(createButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                UiTestUtilities.logError(`Create App Panel didn't show up: ${e}`);
                throw e;
            });
        } catch (err: any) {
            throw new Error(err);
        }
        return this.applicationCreatePanel;
    }

    /**
     *  Click the Add New Button
     */
    public async clickSyncAppsButton(): Promise<ApplicationsSyncPanel> {
        try {
            const syncAppsButton = await UiTestUtilities.findUiElement(this.driver, ApplicationsList.SYNC_APPS_BUTTON);
            await syncAppsButton.click();

            const synchronizeButton = await this.driver.wait(until.elementLocated(ApplicationsSyncPanel.BUTTON_SYNCHRONIZE), Configuration.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(synchronizeButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                UiTestUtilities.logError(`Sync Apps Panel didn't show up: ${e}`);
                throw e;
            });
        } catch (err: any) {
            throw new Error(err);
        }
        return this.applicationsSyncPanel;
    }

    /**
     * Click the Sync button on the App tile
     *
     * @param appName
     */
    public async clickSyncButtonOnApp(appName: string): Promise<ApplicationSyncPanel> {
        try {
            const syncButton = await UiTestUtilities.findUiElement(this.driver, this.getSyncButtonLocatorForApp(appName));
            await syncButton.click();

            const synchronizeButton = await this.driver.wait(until.elementLocated(ApplicationSyncPanel.BUTTON_SYNCHRONIZE), Configuration.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(synchronizeButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                UiTestUtilities.logError(`Sync App Panel didn't show up: ${e}`);
                throw e;
            });
        } catch (err: any) {
            throw new Error(err);
        }
        return this.applicationSyncPanel;
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
        } catch (err: any) {
            throw new Error(err);
        }
        return this.popupManager;
    }

    /**
     * Click on the Refresh button on the App tile
     *
     * @param appName
     */
    public async clickRefreshButtonOnApp(appName: string): Promise<void> {
        try {
            const refreshButton = await UiTestUtilities.findUiElement(this.driver, this.getRefreshButtonLocatorForApp(appName));
            await this.driver.wait(until.elementIsVisible(refreshButton), Configuration.TEST_TIMEOUT);
            await refreshButton.click();
        } catch (err: any) {
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
            }, Configuration.TEST_TIMEOUT);
        } catch (err: any) {
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
            }, Configuration.TEST_TIMEOUT);
        } catch (err: any) {
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
            opStateElem = await this.driver.wait(until.elementLocated(this.getApplicationOperationsTitle(appName)), Configuration.TEST_IS_NOT_VISIBLE_TIMEOUT);
            UiTestUtilities.logError('Unexpected to locate Operation element.');
            opState = await opStateElem.getAttribute('title');
        } catch (e) {
            // ignore since we expect to not have any existing operations
        }
        if (opStateElem) {
            throw new Error('Expecting no other operations. Actual: ' + opState);
        }
    }

    /**
     * Checks if an application tile has disappeared from the UI.
     *
     * @param appName - The name of the application to check
     * @returns Promise<boolean> - True if it disappeared
     */
    public async waitForApplicationTileToDisappear(appName: string): Promise<boolean> {
        const locator = this.getApplicationTileLocator(appName);

        try {
            return await this.driver.wait(async () => {
                const elements = await this.driver.findElements(locator);
                if (elements.length === 0) {
                    return true;
                }
                return !await elements[0].isDisplayed();
            }, Configuration.TEST_IS_NOT_VISIBLE_TIMEOUT);
        } catch (e) {
            UiTestUtilities.logError(`Application Tile for "${appName}" still present after timeout. Error: ${e}`);
            throw e;
        }
    }
    // Locators
    private getApplicationTileSelector(appName: string): string {
        return '//div[contains(@class, "qe-applications-list-' + Configuration.ARGOCD_NAMESPACE + '_' + appName + '")]';
    }

    private getApplicationTileLocator(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName));
    }

    private getSyncButtonLocatorForApp(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//a[contains(@qe-id, "applications-tiles-button-sync")]');
    }

    private getDeleteButtonLocatorForApp(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//a[contains(@qe-id, "applications-tiles-button-delete")]');
    }

    private getRefreshButtonLocatorForApp(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//a[contains(@qe-id, "applications-tiles-button-refresh")]');
    }

    private getApplicationHealthTitle(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//i[contains(@qe-id, "utils-health-status-title")]');
    }

    private getApplicationSyncTitle(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//i[contains(@qe-id, "utils-sync-status-title")]');
    }

    private getApplicationOperationsTitle(appName: string): By {
        return By.xpath(this.getApplicationTileSelector(appName) + '//i[contains(@qe-id, "utils-operations-status-title")]');
    }
}
